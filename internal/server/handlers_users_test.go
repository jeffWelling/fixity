package server

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"testing"
)

// Helper function to create an admin user
func createAdminUser(t *testing.T, server *Server) (string, string) {
	user, err := server.auth.CreateUser(context.Background(), "admin", "password", "", true)
	if err != nil {
		t.Fatalf("failed to create admin user: %v", err)
	}

	session, err := server.auth.Login(context.Background(), "admin", "password")
	if err != nil {
		t.Fatalf("failed to login admin: %v", err)
	}

	return fmt.Sprintf("%d", user.ID), session.Token
}

// Helper function to create a regular user
func createRegularUser(t *testing.T, server *Server, username string) (string, string) {
	user, err := server.auth.CreateUser(context.Background(), username, "password", "", false)
	if err != nil {
		t.Fatalf("failed to create regular user: %v", err)
	}

	session, err := server.auth.Login(context.Background(), username, "password")
	if err != nil {
		t.Fatalf("failed to login regular user: %v", err)
	}

	return fmt.Sprintf("%d", user.ID), session.Token
}

func TestHandleListUsers(t *testing.T) {
	server := setupTestServer(t)

	t.Run("displays users list as admin", func(t *testing.T) {
		adminID, adminToken := createAdminUser(t, server)
		defer server.db.Users.Delete(context.Background(), mustParseInt64(adminID))

		w, _ := makeAuthenticatedRequest(server, http.MethodGet, "/users", adminToken, nil)

		resp := w.Result()
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("expected status 200, got %d", resp.StatusCode)
		}

		body := w.Body.String()
		if !strings.Contains(body, "Users") {
			t.Error("response should contain 'Users' title")
		}
		if !strings.Contains(body, "admin") {
			t.Error("response should contain admin username")
		}
	})

	t.Run("denies access to regular user", func(t *testing.T) {
		userID, userToken := createRegularUser(t, server, "regularuser")
		defer server.db.Users.Delete(context.Background(), mustParseInt64(userID))

		w, _ := makeAuthenticatedRequest(server, http.MethodGet, "/users", userToken, nil)

		resp := w.Result()
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusForbidden {
			t.Errorf("expected status 403, got %d", resp.StatusCode)
		}
	})
}

func TestHandleNewUserPage(t *testing.T) {
	server := setupTestServer(t)

	t.Run("displays user creation form as admin", func(t *testing.T) {
		adminID, adminToken := createAdminUser(t, server)
		defer server.db.Users.Delete(context.Background(), mustParseInt64(adminID))

		w, _ := makeAuthenticatedRequest(server, http.MethodGet, "/users/new", adminToken, nil)

		resp := w.Result()
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("expected status 200, got %d", resp.StatusCode)
		}

		body := w.Body.String()
		if !strings.Contains(body, "Add New User") || !strings.Contains(body, "Create User") {
			t.Error("response should contain user creation form title")
		}
		if !strings.Contains(body, `name="username"`) {
			t.Error("response should contain username input")
		}
		if !strings.Contains(body, `name="password"`) {
			t.Error("response should contain password input")
		}
		if !strings.Contains(body, `name="is_admin"`) {
			t.Error("response should contain is_admin checkbox")
		}
	})

	t.Run("denies access to regular user", func(t *testing.T) {
		userID, userToken := createRegularUser(t, server, "regularuser2")
		defer server.db.Users.Delete(context.Background(), mustParseInt64(userID))

		w, _ := makeAuthenticatedRequest(server, http.MethodGet, "/users/new", userToken, nil)

		resp := w.Result()
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusForbidden {
			t.Errorf("expected status 403, got %d", resp.StatusCode)
		}
	})
}

