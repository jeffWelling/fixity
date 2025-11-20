package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/jeffanddom/fixity/internal/auth"
	"github.com/jeffanddom/fixity/internal/coordinator"
	"github.com/jeffanddom/fixity/tests/testutil"
)

func setupTestServer(t *testing.T) *Server {
	db := testutil.NewTestDB(t)
	t.Cleanup(func() {
		testutil.CleanupDB(t, db)
		db.Close()
	})

	authService := auth.NewService(db, auth.Config{})
	coord := coordinator.NewCoordinator(db, coordinator.Config{})

	server, err := New(db, authService, coord, Config{
		SessionCookieName: "test_session",
	})
	if err != nil {
		t.Fatalf("failed to create server: %v", err)
	}

	return server
}

func TestHandleLoginPage(t *testing.T) {
	server := setupTestServer(t)

	t.Run("displays login form", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/login", nil)
		w := httptest.NewRecorder()

		server.handleLoginPage(w, req)

		resp := w.Result()
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("expected status 200, got %d", resp.StatusCode)
		}

		body := w.Body.String()
		if !strings.Contains(body, "Fixity Login") {
			t.Error("response should contain 'Fixity Login'")
		}
		if !strings.Contains(body, `type="password"`) {
			t.Error("response should contain password input")
		}
		if !strings.Contains(body, `action="/login"`) {
			t.Error("response should contain login form action")
		}
	})

	t.Run("redirects to dashboard if already logged in", func(t *testing.T) {
		// Create a user and session
		user, _ := server.auth.CreateUser(context.Background(), "testuser", "password", "", false)
		session, _ := server.auth.Login(context.Background(), "testuser", "password")

		req := httptest.NewRequest(http.MethodGet, "/login", nil)
		req.AddCookie(&http.Cookie{
			Name:  "test_session",
			Value: session.Token,
		})
		w := httptest.NewRecorder()

		server.handleLoginPage(w, req)

		resp := w.Result()
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusSeeOther {
			t.Errorf("expected status 303, got %d", resp.StatusCode)
		}

		location := resp.Header.Get("Location")
		if location != "/" {
			t.Errorf("expected redirect to /, got %s", location)
		}

		// Cleanup
		server.auth.Logout(context.Background(), session.Token)
		server.db.Users.Delete(context.Background(), user.ID)
	})
}

func TestHandleLogin(t *testing.T) {
	server := setupTestServer(t)

	t.Run("successful login with valid credentials", func(t *testing.T) {
		// Create a test user
		user, err := server.auth.CreateUser(context.Background(), "testuser", "password123", "", false)
		if err != nil {
			t.Fatalf("failed to create user: %v", err)
		}
		defer server.db.Users.Delete(context.Background(), user.ID)

		// Submit login form
		form := url.Values{}
		form.Add("username", "testuser")
		form.Add("password", "password123")

		req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		w := httptest.NewRecorder()

		server.handleLogin(w, req)

		resp := w.Result()
		defer resp.Body.Close()

		// Should redirect to dashboard
		if resp.StatusCode != http.StatusSeeOther {
			t.Errorf("expected status 303, got %d", resp.StatusCode)
		}

		location := resp.Header.Get("Location")
		if location != "/" {
			t.Errorf("expected redirect to /, got %s", location)
		}

		// Should set session cookie
		cookies := resp.Cookies()
		var sessionCookie *http.Cookie
		for _, c := range cookies {
			if c.Name == "test_session" {
				sessionCookie = c
				break
			}
		}

		if sessionCookie == nil {
			t.Fatal("expected session cookie to be set")
		}

		if sessionCookie.Value == "" {
			t.Error("session cookie should have a value")
		}

		if !sessionCookie.HttpOnly {
			t.Error("session cookie should be HttpOnly")
		}

		// Cleanup session
		server.auth.Logout(context.Background(), sessionCookie.Value)
	})

	t.Run("login fails with incorrect password", func(t *testing.T) {
		// Create a test user
		user, err := server.auth.CreateUser(context.Background(), "testuser2", "correctpass", "", false)
		if err != nil {
			t.Fatalf("failed to create user: %v", err)
		}
		defer server.db.Users.Delete(context.Background(), user.ID)

		// Submit login form with wrong password
		form := url.Values{}
		form.Add("username", "testuser2")
		form.Add("password", "wrongpassword")

		req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		w := httptest.NewRecorder()

		server.handleLogin(w, req)

		resp := w.Result()
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("expected status 401, got %d", resp.StatusCode)
		}

		body := w.Body.String()
		if !strings.Contains(body, "Invalid username or password") {
			t.Error("response should contain error message")
		}
	})

	t.Run("login fails with non-existent user", func(t *testing.T) {
		form := url.Values{}
		form.Add("username", "nonexistent")
		form.Add("password", "password")

		req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		w := httptest.NewRecorder()

		server.handleLogin(w, req)

		resp := w.Result()
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("expected status 401, got %d", resp.StatusCode)
		}
	})

	t.Run("login fails with malformed form data", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader("invalid%%form"))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		w := httptest.NewRecorder()

		server.handleLogin(w, req)

		resp := w.Result()
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("expected status 400, got %d", resp.StatusCode)
		}
	})

	t.Run("login fails with empty username", func(t *testing.T) {
		form := url.Values{}
		form.Add("username", "")
		form.Add("password", "password")

		req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		w := httptest.NewRecorder()

		server.handleLogin(w, req)

		resp := w.Result()
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("expected status 401, got %d", resp.StatusCode)
		}
	})

	t.Run("login fails with empty password", func(t *testing.T) {
		user, _ := server.auth.CreateUser(context.Background(), "emptypasstest", "password123", "", false)
		defer server.db.Users.Delete(context.Background(), user.ID)

		form := url.Values{}
		form.Add("username", "emptypasstest")
		form.Add("password", "")

		req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		w := httptest.NewRecorder()

		server.handleLogin(w, req)

		resp := w.Result()
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("expected status 401, got %d", resp.StatusCode)
		}
	})

	t.Run("login with case-sensitive username", func(t *testing.T) {
		user, _ := server.auth.CreateUser(context.Background(), "CaseSensitive", "password123", "", false)
		defer server.db.Users.Delete(context.Background(), user.ID)

		// Try login with different case
		form := url.Values{}
		form.Add("username", "casesensitive")
		form.Add("password", "password123")

		req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		w := httptest.NewRecorder()

		server.handleLogin(w, req)

		resp := w.Result()
		defer resp.Body.Close()

		// Should fail since username is case-sensitive
		if resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("expected status 401 for case mismatch, got %d", resp.StatusCode)
		}
	})
}

