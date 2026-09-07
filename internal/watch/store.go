// Package watch provides domain watch functionality with webhook notifications.
package watch

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"go.etcd.io/bbolt"
)

// Store manages persistent storage of watch data using bbolt.
type Store struct {
	db   *bbolt.DB
	path string
	mu   sync.RWMutex
}

// WatchData represents a stored watch entry.
type WatchData struct {
	ID          string    `json:"id"`
	Domain      string    `json:"domain"`
	WebhookURL  string    `json:"webhook_url"`
	Secret      string    `json:"secret"` // HMAC secret for webhook signature
	CreatedAt   time.Time `json:"created_at"`
	ExpiresAt   time.Time `json:"expires_at"`
	LastChecked time.Time `json:"last_checked"`
	LastStatus  string    `json:"last_status"` // "available" or "taken"
	ClientIP    string    `json:"client_ip"`  // For abuse prevention
	Delivered   bool      `json:"delivered"`  // True after webhook delivery

	// Reconnection fields
	DeliveryFailures     int       `json:"delivery_failures"`     // Consecutive delivery failures
	LastDeliveryAttempt  time.Time `json:"last_delivery_attempt"` // Last webhook delivery attempt
	Dead                 bool      `json:"dead"`                  // Watch is permanently dead
	DeadSince           time.Time `json:"dead_since"`            // When watch was marked dead
}

// watchBucket is the name of the bbolt bucket storing watch data.
const watchBucket = "watches"

// NewStore creates or opens a bbolt database at the given path.
func NewStore(path string) (*Store, error) {
	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("create watch db directory: %w", err)
	}

	// Open database (creates if doesn't exist)
	db, err := bbolt.Open(path, 0600, &bbolt.Options{
		Timeout: 10 * time.Second,
	})
	if err != nil {
		return nil, fmt.Errorf("open watch db: %w", err)
	}

	// Create bucket if it doesn't exist
	if err := db.Update(func(tx *bbolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte(watchBucket))
		return err
	}); err != nil {
		db.Close()
		return nil, fmt.Errorf("create bucket: %w", err)
	}

	return &Store{
		db:   db,
		path: path,
	}, nil
}

// Close closes the database.
func (s *Store) Close() error {
	return s.db.Close()
}

// Create stores a new watch entry.
func (s *Store) Create(data *WatchData) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(watchBucket))
		if b == nil {
			return fmt.Errorf("bucket not found")
		}

		// Check for duplicate (same domain + webhook + not expired)
		c := b.Cursor()
		for k, v := c.First(); k != nil; k, v = c.Next() {
			var existing WatchData
			if err := json.Unmarshal(v, &existing); err != nil {
				continue
			}
			if existing.Domain == data.Domain && existing.WebhookURL == data.WebhookURL && existing.ExpiresAt.After(time.Now()) {
				return fmt.Errorf("watch already exists for this domain and webhook")
			}
		}

		// Serialize and store
		encoded, err := json.Marshal(data)
		if err != nil {
			return fmt.Errorf("marshal watch data: %w", err)
		}

		return b.Put([]byte(data.ID), encoded)
	})
}

// Get retrieves a watch entry by ID.
func (s *Store) Get(id string) (*WatchData, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var data WatchData

	err := s.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(watchBucket))
		if b == nil {
			return fmt.Errorf("bucket not found")
		}

		v := b.Get([]byte(id))
		if v == nil {
			return fmt.Errorf("watch not found")
		}

		return json.Unmarshal(v, &data)
	})

	if err != nil {
		return nil, err
	}

	return &data, nil
}

// Update updates a watch entry.
func (s *Store) Update(data *WatchData) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(watchBucket))
		if b == nil {
			return fmt.Errorf("bucket not found")
		}

		// Verify exists
		if v := b.Get([]byte(data.ID)); v == nil {
			return fmt.Errorf("watch not found")
		}

		// Serialize and update
		encoded, err := json.Marshal(data)
		if err != nil {
			return fmt.Errorf("marshal watch data: %w", err)
		}

		return b.Put([]byte(data.ID), encoded)
	})
}

// Delete removes a watch entry by ID.
func (s *Store) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(watchBucket))
		if b == nil {
			return fmt.Errorf("bucket not found")
		}

		return b.Delete([]byte(id))
	})
}

// ListAll returns all active (non-expired) watch entries.
func (s *Store) ListAll() ([]*WatchData, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var watches []*WatchData

	err := s.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(watchBucket))
		if b == nil {
			return fmt.Errorf("bucket not found")
		}

		now := time.Now()
		return b.ForEach(func(k, v []byte) error {
			var data WatchData
			if err := json.Unmarshal(v, &data); err != nil {
				return nil // Skip corrupted entries
			}

			// Skip expired or delivered watches
			if data.ExpiresAt.Before(now) || data.Delivered {
				return nil
			}

			watches = append(watches, &data)
			return nil
		})
	})

	if err != nil {
		return nil, err
	}

	return watches, nil
}

// ListByIP returns all active watches for a specific client IP, for abuse prevention.
func (s *Store) ListByIP(clientIP string, since time.Time) ([]*WatchData, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var watches []*WatchData

	err := s.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(watchBucket))
		if b == nil {
			return fmt.Errorf("bucket not found")
		}

		return b.ForEach(func(k, v []byte) error {
			var data WatchData
			if err := json.Unmarshal(v, &data); err != nil {
				return nil
			}

			// Skip if not matching IP or outside time window
			if data.ClientIP != clientIP || data.CreatedAt.Before(since) {
				return nil
			}

			watches = append(watches, &data)
			return nil
		})
	})

	if err != nil {
		return nil, err
	}

	return watches, nil
}

// CleanupExpired removes all expired and delivered watch entries.
// Should be called periodically to prevent database bloat.
func (s *Store) CleanupExpired() (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	count := 0
	now := time.Now()

	err := s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(watchBucket))
		if b == nil {
			return fmt.Errorf("bucket not found")
		}

		var toDelete [][]byte

		// Find entries to delete
		c := b.Cursor()
		for k, v := c.First(); k != nil; k, v = c.Next() {
			var data WatchData
			if err := json.Unmarshal(v, &data); err != nil {
				toDelete = append(toDelete, k)
				continue
			}

			// Delete if expired or delivered
			if data.ExpiresAt.Before(now) || data.Delivered {
				toDelete = append(toDelete, k)
			}
		}

		// Delete them
		for _, k := range toDelete {
			if err := b.Delete(k); err != nil {
				return err
			}
			count++
		}

		return nil
	})

	return count, err
}

// Count returns the number of active (non-expired, non-delivered) watches.
func (s *Store) Count() (int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	count := 0
	now := time.Now()

	err := s.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(watchBucket))
		if b == nil {
			return fmt.Errorf("bucket not found")
		}

		return b.ForEach(func(k, v []byte) error {
			var data WatchData
			if err := json.Unmarshal(v, &data); err != nil {
				return nil
			}

			// Skip expired or delivered
			if data.ExpiresAt.Before(now) || data.Delivered {
				return nil
			}

			count++
			return nil
		})
	})

	return count, err
}

// Path returns the filesystem path to the database.
func (s *Store) Path() string {
	return s.path
}