func TestHandleCreateUser(t *testing.T) {
	server := setupTestServer(t)

	t.Run("creates regular user as admin", func(t *testing.T) {
		adminID, adminToken := createAdminUser(t, server)
		defer server.db.Users.Delete(context.Background(), mustParseInt64(adminID))

		form := url.Values{}
		form.Add("username", "newuser")
		form.Add("password", "newpassword")
		form.Add("password_confirm", "newpassword")
		form.Add("email", "newuser@example.com")

		w, _ := makeAuthenticatedRequest(server, http.MethodPost, "/users", adminToken, form)

		resp := w.Result()
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusSeeOther {
			body := w.Body.String()
			t.Fatalf("expected status 303, got %d. Body: %s", resp.StatusCode, body)
		}

		location := resp.Header.Get("Location")
		if location != "/users" {
			t.Errorf("expected redirect to /users, got %s", location)
		}

		// Verify user was created
		user, err := server.db.Users.GetByUsername(context.Background(), "newuser")
		if err != nil || user == nil {
			t.Error("user should have been created")
		} else {
			if user.IsAdmin {
				t.Error("user should not be admin by default")
			}
			if user.Email == nil || *user.Email != "newuser@example.com" {
				t.Error("user email should be set")
			}
			server.db.Users.Delete(context.Background(), user.ID)
		}
	})

	t.Run("creates admin user as admin", func(t *testing.T) {
		adminID, adminToken := createAdminUser(t, server)
		defer server.db.Users.Delete(context.Background(), mustParseInt64(adminID))

		form := url.Values{}
		form.Add("username", "newadmin")
		form.Add("password", "adminpassword")
		form.Add("password_confirm", "adminpassword")
		form.Add("is_admin", "true")

		w, _ := makeAuthenticatedRequest(server, http.MethodPost, "/users", adminToken, form)

		resp := w.Result()
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusSeeOther {
			t.Errorf("expected status 303, got %d", resp.StatusCode)
		}

		// Verify admin user was created
		user, err := server.db.Users.GetByUsername(context.Background(), "newadmin")
		if err != nil || user == nil {
			t.Error("admin user should have been created")
		} else {
			if !user.IsAdmin {
				t.Error("user should be admin")
			}
			server.db.Users.Delete(context.Background(), user.ID)
		}
	})

	t.Run("denies access to regular user", func(t *testing.T) {
		userID, userToken := createRegularUser(t, server, "regularuser3")
		defer server.db.Users.Delete(context.Background(), mustParseInt64(userID))

		form := url.Values{}
		form.Add("username", "shouldnotbecreated")
		form.Add("password", "password")
		form.Add("password_confirm", "password")

		w, _ := makeAuthenticatedRequest(server, http.MethodPost, "/users", userToken, form)

		resp := w.Result()
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusForbidden {
			t.Errorf("expected status 403, got %d", resp.StatusCode)
		}
	})

	t.Run("rejects duplicate username", func(t *testing.T) {
		adminID, adminToken := createAdminUser(t, server)
		defer server.db.Users.Delete(context.Background(), mustParseInt64(adminID))

		// Create first user
		form := url.Values{}
		form.Add("username", "duplicate")
		form.Add("password", "password1")
		form.Add("password_confirm", "password1")

		w1, _ := makeAuthenticatedRequest(server, http.MethodPost, "/users", adminToken, form)
		resp1 := w1.Result()
		resp1.Body.Close()

		// Try to create duplicate
		form2 := url.Values{}
		form2.Add("username", "duplicate")
		form2.Add("password", "password2")
		form2.Add("password_confirm", "password2")

		w2, _ := makeAuthenticatedRequest(server, http.MethodPost, "/users", adminToken, form2)
		resp2 := w2.Result()
		defer resp2.Body.Close()

		if resp2.StatusCode != http.StatusOK {
			t.Errorf("expected status 200 (form redisplay), got %d", resp2.StatusCode)
		}

		body := w2.Body.String()
		if !strings.Contains(body, "already exists") && !strings.Contains(body, "duplicate") && !strings.Contains(body, "error") {
			t.Error("response should contain error message about duplicate")
		}

		// Cleanup
		user, _ := server.db.Users.GetByUsername(context.Background(), "duplicate")
		if user != nil {
			server.db.Users.Delete(context.Background(), user.ID)
		}
	})

	t.Run("rejects mismatched passwords", func(t *testing.T) {
		adminID, adminToken := createAdminUser(t, server)
		defer server.db.Users.Delete(context.Background(), mustParseInt64(adminID))

		form := url.Values{}
		form.Add("username", "mismatchuser")
		form.Add("password", "password1")
		form.Add("password_confirm", "password2")

		w, _ := makeAuthenticatedRequest(server, http.MethodPost, "/users", adminToken, form)

		resp := w.Result()
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("expected status 200 (form redisplay), got %d", resp.StatusCode)
		}

		body := w.Body.String()
		lowerBody := strings.ToLower(body)
		if !strings.Contains(lowerBody, "password") && !strings.Contains(lowerBody, "match") && !strings.Contains(lowerBody, "error") {
			t.Error("response should contain error about password mismatch")
		}
	})

	t.Run("rejects empty username", func(t *testing.T) {
		adminID, adminToken := createAdminUser(t, server)
		defer server.db.Users.Delete(context.Background(), mustParseInt64(adminID))

		form := url.Values{}
		form.Add("username", "")
		form.Add("password", "password")
		form.Add("password_confirm", "password")

		w, _ := makeAuthenticatedRequest(server, http.MethodPost, "/users", adminToken, form)

		resp := w.Result()
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("expected status 200 (form redisplay), got %d", resp.StatusCode)
		}
	})

	t.Run("rejects empty password", func(t *testing.T) {
		adminID, adminToken := createAdminUser(t, server)
		defer server.db.Users.Delete(context.Background(), mustParseInt64(adminID))

		form := url.Values{}
		form.Add("username", "emptypassuser")
		form.Add("password", "")
		form.Add("password_confirm", "")

		w, _ := makeAuthenticatedRequest(server, http.MethodPost, "/users", adminToken, form)

		resp := w.Result()
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("expected status 200 (form redisplay), got %d", resp.StatusCode)
		}
	})
}

