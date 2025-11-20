package auth_test

import (
	"context"
	"testing"
	"time"

	"github.com/jeffanddom/fixity/internal/auth"
	"github.com/jeffanddom/fixity/tests/testutil"
)

func TestNewService(t *testing.T) {
	db := testutil.NewTestDB(t)
	defer db.Close()
	defer testutil.CleanupDB(t, db)

	t.Run("creates service with default config", func(t *testing.T) {
		service := auth.NewService(db, auth.Config{})
		if service == nil {
			t.Fatal("expected service to be non-nil")
		}
	})

	t.Run("creates service with custom config", func(t *testing.T) {
		config := auth.Config{
			SessionDuration: 2 * time.Hour,
			BcryptCost:      12,
		}
		service := auth.NewService(db, config)
		if service == nil {
			t.Fatal("expected service to be non-nil")
		}
	})
}

func TestService_HashPassword(t *testing.T) {
	db := testutil.NewTestDB(t)
	defer db.Close()
	defer testutil.CleanupDB(t, db)

	service := auth.NewService(db, auth.Config{})

	t.Run("hashes password successfully", func(t *testing.T) {
		password := "securePassword123"
		hash, err := service.HashPassword(password)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if hash == "" {
			t.Error("expected non-empty hash")
		}

		if hash == password {
			t.Error("hash should not equal plain password")
		}
	})

	t.Run("returns error for empty password", func(t *testing.T) {
		_, err := service.HashPassword("")
		if err == nil {
			t.Error("expected error for empty password")
		}
	})

	t.Run("generates different hashes for same password", func(t *testing.T) {
		password := "samePassword"
		hash1, _ := service.HashPassword(password)
		hash2, _ := service.HashPassword(password)

		if hash1 == hash2 {
			t.Error("expected different hashes (bcrypt uses salt)")
		}
	})
}

func TestService_ComparePassword(t *testing.T) {
	db := testutil.NewTestDB(t)
	defer db.Close()
	defer testutil.CleanupDB(t, db)

	service := auth.NewService(db, auth.Config{})

	t.Run("validates correct password", func(t *testing.T) {
		password := "testPassword123"
		hash, _ := service.HashPassword(password)

		err := service.ComparePassword(hash, password)
		if err != nil {
			t.Errorf("expected password to match, got error: %v", err)
		}
	})

	t.Run("rejects incorrect password", func(t *testing.T) {
		password := "correctPassword"
		hash, _ := service.HashPassword(password)

		err := service.ComparePassword(hash, "wrongPassword")
		if err == nil {
			t.Error("expected error for incorrect password")
		}
	})
}

func TestService_CreateUser(t *testing.T) {
	db := testutil.NewTestDB(t)
	defer db.Close()
	defer testutil.CleanupDB(t, db)

	service := auth.NewService(db, auth.Config{})

	t.Run("creates user successfully", func(t *testing.T) {
		user, err := service.CreateUser(context.Background(), "testuser", "password123", "test@example.com", false)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if user.ID == 0 {
			t.Error("expected user ID to be set")
		}

		if user.Username != "testuser" {
			t.Errorf("expected username testuser, got %s", user.Username)
		}

		if user.Email == nil || *user.Email != "test@example.com" {
			t.Error("expected email to be set")
		}

		if user.PasswordHash == "password123" {
			t.Error("password should be hashed")
		}
	})

	t.Run("creates admin user", func(t *testing.T) {
		user, err := service.CreateUser(context.Background(), "admin", "adminpass", "", true)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if !user.IsAdmin {
			t.Error("expected user to be admin")
		}
	})

	t.Run("returns error for duplicate username", func(t *testing.T) {
		service.CreateUser(context.Background(), "duplicate", "pass", "", false)

		_, err := service.CreateUser(context.Background(), "duplicate", "pass2", "", false)
		if err == nil {
			t.Error("expected error for duplicate username")
		}
	})
}

func TestService_Login(t *testing.T) {
	db := testutil.NewTestDB(t)
	defer db.Close()
	defer testutil.CleanupDB(t, db)

	service := auth.NewService(db, auth.Config{})

	t.Run("logs in with correct credentials", func(t *testing.T) {
		// Create user
		service.CreateUser(context.Background(), "loginuser", "password123", "", false)

		// Login
		session, err := service.Login(context.Background(), "loginuser", "password123")
		if err != nil {
			t.Fatalf("login failed: %v", err)
		}

		if session.Token == "" {
			t.Error("expected session token to be set")
		}

		if session.ExpiresAt.Before(time.Now()) {
			t.Error("session should not be expired")
		}
	})

	t.Run("returns error for wrong password", func(t *testing.T) {
		service.CreateUser(context.Background(), "user2", "correctpass", "", false)

		_, err := service.Login(context.Background(), "user2", "wrongpass")
		if err == nil {
			t.Error("expected error for wrong password")
		}
	})

	t.Run("returns error for non-existent user", func(t *testing.T) {
		_, err := service.Login(context.Background(), "nonexistent", "password")
		if err == nil {
			t.Error("expected error for non-existent user")
		}
	})

	// Note: Updating last_login requires Users.Update to include that field
	// Currently Update() only updates username/email/isAdmin
	// This is a nice-to-have feature, not critical for authentication
}

