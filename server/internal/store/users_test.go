package store_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/tiennm99/dleague/server/internal/store"
)

func TestUserRepo_UpsertAndGet(t *testing.T) {
	uri := mongoTestURI(t)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	client, err := store.Connect(ctx, uri)
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer func() { _ = client.Disconnect(context.Background()) }()

	db := client.Database()
	repo := store.NewUserRepo(db)

	// Use a unique UID per test run to avoid collisions in shared test DB.
	uid := fmt.Sprintf("test-uid-%d", time.Now().UnixNano())
	profile := store.UserProfile{
		DisplayName: "Test Player",
		AvatarURL:   "https://example.com/avatar.png",
		Verified:    true,
	}

	// Upsert should succeed.
	if err := repo.UpsertByUID(ctx, uid, profile); err != nil {
		t.Fatalf("UpsertByUID: %v", err)
	}

	// GetByUID should return the same document.
	got, err := repo.GetByUID(ctx, uid)
	if err != nil {
		t.Fatalf("GetByUID: %v", err)
	}
	if got == nil {
		t.Fatal("GetByUID: returned nil, want document")
	}
	if got.ID != uid {
		t.Errorf("ID = %q, want %q", got.ID, uid)
	}
	if got.DisplayName != profile.DisplayName {
		t.Errorf("DisplayName = %q, want %q", got.DisplayName, profile.DisplayName)
	}
	if got.AvatarURL != profile.AvatarURL {
		t.Errorf("AvatarURL = %q, want %q", got.AvatarURL, profile.AvatarURL)
	}
	if !got.Verified {
		t.Error("Verified = false, want true")
	}
	if got.SchemaVersion != 1 {
		t.Errorf("SchemaVersion = %d, want 1", got.SchemaVersion)
	}

	// Second upsert updates last_login; CreatedAt should be stable.
	firstCreated := got.CreatedAt
	if err := repo.UpsertByUID(ctx, uid, profile); err != nil {
		t.Fatalf("UpsertByUID (second): %v", err)
	}
	got2, err := repo.GetByUID(ctx, uid)
	if err != nil {
		t.Fatalf("GetByUID (second): %v", err)
	}
	if !got2.CreatedAt.Equal(firstCreated) {
		t.Errorf("CreatedAt changed on re-upsert: %v → %v", firstCreated, got2.CreatedAt)
	}

	// Cleanup: delete test document (best-effort; test DB is ephemeral).
	_, _ = db.Collection("users").DeleteOne(ctx, map[string]string{"_id": uid})
}

func TestUserRepo_GetByUID_NotFound(t *testing.T) {
	uri := mongoTestURI(t)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client, err := store.Connect(ctx, uri)
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer func() { _ = client.Disconnect(context.Background()) }()

	repo := store.NewUserRepo(client.Database())

	// Non-existent UID must return (nil, nil).
	got, err := repo.GetByUID(ctx, "uid-does-not-exist-xyz")
	if err != nil {
		t.Fatalf("GetByUID(missing): %v", err)
	}
	if got != nil {
		t.Errorf("GetByUID(missing): got %+v, want nil", got)
	}
}

func TestUserRepo_EmptyUID_Rejected(t *testing.T) {
	uri := mongoTestURI(t)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client, err := store.Connect(ctx, uri)
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer func() { _ = client.Disconnect(context.Background()) }()

	repo := store.NewUserRepo(client.Database())

	// Empty UID must be rejected with ErrEmptyUID.
	if err := repo.UpsertByUID(ctx, "", store.UserProfile{}); err == nil {
		t.Error("UpsertByUID(empty uid): expected error, got nil")
	}
	if _, err := repo.GetByUID(ctx, ""); err == nil {
		t.Error("GetByUID(empty uid): expected error, got nil")
	}
}