func TestHandleViewUser(t *testing.T) {
	server := setupTestServer(t)

	t.Run("displays user details as admin", func(t *testing.T) {
		adminID, adminToken := createAdminUser(t, server)
		defer server.db.Users.Delete(context.Background(), mustParseInt64(adminID))

		userID, _ := createRegularUser(t, server, "viewuser")
		defer server.db.Users.Delete(context.Background(), mustParseInt64(userID))

		w, _ := makeAuthenticatedRequest(server, http.MethodGet, fmt.Sprintf("/users/%s", userID), adminToken, nil)

		resp := w.Result()
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("expected status 200, got %d", resp.StatusCode)
		}

		body := w.Body.String()
		if !strings.Contains(body, "User Details") || !strings.Contains(body, "viewuser") {
			t.Error("response should contain user details")
		}
		if !strings.Contains(body, "viewuser") {
			t.Error("response should contain username")
		}
	})

	t.Run("denies access to regular user", func(t *testing.T) {
		userID, userToken := createRegularUser(t, server, "regularuser4")
		defer server.db.Users.Delete(context.Background(), mustParseInt64(userID))

		w, _ := makeAuthenticatedRequest(server, http.MethodGet, fmt.Sprintf("/users/%s", userID), userToken, nil)

		resp := w.Result()
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusForbidden {
			t.Errorf("expected status 403, got %d", resp.StatusCode)
		}
	})

	t.Run("returns 404 for non-existent user", func(t *testing.T) {
		adminID, adminToken := createAdminUser(t, server)
		defer server.db.Users.Delete(context.Background(), mustParseInt64(adminID))

		w, _ := makeAuthenticatedRequest(server, http.MethodGet, "/users/999999", adminToken, nil)

		resp := w.Result()
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("expected status 404, got %d", resp.StatusCode)
		}
	})

	t.Run("returns 400 for invalid user ID", func(t *testing.T) {
		adminID, adminToken := createAdminUser(t, server)
		defer server.db.Users.Delete(context.Background(), mustParseInt64(adminID))

		w, _ := makeAuthenticatedRequest(server, http.MethodGet, "/users/invalid", adminToken, nil)

		resp := w.Result()
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("expected status 400, got %d", resp.StatusCode)
		}
	})
}

func TestHandleDeleteUser(t *testing.T) {
	server := setupTestServer(t)

	t.Run("deletes user as admin", func(t *testing.T) {
		adminID, adminToken := createAdminUser(t, server)
		defer server.db.Users.Delete(context.Background(), mustParseInt64(adminID))

		userID, _ := createRegularUser(t, server, "deleteuser")

		w, _ := makeAuthenticatedRequest(server, http.MethodDelete, fmt.Sprintf("/users/%s", userID), adminToken, nil)

		resp := w.Result()
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusSeeOther {
			t.Errorf("expected status 303, got %d", resp.StatusCode)
		}

		location := resp.Header.Get("Location")
		if location != "/users" {
			t.Errorf("expected redirect to /users, got %s", location)
		}

		// Verify user was deleted
		_, err := server.db.Users.GetByID(context.Background(), mustParseInt64(userID))
		if err == nil {
			t.Error("user should have been deleted")
		}
	})

	t.Run("denies access to regular user", func(t *testing.T) {
		userID, userToken := createRegularUser(t, server, "regularuser5")
		defer server.db.Users.Delete(context.Background(), mustParseInt64(userID))

		targetID, _ := createRegularUser(t, server, "targetuser")
		defer server.db.Users.Delete(context.Background(), mustParseInt64(targetID))

		w, _ := makeAuthenticatedRequest(server, http.MethodDelete, fmt.Sprintf("/users/%s", targetID), userToken, nil)

		resp := w.Result()
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusForbidden {
			t.Errorf("expected status 403, got %d", resp.StatusCode)
		}
	})

	t.Run("returns 404 for non-existent user", func(t *testing.T) {
		adminID, adminToken := createAdminUser(t, server)
		defer server.db.Users.Delete(context.Background(), mustParseInt64(adminID))

		w, _ := makeAuthenticatedRequest(server, http.MethodDelete, "/users/999999", adminToken, nil)

		resp := w.Result()
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("expected status 404, got %d", resp.StatusCode)
		}
	})
}

// Helper function to parse int64
func mustParseInt64(s string) int64 {
	var id int64
	fmt.Sscanf(s, "%d", &id)
	return id
}