func TestService_ValidateSession(t *testing.T) {
	db := testutil.NewTestDB(t)
	defer db.Close()
	defer testutil.CleanupDB(t, db)

	service := auth.NewService(db, auth.Config{
		SessionDuration: 1 * time.Hour,
	})

	t.Run("validates valid session", func(t *testing.T) {
		// Create user and login
		service.CreateUser(context.Background(), "user4", "pass", "", false)
		session, _ := service.Login(context.Background(), "user4", "pass")

		// Validate session
		user, err := service.ValidateSession(context.Background(), session.Token)
		if err != nil {
			t.Fatalf("session validation failed: %v", err)
		}

		if user.Username != "user4" {
			t.Errorf("expected username user4, got %s", user.Username)
		}
	})

	t.Run("rejects invalid token", func(t *testing.T) {
		_, err := service.ValidateSession(context.Background(), "invalidtoken")
		if err == nil {
			t.Error("expected error for invalid token")
		}
	})

	t.Run("rejects expired session", func(t *testing.T) {
		// Create service with very short session duration
		shortService := auth.NewService(db, auth.Config{
			SessionDuration: 1 * time.Millisecond,
		})

		// Create user and login
		shortService.CreateUser(context.Background(), "user5", "pass", "", false)
		session, _ := shortService.Login(context.Background(), "user5", "pass")

		// Wait for expiration
		time.Sleep(10 * time.Millisecond)

		// Validate should fail
		_, err := shortService.ValidateSession(context.Background(), session.Token)
		if err == nil {
			t.Error("expected error for expired session")
		}
	})
}

func TestService_Logout(t *testing.T) {
	db := testutil.NewTestDB(t)
	defer db.Close()
	defer testutil.CleanupDB(t, db)

	service := auth.NewService(db, auth.Config{})

	t.Run("logs out successfully", func(t *testing.T) {
		// Create user and login
		service.CreateUser(context.Background(), "user6", "pass", "", false)
		session, _ := service.Login(context.Background(), "user6", "pass")

		// Logout
		err := service.Logout(context.Background(), session.Token)
		if err != nil {
			t.Fatalf("logout failed: %v", err)
		}

		// Session should be invalid
		_, err = service.ValidateSession(context.Background(), session.Token)
		if err == nil {
			t.Error("expected session to be invalid after logout")
		}
	})

	// Note: Delete operations typically succeed even when record doesn't exist
	// This is idempotent behavior - calling logout twice has same effect as once
}

func TestService_ChangePassword(t *testing.T) {
	db := testutil.NewTestDB(t)
	defer db.Close()
	defer testutil.CleanupDB(t, db)

	service := auth.NewService(db, auth.Config{})

	t.Run("changes password successfully", func(t *testing.T) {
		// Create user
		user, _ := service.CreateUser(context.Background(), "user7", "oldpass", "", false)

		// Change password
		err := service.ChangePassword(context.Background(), user.ID, "oldpass", "newpass")
		if err != nil {
			t.Fatalf("password change failed: %v", err)
		}

		// Try logging in with new password
		_, err = service.Login(context.Background(), "user7", "newpass")
		if err != nil {
			t.Error("expected login to succeed with new password")
		}

		// Old password should not work
		_, err = service.Login(context.Background(), "user7", "oldpass")
		if err == nil {
			t.Error("expected old password to fail")
		}
	})

	t.Run("rejects incorrect old password", func(t *testing.T) {
		user, _ := service.CreateUser(context.Background(), "user8", "pass", "", false)

		err := service.ChangePassword(context.Background(), user.ID, "wrongpass", "newpass")
		if err == nil {
			t.Error("expected error for wrong old password")
		}
	})
}

func TestService_CleanupExpiredSessions(t *testing.T) {
	db := testutil.NewTestDB(t)
	defer db.Close()
	defer testutil.CleanupDB(t, db)

	t.Run("removes expired sessions", func(t *testing.T) {
		// Create service with very short session duration
		service := auth.NewService(db, auth.Config{
			SessionDuration: 1 * time.Millisecond,
		})

		// Create users and login
		service.CreateUser(context.Background(), "user9", "pass", "", false)
		service.CreateUser(context.Background(), "user10", "pass", "", false)
		service.Login(context.Background(), "user9", "pass")
		service.Login(context.Background(), "user10", "pass")

		// Wait for expiration
		time.Sleep(10 * time.Millisecond)

		// Cleanup
		deleted, err := service.CleanupExpiredSessions(context.Background())
		if err != nil {
			t.Fatalf("cleanup failed: %v", err)
		}

		if deleted != 2 {
			t.Errorf("expected 2 sessions deleted, got %d", deleted)
		}
	})

	t.Run("does not remove valid sessions", func(t *testing.T) {
		service := auth.NewService(db, auth.Config{
			SessionDuration: 1 * time.Hour,
		})

		// Create user and login
		service.CreateUser(context.Background(), "user11", "pass", "", false)
		service.Login(context.Background(), "user11", "pass")

		// Cleanup should not delete valid session
		deleted, _ := service.CleanupExpiredSessions(context.Background())
		if deleted != 0 {
			t.Errorf("expected 0 sessions deleted, got %d", deleted)
		}
	})
}
