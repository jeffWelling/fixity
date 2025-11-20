package database_test

import (
	"context"
	"testing"

	"github.com/jeffanddom/fixity/internal/database"
	"github.com/jeffanddom/fixity/tests/testutil"
	"golang.org/x/crypto/bcrypt"
)

func TestUserRepository_Create(t *testing.T) {
	db := testutil.NewTestDB(t)
	defer db.Close()
	defer testutil.CleanupDB(t, db)

	t.Run("creates user successfully", func(t *testing.T) {
		hash, _ := bcrypt.GenerateFromPassword([]byte("password123"), bcrypt.DefaultCost)

		user := &database.User{
			Username:     "testuser",
			PasswordHash: string(hash),
			IsAdmin:      false,
		}

		err := db.Users.Create(context.Background(), user)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if user.ID == 0 {
			t.Error("expected ID to be set")
		}

		if user.CreatedAt.IsZero() {
			t.Error("expected CreatedAt to be set")
		}
	})

	t.Run("creates admin user", func(t *testing.T) {
		hash, _ := bcrypt.GenerateFromPassword([]byte("adminpass"), bcrypt.DefaultCost)
		email := "admin@example.com"

		user := &database.User{
			Username:     "admin",
			PasswordHash: string(hash),
			Email:        &email,
			IsAdmin:      true,
		}

		err := db.Users.Create(context.Background(), user)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Verify
		retrieved, err := db.Users.GetByID(context.Background(), user.ID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if !retrieved.IsAdmin {
			t.Error("expected user to be admin")
		}

		if retrieved.Email == nil || *retrieved.Email != email {
			t.Errorf("expected email %s, got %v", email, retrieved.Email)
		}
	})

	t.Run("enforces unique username constraint", func(t *testing.T) {
		username := "duplicate"
		testutil.MustCreateUser(t, db, username, false)

		// Try to create duplicate
		hash, _ := bcrypt.GenerateFromPassword([]byte("password"), bcrypt.DefaultCost)
		duplicate := &database.User{
			Username:     username,
			PasswordHash: string(hash),
		}

		err := db.Users.Create(context.Background(), duplicate)
		if err == nil {
			t.Error("expected error for duplicate username")
		}
	})

	t.Run("enforces username length constraint", func(t *testing.T) {
		hash, _ := bcrypt.GenerateFromPassword([]byte("password"), bcrypt.DefaultCost)

		// Too short
		tooShort := &database.User{
			Username:     "ab",
			PasswordHash: string(hash),
		}

		err := db.Users.Create(context.Background(), tooShort)
		if err == nil {
			t.Error("expected error for username too short")
		}
	})
}

func TestUserRepository_GetByID(t *testing.T) {
	db := testutil.NewTestDB(t)
	defer db.Close()
	defer testutil.CleanupDB(t, db)

	user := testutil.MustCreateUser(t, db, "testuser", false)

	t.Run("retrieves user by ID", func(t *testing.T) {
		retrieved, err := db.Users.GetByID(context.Background(), user.ID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if retrieved.Username != user.Username {
			t.Errorf("expected username %s, got %s", user.Username, retrieved.Username)
		}
	})

	t.Run("returns error for non-existent ID", func(t *testing.T) {
		_, err := db.Users.GetByID(context.Background(), 999999)
		if err == nil {
			t.Error("expected error for non-existent ID")
		}
	})
}

func TestUserRepository_GetByUsername(t *testing.T) {
	db := testutil.NewTestDB(t)
	defer db.Close()
	defer testutil.CleanupDB(t, db)

	username := "findme"
	user := testutil.MustCreateUser(t, db, username, false)

	t.Run("retrieves user by username", func(t *testing.T) {
		retrieved, err := db.Users.GetByUsername(context.Background(), username)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if retrieved == nil {
			t.Fatal("expected user, got nil")
		}

		if retrieved.ID != user.ID {
			t.Errorf("expected ID %d, got %d", user.ID, retrieved.ID)
		}
	})

	t.Run("returns nil for non-existent username", func(t *testing.T) {
		retrieved, err := db.Users.GetByUsername(context.Background(), "nonexistent")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if retrieved != nil {
			t.Error("expected nil for non-existent username")
		}
	})
}

func TestUserRepository_ListAll(t *testing.T) {
	db := testutil.NewTestDB(t)
	defer db.Close()
	defer testutil.CleanupDB(t, db)

	testutil.MustCreateUser(t, db, "user1", false)
	testutil.MustCreateUser(t, db, "user2", false)
	testutil.MustCreateUser(t, db, "admin", true)

	t.Run("lists all users", func(t *testing.T) {
		users, err := db.Users.ListAll(context.Background())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(users) != 3 {
			t.Errorf("expected 3 users, got %d", len(users))
		}

		// Verify sorted by username
		if users[0].Username > users[1].Username {
			t.Error("expected users to be sorted by username")
		}
	})
}

func TestUserRepository_Update(t *testing.T) {
	db := testutil.NewTestDB(t)
	defer db.Close()
	defer testutil.CleanupDB(t, db)

	user := testutil.MustCreateUser(t, db, "testuser", false)

	t.Run("updates user successfully", func(t *testing.T) {
		newEmail := "newemail@example.com"
		user.Username = "updateduser"
		user.Email = &newEmail
		user.IsAdmin = true

		err := db.Users.Update(context.Background(), user)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Verify
		updated, err := db.Users.GetByID(context.Background(), user.ID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if updated.Username != "updateduser" {
			t.Errorf("expected username updateduser, got %s", updated.Username)
		}

		if updated.Email == nil || *updated.Email != newEmail {
			t.Errorf("expected email %s, got %v", newEmail, updated.Email)
		}

		if !updated.IsAdmin {
			t.Error("expected user to be admin")
		}
	})

	t.Run("does not update password via Update method", func(t *testing.T) {
		originalHash := user.PasswordHash

		// Try to update (password shouldn't change via Update)
		user.Username = "anotherchange"
		err := db.Users.Update(context.Background(), user)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Verify password unchanged
		updated, err := db.Users.GetByID(context.Background(), user.ID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if updated.PasswordHash != originalHash {
			t.Error("password hash should not change via Update method")
		}
	})

	t.Run("returns error for non-existent user", func(t *testing.T) {
		nonExistent := &database.User{
			ID:       999999,
			Username: "nonexistent",
		}

		err := db.Users.Update(context.Background(), nonExistent)
		if err == nil {
			t.Error("expected error for non-existent user")
		}
	})
}

func TestUserRepository_UpdatePassword(t *testing.T) {
	db := testutil.NewTestDB(t)
	defer db.Close()
	defer testutil.CleanupDB(t, db)

	user := testutil.MustCreateUser(t, db, "testuser", false)
	originalHash := user.PasswordHash

	t.Run("updates password successfully", func(t *testing.T) {
		newHash, _ := bcrypt.GenerateFromPassword([]byte("newpassword"), bcrypt.DefaultCost)

		err := db.Users.UpdatePassword(context.Background(), user.ID, string(newHash))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Verify
		updated, err := db.Users.GetByID(context.Background(), user.ID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if updated.PasswordHash == originalHash {
			t.Error("expected password hash to change")
		}

		if updated.PasswordHash != string(newHash) {
			t.Error("password hash does not match new hash")
		}
	})

	t.Run("returns error for non-existent user", func(t *testing.T) {
		err := db.Users.UpdatePassword(context.Background(), 999999, "somehash")
		if err == nil {
			t.Error("expected error for non-existent user")
		}
	})
}

func TestUserRepository_UpdateLastLogin(t *testing.T) {
	db := testutil.NewTestDB(t)
	defer db.Close()
	defer testutil.CleanupDB(t, db)

	user := testutil.MustCreateUser(t, db, "testuser", false)

	t.Run("updates last login timestamp", func(t *testing.T) {
		if user.LastLogin != nil {
			t.Error("expected last_login to be nil initially")
		}

		err := db.Users.UpdateLastLogin(context.Background(), user.ID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Verify
		updated, err := db.Users.GetByID(context.Background(), user.ID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if updated.LastLogin == nil {
			t.Error("expected last_login to be set")
		}
	})
}

func TestUserRepository_Delete(t *testing.T) {
	db := testutil.NewTestDB(t)
	defer db.Close()
	defer testutil.CleanupDB(t, db)

	user := testutil.MustCreateUser(t, db, "testuser", false)

	t.Run("deletes user successfully", func(t *testing.T) {
		err := db.Users.Delete(context.Background(), user.ID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Verify deletion
		_, err = db.Users.GetByID(context.Background(), user.ID)
		if err == nil {
			t.Error("expected error when getting deleted user")
		}
	})

	t.Run("returns error for non-existent user", func(t *testing.T) {
		err := db.Users.Delete(context.Background(), 999999)
		if err == nil {
			t.Error("expected error for non-existent user")
		}
	})
}

func TestSessionRepository_Create(t *testing.T) {
	db := testutil.NewTestDB(t)
	defer db.Close()
	defer testutil.CleanupDB(t, db)

	user := testutil.MustCreateUser(t, db, "testuser", false)

	t.Run("creates session successfully", func(t *testing.T) {
		session := &database.Session{
			Token:     "test-token-123",
			UserID:    user.ID,
			ExpiresAt: testutil.TimeNow().Add(24 * testutil.Hour),
		}

		err := db.Sessions.Create(context.Background(), session)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if session.CreatedAt.IsZero() {
			t.Error("expected CreatedAt to be set")
		}
	})
}

func TestSessionRepository_GetByToken(t *testing.T) {
	db := testutil.NewTestDB(t)
	defer db.Close()
	defer testutil.CleanupDB(t, db)

	user := testutil.MustCreateUser(t, db, "testuser", false)

	token := "valid-token"
	session := &database.Session{
		Token:     token,
		UserID:    user.ID,
		ExpiresAt: testutil.TimeNow().Add(24 * testutil.Hour),
	}
	db.Sessions.Create(context.Background(), session)

	t.Run("retrieves valid session", func(t *testing.T) {
		retrieved, err := db.Sessions.GetByToken(context.Background(), token)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if retrieved == nil {
			t.Fatal("expected session, got nil")
		}

		if retrieved.UserID != user.ID {
			t.Errorf("expected user ID %d, got %d", user.ID, retrieved.UserID)
		}
	})

	t.Run("returns nil for expired session", func(t *testing.T) {
		expiredToken := "expired-token"
		expired := &database.Session{
			Token:     expiredToken,
			UserID:    user.ID,
			ExpiresAt: testutil.TimeNow().Add(-1 * testutil.Hour),
		}
		db.Sessions.Create(context.Background(), expired)

		retrieved, err := db.Sessions.GetByToken(context.Background(), expiredToken)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if retrieved != nil {
			t.Error("expected nil for expired session")
		}
	})

	t.Run("returns nil for non-existent token", func(t *testing.T) {
		retrieved, err := db.Sessions.GetByToken(context.Background(), "nonexistent")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if retrieved != nil {
			t.Error("expected nil for non-existent token")
		}
	})
}

func TestSessionRepository_Delete(t *testing.T) {
	db := testutil.NewTestDB(t)
	defer db.Close()
	defer testutil.CleanupDB(t, db)

	user := testutil.MustCreateUser(t, db, "testuser", false)

	token := "delete-me"
	session := &database.Session{
		Token:     token,
		UserID:    user.ID,
		ExpiresAt: testutil.TimeNow().Add(24 * testutil.Hour),
	}
	db.Sessions.Create(context.Background(), session)

	t.Run("deletes session successfully", func(t *testing.T) {
		err := db.Sessions.Delete(context.Background(), token)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Verify deletion
		retrieved, err := db.Sessions.GetByToken(context.Background(), token)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if retrieved != nil {
			t.Error("expected nil after deletion")
		}
	})
}

func TestSessionRepository_DeleteExpired(t *testing.T) {
	db := testutil.NewTestDB(t)
	defer db.Close()
	defer testutil.CleanupDB(t, db)

	user := testutil.MustCreateUser(t, db, "testuser", false)

	// Create valid session
	valid := &database.Session{
		Token:     "valid",
		UserID:    user.ID,
		ExpiresAt: testutil.TimeNow().Add(24 * testutil.Hour),
	}
	db.Sessions.Create(context.Background(), valid)

	// Create expired sessions
	expired1 := &database.Session{
		Token:     "expired1",
		UserID:    user.ID,
		ExpiresAt: testutil.TimeNow().Add(-1 * testutil.Hour),
	}
	db.Sessions.Create(context.Background(), expired1)

	expired2 := &database.Session{
		Token:     "expired2",
		UserID:    user.ID,
		ExpiresAt: testutil.TimeNow().Add(-2 * testutil.Hour),
	}
	db.Sessions.Create(context.Background(), expired2)

	t.Run("deletes only expired sessions", func(t *testing.T) {
		count, err := db.Sessions.DeleteExpired(context.Background())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if count != 2 {
			t.Errorf("expected 2 expired sessions deleted, got %d", count)
		}

		// Verify valid session still exists
		retrieved, err := db.Sessions.GetByToken(context.Background(), "valid")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if retrieved == nil {
			t.Error("expected valid session to still exist")
		}
	})
}

func TestSessionRepository_DeleteForUser(t *testing.T) {
	db := testutil.NewTestDB(t)
	defer db.Close()
	defer testutil.CleanupDB(t, db)

	user1 := testutil.MustCreateUser(t, db, "user1", false)
	user2 := testutil.MustCreateUser(t, db, "user2", false)

	// Create sessions for both users
	session1 := &database.Session{
		Token:     "user1-session",
		UserID:    user1.ID,
		ExpiresAt: testutil.TimeNow().Add(24 * testutil.Hour),
	}
	db.Sessions.Create(context.Background(), session1)

	session2 := &database.Session{
		Token:     "user2-session",
		UserID:    user2.ID,
		ExpiresAt: testutil.TimeNow().Add(24 * testutil.Hour),
	}
	db.Sessions.Create(context.Background(), session2)

	t.Run("deletes sessions for specific user only", func(t *testing.T) {
		err := db.Sessions.DeleteForUser(context.Background(), user1.ID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Verify user1 session deleted
		retrieved1, err := db.Sessions.GetByToken(context.Background(), "user1-session")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if retrieved1 != nil {
			t.Error("expected user1 session to be deleted")
		}

		// Verify user2 session still exists
		retrieved2, err := db.Sessions.GetByToken(context.Background(), "user2-session")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if retrieved2 == nil {
			t.Error("expected user2 session to still exist")
		}
	})
}
