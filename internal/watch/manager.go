package watch

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"golang.org/x/sync/semaphore"

	"github.com/jedarden/domain-check/internal/domain"
)

// Manager handles domain watch polling and webhook delivery.
type Manager struct {
	store         *Store
	webhook       *WebhookClient
	checker       DomainChecker
	pollInterval  time.Duration
	maxTTL        time.Duration
	maxWatchesPerIP int
	logger        *slog.Logger

	// Concurrency control
	pollSemaphore *semaphore.Weighted

	// State
	mu             sync.RWMutex
	cancel         context.CancelFunc
	wg            sync.WaitGroup
	running       bool
}

// DomainChecker is the interface for checking domain availability.
// This matches the checker.Checker interface but we use a smaller interface
// to avoid circular dependencies.
type DomainChecker interface {
	Check(ctx context.Context, domain string) (*domain.DomainResult, error)
}

// ManagerConfig holds configuration for the watch manager.
type ManagerConfig struct {
	Store           *Store
	WebhookClient   *WebhookClient
	Checker         DomainChecker
	PollInterval    time.Duration
	MaxTTL          time.Duration
	MaxWatchesPerIP int
	Logger          *slog.Logger
}

// DefaultManagerConfig returns default manager configuration.
func DefaultManagerConfig() ManagerConfig {
	return ManagerConfig{
		PollInterval:    15 * time.Minute,
		MaxTTL:          90 * 24 * time.Hour, // 90 days
		MaxWatchesPerIP: 10,                  // Per IP per day
		Logger:          slog.Default(),
	}
}

// NewManager creates a new watch manager.
func NewManager(cfg ManagerConfig) *Manager {
	if cfg.PollInterval == 0 {
		cfg.PollInterval = DefaultManagerConfig().PollInterval
	}
	if cfg.MaxTTL == 0 {
		cfg.MaxTTL = DefaultManagerConfig().MaxTTL
	}
	if cfg.MaxWatchesPerIP == 0 {
		cfg.MaxWatchesPerIP = DefaultManagerConfig().MaxWatchesPerIP
	}
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}

	return &Manager{
		store:          cfg.Store,
		webhook:        cfg.WebhookClient,
		checker:        cfg.Checker,
		pollInterval:   cfg.PollInterval,
		maxTTL:         cfg.MaxTTL,
		maxWatchesPerIP: cfg.MaxWatchesPerIP,
		logger:         cfg.Logger,
		pollSemaphore:  semaphore.NewWeighted(5), // Max 5 concurrent polls
	}
}

// Start begins the watch polling loop.
func (m *Manager) Start(ctx context.Context) error {
	m.mu.Lock()
	if m.running {
		m.mu.Unlock()
		return fmt.Errorf("manager already running")
	}
	m.running = true
	m.mu.Unlock()

	ctx, cancel := context.WithCancel(ctx)
	m.cancel = cancel

	m.wg.Add(1)
	go m.pollLoop(ctx)

	m.logger.Info("watch manager started", "poll_interval", m.pollInterval)
	return nil
}

// Stop gracefully shuts down the watch manager.
func (m *Manager) Stop() {
	m.mu.Lock()
	if !m.running {
		m.mu.Unlock()
		return
	}
	m.running = false
	m.mu.Unlock()

	if m.cancel != nil {
		m.cancel()
	}

	m.wg.Wait()
	m.logger.Info("watch manager stopped")
}

// pollLoop runs the polling loop until context is canceled.
func (m *Manager) pollLoop(ctx context.Context) {
	defer m.wg.Done()

	ticker := time.NewTicker(m.pollInterval)
	defer ticker.Stop()

	// Run once immediately on startup
	m.pollWatches(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.pollWatches(ctx)
		}
	}
}

// pollWatches checks all active watches and delivers webhooks for transitions.
func (m *Manager) pollWatches(ctx context.Context) {
	watches, err := m.store.ListAll()
	if err != nil {
		m.logger.Error("failed to list watches", "error", err)
		return
	}

	if len(watches) == 0 {
		return
	}

	m.logger.Debug("polling watches", "count", len(watches))

	// Limit concurrent polling to avoid overwhelming the checker
	sem := m.pollSemaphore
	var wg sync.WaitGroup

	for _, watch := range watches {
		wg.Add(1)
		go func(w *WatchData) {
			defer wg.Done()

			// Acquire semaphore
			if err := sem.Acquire(ctx, 1); err != nil {
				return
			}
			defer sem.Release(1)

			m.checkWatch(ctx, w)
		}(watch)
	}

	wg.Wait()
}

