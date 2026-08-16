package watch

import (
	"path/filepath"
	"testing"
	"time"

	"go.etcd.io/bbolt"
)

func TestNewStore(t *testing.T) {
	// Test with a temporary path
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	store, err := NewStore(dbPath)
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}
	defer store.Close()

	if store == nil {
		t.Fatal("store is nil")
	}

	if store.Path() != dbPath {
		t.Errorf("Path() = %s, want %s", store.Path(), dbPath)
	}

	// Verify bucket was created
	if err := store.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(watchBucket))
		if b == nil {
			t.Error("watch bucket was not created")
		}
		return nil
	}); err != nil {
		t.Errorf("failed to verify bucket: %v", err)
	}
}

func TestStore_CreateAndGet(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	watch := &WatchData{
		ID:         "test-id",
		Domain:     "example.com",
		WebhookURL: "https://example.com/webhook",
		Secret:     "test-secret",
		CreatedAt:  time.Now(),
		ExpiresAt:  time.Now().Add(24 * time.Hour),
		LastStatus: "taken",
		ClientIP:   "127.0.0.1",
	}

	// Create watch
	if err := store.Create(watch); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// Get watch
	retrieved, err := store.Get("test-id")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	if retrieved.ID != watch.ID {
		t.Errorf("ID = %s, want %s", retrieved.ID, watch.ID)
	}
	if retrieved.Domain != watch.Domain {
		t.Errorf("Domain = %s, want %s", retrieved.Domain, watch.Domain)
	}
	if retrieved.WebhookURL != watch.WebhookURL {
		t.Errorf("WebhookURL = %s, want %s", retrieved.WebhookURL, watch.WebhookURL)
	}
	if retrieved.Secret != watch.Secret {
		t.Errorf("Secret = %s, want %s", retrieved.Secret, watch.Secret)
	}
}