func TestHandleLogout(t *testing.T) {
	server := setupTestServer(t)

	t.Run("successful logout clears session", func(t *testing.T) {
		// Create user and login
		user, _ := server.auth.CreateUser(context.Background(), "logoutuser", "password", "", false)
		defer server.db.Users.Delete(context.Background(), user.ID)

		session, _ := server.auth.Login(context.Background(), "logoutuser", "password")

		// Logout
		req := httptest.NewRequest(http.MethodPost, "/logout", nil)
		req.AddCookie(&http.Cookie{
			Name:  "test_session",
			Value: session.Token,
		})
		w := httptest.NewRecorder()

		server.handleLogout(w, req)

		resp := w.Result()
		defer resp.Body.Close()

		// Should redirect to login
		if resp.StatusCode != http.StatusSeeOther {
			t.Errorf("expected status 303, got %d", resp.StatusCode)
		}

		location := resp.Header.Get("Location")
		if location != "/login" {
			t.Errorf("expected redirect to /login, got %s", location)
		}

		// Should clear session cookie
		cookies := resp.Cookies()
		var sessionCookie *http.Cookie
		for _, c := range cookies {
			if c.Name == "test_session" {
				sessionCookie = c
				break
			}
		}

		if sessionCookie == nil {
			t.Fatal("expected session cookie to be cleared")
		}

		if sessionCookie.MaxAge != -1 {
			t.Error("session cookie MaxAge should be -1 to delete it")
		}

		// Session should be invalid
		_, err := server.auth.ValidateSession(context.Background(), session.Token)
		if err == nil {
			t.Error("session should be invalidated")
		}
	})

	t.Run("logout works even without session cookie", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/logout", nil)
		w := httptest.NewRecorder()

		server.handleLogout(w, req)

		resp := w.Result()
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusSeeOther {
			t.Errorf("expected status 303, got %d", resp.StatusCode)
		}
	})
}

func TestHandleHealth(t *testing.T) {
	server := setupTestServer(t)

	t.Run("returns ok status", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/health", nil)
		w := httptest.NewRecorder()

		server.handleHealth(w, req)

		resp := w.Result()
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("expected status 200, got %d", resp.StatusCode)
		}

		contentType := resp.Header.Get("Content-Type")
		if contentType != "application/json" {
			t.Errorf("expected Content-Type application/json, got %s", contentType)
		}

		body := w.Body.String()
		if !strings.Contains(body, `"status":"ok"`) {
			t.Error("response should contain status:ok")
		}
		if !strings.Contains(body, `"timestamp"`) {
			t.Error("response should contain timestamp")
		}
	})
}