// checkWatch checks a single watch and delivers webhook if status changed.
func (m *Manager) checkWatch(ctx context.Context, watch *WatchData) {
	// Check domain availability
	result, err := m.checker.Check(ctx, watch.Domain)
	if err != nil {
		m.logger.Error("failed to check watched domain",
			"domain", watch.Domain,
			"watch_id", watch.ID,
			"error", err)
		return
	}

	// Update last checked time
	watch.LastChecked = time.Now()

	// Determine current status
	currentStatus := "taken"
	if result.Available {
		currentStatus = "available"
	}

	// Check for transition from taken to available
	if watch.LastStatus == "taken" && currentStatus == "available" {
		m.logger.Info("domain became available, delivering webhook",
			"domain", watch.Domain,
			"watch_id", watch.ID,
			"webhook_url", watch.WebhookURL)

		// Deliver webhook with retries
		if err := m.deliverWebhookWithRetry(ctx, watch, result); err == nil {
			// Mark as delivered (single-fire)
			watch.Delivered = true
			m.logger.Info("webhook delivered successfully",
				"domain", watch.Domain,
				"watch_id", watch.ID)
		} else {
			m.logger.Error("webhook delivery failed after retries",
				"domain", watch.Domain,
				"watch_id", watch.ID,
				"error", err)
		}
	} else if watch.LastStatus != currentStatus {
		// Status changed but not the transition we care about
		m.logger.Debug("domain status changed",
			"domain", watch.Domain,
			"watch_id", watch.ID,
			"old_status", watch.LastStatus,
			"new_status", currentStatus)
	}

	// Update status
	watch.LastStatus = currentStatus

	// Save updated watch data
	if err := m.store.Update(watch); err != nil {
		m.logger.Error("failed to update watch data",
			"domain", watch.Domain,
			"watch_id", watch.ID,
			"error", err)
	}
}

// deliverWebhookWithRetry delivers a webhook with exponential backoff retry.
func (m *Manager) deliverWebhookWithRetry(ctx context.Context, watch *WatchData, result *domain.DomainResult) error {
	payload := &WebhookPayload{
		Domain:    result.Domain,
		Available: result.Available,
		CheckedAt: result.CheckedAt,
	}

	// Retry with exponential backoff: 1s, 2s, 4s, 8s, max 30s
	attempts := []time.Duration{1 * time.Second, 2 * time.Second, 4 * time.Second, 8 * time.Second}
	maxWait := 30 * time.Second

	for i, wait := range attempts {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(wait):
		}

		statusCode, err := m.webhook.Deliver(ctx, watch.WebhookURL, watch.Secret, payload)
		if err == nil && statusCode >= 200 && statusCode < 300 {
			return nil
		}

		m.logger.Debug("webhook delivery attempt failed",
			"domain", watch.Domain,
			"watch_id", watch.ID,
			"attempt", i+1,
			"status_code", statusCode,
			"error", err)
	}

	// Final attempt with longer wait
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(maxWait):
	}

	statusCode, err := m.webhook.Deliver(ctx, watch.WebhookURL, watch.Secret, payload)
	if err == nil && statusCode >= 200 && statusCode < 300 {
		return nil
	}

	return fmt.Errorf("webhook delivery failed: status=%d, err=%w", statusCode, err)
}

// Register creates a new watch with abuse prevention checks.
func (m *Manager) Register(domain, webhookURL, clientIP string) (*WatchData, error) {
	// Validate inputs
	if domain == "" {
		return nil, fmt.Errorf("domain is required")
	}
	if webhookURL == "" {
		return nil, fmt.Errorf("webhook_url is required")
	}

	// Check abuse limits: max watches per IP per day
	since := time.Now().Add(-24 * time.Hour)
	existingWatches, err := m.store.ListByIP(clientIP, since)
	if err != nil {
		return nil, fmt.Errorf("failed to check abuse limits: %w", err)
	}

	if len(existingWatches) >= m.maxWatchesPerIP {
		return nil, fmt.Errorf("maximum watches per IP reached: %d per 24 hours", m.maxWatchesPerIP)
	}

	// Generate ID and secret
	id, err := generateID()
	if err != nil {
		return nil, fmt.Errorf("generate ID: %w", err)
	}

	secret, err := generateSecret()
	if err != nil {
		return nil, fmt.Errorf("generate secret: %w", err)
	}

	// Create watch entry
	now := time.Now()
	watch := &WatchData{
		ID:         id,
		Domain:     domain,
		WebhookURL: webhookURL,
		Secret:     secret,
		CreatedAt:  now,
		ExpiresAt:  now.Add(m.maxTTL),
		LastStatus: "taken", // Assume initially taken until first check
		ClientIP:   clientIP,
	}

	// Store
	if err := m.store.Create(watch); err != nil {
		return nil, fmt.Errorf("create watch: %w", err)
	}

	m.logger.Info("watch registered",
		"domain", domain,
		"watch_id", id,
		"expires_at", watch.ExpiresAt,
		"client_ip", clientIP)

	return watch, nil
}

// Cancel removes a watch by ID, requiring the correct secret.
func (m *Manager) Cancel(id, secret string) error {
	// Get watch to verify secret
	watch, err := m.store.Get(id)
	if err != nil {
		return fmt.Errorf("watch not found")
	}

	// Verify secret
	if watch.Secret != secret {
		return fmt.Errorf("invalid secret")
	}

	// Delete
	if err := m.store.Delete(id); err != nil {
		return fmt.Errorf("delete watch: %w", err)
	}

	m.logger.Info("watch canceled", "watch_id", id, "domain", watch.Domain)
	return nil
}

// generateID generates a random watch ID.
func generateID() (string, error) {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// generateSecret generates a random webhook secret.
func generateSecret() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