func TestStore_CreateDuplicate(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	now := time.Now()
	watch1 := &WatchData{
		ID:         "id-1",
		Domain:     "example.com",
		WebhookURL: "https://example.com/webhook",
		Secret:     "secret-1",
		CreatedAt:  now,
		ExpiresAt:  now.Add(24 * time.Hour),
		ClientIP:   "127.0.0.1",
	}

	if err := store.Create(watch1); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// Try to create duplicate (same domain + webhook + not expired)
	watch2 := &WatchData{
		ID:         "id-2",
		Domain:     "example.com",
		WebhookURL: "https://example.com/webhook",
		Secret:     "secret-2",
		CreatedAt:  now.Add(1 * time.Hour),
		ExpiresAt:  now.Add(48 * time.Hour),
		ClientIP:   "127.0.0.1",
	}

	err := store.Create(watch2)
	if err == nil {
		t.Error("Create should have failed for duplicate watch")
	}
	if err != nil && err.Error() != "watch already exists for this domain and webhook" {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestStore_CreateExpiredDuplicate(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	now := time.Now()
	watch1 := &WatchData{
		ID:         "id-1",
		Domain:     "example.com",
		WebhookURL: "https://example.com/webhook",
		Secret:     "secret-1",
		CreatedAt:  now.Add(-48 * time.Hour),
		ExpiresAt:  now.Add(-1 * time.Hour), // Expired
		ClientIP:   "127.0.0.1",
	}

	if err := store.Create(watch1); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// Creating same domain + webhook should succeed if previous watch is expired
	watch2 := &WatchData{
		ID:         "id-2",
		Domain:     "example.com",
		WebhookURL: "https://example.com/webhook",
		Secret:     "secret-2",
		CreatedAt:  now,
		ExpiresAt:  now.Add(24 * time.Hour),
		ClientIP:   "127.0.0.1",
	}

	if err := store.Create(watch2); err != nil {
		t.Errorf("Create should have succeeded for expired duplicate: %v", err)
	}
}

func TestStore_Update(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	watch := &WatchData{
		ID:         "test-id",
		Domain:     "example.com",
		WebhookURL: "https://example.com/webhook",
		Secret:     "test-secret",
		CreatedAt:  time.Now(),
		ExpiresAt:  time.Now().Add(24 * time.Hour),
		LastStatus: "taken",
		ClientIP:   "127.0.0.1",
	}

	if err := store.Create(watch); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// Update watch
	watch.LastStatus = "available"
	watch.Delivered = true
	if err := store.Update(watch); err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	// Verify update
	retrieved, err := store.Get("test-id")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	if retrieved.LastStatus != "available" {
		t.Errorf("LastStatus = %s, want available", retrieved.LastStatus)
	}
	if !retrieved.Delivered {
		t.Error("Delivered = false, want true")
	}
}

func TestStore_Delete(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	watch := &WatchData{
		ID:         "test-id",
		Domain:     "example.com",
		WebhookURL: "https://example.com/webhook",
		Secret:     "test-secret",
		CreatedAt:  time.Now(),
		ExpiresAt:  time.Now().Add(24 * time.Hour),
		ClientIP:   "127.0.0.1",
	}

	if err := store.Create(watch); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// Delete watch
	if err := store.Delete("test-id"); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	// Verify deleted
	_, err := store.Get("test-id")
	if err == nil {
		t.Error("Get should have failed after Delete")
	}
}

func TestStore_ListAll(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	now := time.Now()

	// Create multiple watches
	watches := []*WatchData{
		{
			ID:         "id-1",
			Domain:     "example1.com",
			WebhookURL: "https://example.com/webhook1",
			Secret:     "secret-1",
			CreatedAt:  now,
			ExpiresAt:  now.Add(24 * time.Hour),
			ClientIP:   "127.0.0.1",
		},
		{
			ID:         "id-2",
			Domain:     "example2.com",
			WebhookURL: "https://example.com/webhook2",
			Secret:     "secret-2",
			CreatedAt:  now,
			ExpiresAt:  now.Add(24 * time.Hour),
			ClientIP:   "127.0.0.1",
		},
		{
			ID:         "id-3",
			Domain:     "example3.com",
			WebhookURL: "https://example.com/webhook3",
			Secret:     "secret-3",
			CreatedAt:  now,
			ExpiresAt:  now.Add(-1 * time.Hour), // Expired
			ClientIP:   "127.0.0.1",
		},
		{
			ID:         "id-4",
			Domain:     "example4.com",
			WebhookURL: "https://example.com/webhook4",
			Secret:     "secret-4",
			CreatedAt:  now,
			ExpiresAt:  now.Add(24 * time.Hour),
			Delivered:  true, // Already delivered
			ClientIP:   "127.0.0.1",
		},
	}

	for _, w := range watches {
		if err := store.Create(w); err != nil {
			t.Fatalf("Create failed for %s: %v", w.ID, err)
		}
	}

	// List all should return only non-expired, non-delivered watches
	all, err := store.ListAll()
	if err != nil {
		t.Fatalf("ListAll failed: %v", err)
	}

	if len(all) != 2 {
		t.Errorf("ListAll returned %d watches, want 2 (only non-expired, non-delivered)", len(all))
	}

	// Verify which watches were returned
	ids := make(map[string]bool)
	for _, w := range all {
		ids[w.ID] = true
	}

	if !ids["id-1"] {
		t.Error("id-1 should be in ListAll")
	}
	if !ids["id-2"] {
		t.Error("id-2 should be in ListAll")
	}
	if ids["id-3"] {
		t.Error("id-3 (expired) should not be in ListAll")
	}
	if ids["id-4"] {
		t.Error("id-4 (delivered) should not be in ListAll")
	}
}

func TestStore_ListByIP(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	now := time.Now()
	since := now.Add(-24 * time.Hour)

	// Create watches from different IPs
	watches := []*WatchData{
		{
			ID:         "id-1",
			Domain:     "example1.com",
			WebhookURL: "https://example.com/webhook1",
			Secret:     "secret-1",
			CreatedAt:  now,
			ExpiresAt:  now.Add(24 * time.Hour),
			ClientIP:   "192.168.1.1",
		},
		{
			ID:         "id-2",
			Domain:     "example2.com",
			WebhookURL: "https://example.com/webhook2",
			Secret:     "secret-2",
			CreatedAt:  now,
			ExpiresAt:  now.Add(24 * time.Hour),
			ClientIP:   "192.168.1.1",
		},
		{
			ID:         "id-3",
			Domain:     "example3.com",
			WebhookURL: "https://example.com/webhook3",
			Secret:     "secret-3",
			CreatedAt:  now,
			ExpiresAt:  now.Add(24 * time.Hour),
			ClientIP:   "192.168.1.2",
		},
		{
			ID:         "id-4",
			Domain:     "example4.com",
			WebhookURL: "https://example.com/webhook4",
			Secret:     "secret-4",
			CreatedAt:  now.Add(-48 * time.Hour), // Too old
			ExpiresAt:  now.Add(24 * time.Hour),
			ClientIP:   "192.168.1.1",
		},
	}

	for _, w := range watches {
		if err := store.Create(w); err != nil {
			t.Fatalf("Create failed for %s: %v", w.ID, err)
		}
	}

	// List by IP should return only matching IPs within time window
	list, err := store.ListByIP("192.168.1.1", since)
	if err != nil {
		t.Fatalf("ListByIP failed: %v", err)
	}

	if len(list) != 2 {
		t.Errorf("ListByIP returned %d watches, want 2", len(list))
	}

	// Verify which watches were returned
	ids := make(map[string]bool)
	for _, w := range list {
		ids[w.ID] = true
	}

	if !ids["id-1"] {
		t.Error("id-1 should be in ListByIP")
	}
	if !ids["id-2"] {
		t.Error("id-2 should be in ListByIP")
	}
	if ids["id-3"] {
		t.Error("id-3 (different IP) should not be in ListByIP")
	}
	if ids["id-4"] {
		t.Error("id-4 (too old) should not be in ListByIP")
	}
}

func TestStore_CleanupExpired(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	now := time.Now()

	// Create watches
	watches := []*WatchData{
		{
			ID:         "id-1",
			Domain:     "example1.com",
			WebhookURL: "https://example.com/webhook1",
			Secret:     "secret-1",
			CreatedAt:  now,
			ExpiresAt:  now.Add(24 * time.Hour), // Not expired
			ClientIP:   "127.0.0.1",
		},
		{
			ID:         "id-2",
			Domain:     "example2.com",
			WebhookURL: "https://example.com/webhook2",
			Secret:     "secret-2",
			CreatedAt:  now,
			ExpiresAt:  now.Add(-1 * time.Hour), // Expired
			ClientIP:   "127.0.0.1",
		},
		{
			ID:         "id-3",
			Domain:     "example3.com",
			WebhookURL: "https://example.com/webhook3",
			Secret:     "secret-3",
			CreatedAt:  now,
			ExpiresAt:  now.Add(24 * time.Hour),
			Delivered:  true, // Delivered
			ClientIP:   "127.0.0.1",
		},
	}

	for _, w := range watches {
		if err := store.Create(w); err != nil {
			t.Fatalf("Create failed for %s: %v", w.ID, err)
		}
	}

	// Cleanup should remove 2 watches (expired + delivered)
	count, err := store.CleanupExpired()
	if err != nil {
		t.Fatalf("CleanupExpired failed: %v", err)
	}

	if count != 2 {
		t.Errorf("CleanupExpired removed %d watches, want 2", count)
	}

	// Verify only id-1 remains
	all, err := store.ListAll()
	if err != nil {
		t.Fatalf("ListAll failed: %v", err)
	}

	if len(all) != 1 {
		t.Errorf("ListAll returned %d watches, want 1", len(all))
	}

	if len(all) > 0 && all[0].ID != "id-1" {
		t.Errorf("Remaining watch ID = %s, want id-1", all[0].ID)
	}
}

func TestStore_Count(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	now := time.Now()

	// Initially count should be 0
	count, err := store.Count()
	if err != nil {
		t.Fatalf("Count failed: %v", err)
	}
	if count != 0 {
		t.Errorf("Count = %d, want 0", count)
	}

	// Add active watch
	watch1 := &WatchData{
		ID:         "id-1",
		Domain:     "example1.com",
		WebhookURL: "https://example.com/webhook1",
		Secret:     "secret-1",
		CreatedAt:  now,
		ExpiresAt:  now.Add(24 * time.Hour),
		ClientIP:   "127.0.0.1",
	}
	if err := store.Create(watch1); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	count, err = store.Count()
	if err != nil {
		t.Fatalf("Count failed: %v", err)
	}
	if count != 1 {
		t.Errorf("Count = %d, want 1", count)
	}

	// Add expired watch
	watch2 := &WatchData{
		ID:         "id-2",
		Domain:     "example2.com",
		WebhookURL: "https://example.com/webhook2",
		Secret:     "secret-2",
		CreatedAt:  now,
		ExpiresAt:  now.Add(-1 * time.Hour),
		ClientIP:   "127.0.0.1",
	}
	if err := store.Create(watch2); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// Count should still be 1 (expired watches don't count)
	count, err = store.Count()
	if err != nil {
		t.Fatalf("Count failed: %v", err)
	}
	if count != 1 {
		t.Errorf("Count = %d, want 1 (expired watches excluded)", count)
	}

	// Add delivered watch
	watch3 := &WatchData{
		ID:         "id-3",
		Domain:     "example3.com",
		WebhookURL: "https://example.com/webhook3",
		Secret:     "secret-3",
		CreatedAt:  now,
		ExpiresAt:  now.Add(24 * time.Hour),
		Delivered:  true,
		ClientIP:   "127.0.0.1",
	}
	if err := store.Create(watch3); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// Count should still be 1 (delivered watches don't count)
	count, err = store.Count()
	if err != nil {
		t.Fatalf("Count failed: %v", err)
	}
	if count != 1 {
		t.Errorf("Count = %d, want 1 (delivered watches excluded)", count)
	}
}

func TestStore_Close(t *testing.T) {
	store := newTestStore(t)

	// Create a watch to ensure database is initialized
	watch := &WatchData{
		ID:         "test-id",
		Domain:     "example.com",
		WebhookURL: "https://example.com/webhook",
		Secret:     "test-secret",
		CreatedAt:  time.Now(),
		ExpiresAt:  time.Now().Add(24 * time.Hour),
		ClientIP:   "127.0.0.1",
	}

	if err := store.Create(watch); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// Close store
	if err := store.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	// Operations after close should fail
	_, err := store.Get("test-id")
	if err == nil {
		t.Error("Operation after Close should have failed")
	}
}

// newTestStore creates a new test store in a temporary directory.
func newTestStore(t *testing.T) *Store {
	t.Helper()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	store, err := NewStore(dbPath)
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}

	return store
}
