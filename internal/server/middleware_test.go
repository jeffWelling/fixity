package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRequireAuth(t *testing.T) {
	server := setupTestServer(t)

	// Handler that should only be accessible when authenticated
	protectedHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("protected content"))
	})

	t.Run("allows access with valid session", func(t *testing.T) {
		// Create user and session
		user, _ := server.auth.CreateUser(context.Background(), "authuser", "password", "", false)
		defer server.db.Users.Delete(context.Background(), user.ID)

		session, _ := server.auth.Login(context.Background(), "authuser", "password")
		defer server.auth.Logout(context.Background(), session.Token)

		req := httptest.NewRequest(http.MethodGet, "/protected", nil)
		req.AddCookie(&http.Cookie{
			Name:  "test_session",
			Value: session.Token,
		})
		w := httptest.NewRecorder()

		handler := server.requireAuth(protectedHandler)
		handler.ServeHTTP(w, req)

		resp := w.Result()
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("expected status 200, got %d", resp.StatusCode)
		}

		body := w.Body.String()
		if body != "protected content" {
			t.Errorf("expected 'protected content', got %s", body)
		}
	})

	t.Run("redirects to login without session cookie", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/protected", nil)
		w := httptest.NewRecorder()

		handler := server.requireAuth(protectedHandler)
		handler.ServeHTTP(w, req)

		resp := w.Result()
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusSeeOther {
			t.Errorf("expected status 303, got %d", resp.StatusCode)
		}

		location := resp.Header.Get("Location")
		if location != "/login" {
			t.Errorf("expected redirect to /login, got %s", location)
		}
	})

	t.Run("redirects to login with invalid session", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/protected", nil)
		req.AddCookie(&http.Cookie{
			Name:  "test_session",
			Value: "invalid_token_12345",
		})
		w := httptest.NewRecorder()

		handler := server.requireAuth(protectedHandler)
		handler.ServeHTTP(w, req)

		resp := w.Result()
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusSeeOther {
			t.Errorf("expected status 303, got %d", resp.StatusCode)
		}

		// Should also clear the invalid cookie
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
			t.Error("invalid session cookie should be deleted (MaxAge=-1)")
		}
	})

	t.Run("adds user to request context", func(t *testing.T) {
		// Create user and session
		user, _ := server.auth.CreateUser(context.Background(), "contextuser", "password", "", false)
		defer server.db.Users.Delete(context.Background(), user.ID)

		session, _ := server.auth.Login(context.Background(), "contextuser", "password")
		defer server.auth.Logout(context.Background(), session.Token)

		// Handler that checks context
		contextCheckHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctxUser := server.getCurrentUser(r)
			if ctxUser == nil {
				t.Error("user should be in context")
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			if ctxUser.Username != "contextuser" {
				t.Errorf("expected username 'contextuser', got %s", ctxUser.Username)
			}
			w.WriteHeader(http.StatusOK)
		})

		req := httptest.NewRequest(http.MethodGet, "/protected", nil)
		req.AddCookie(&http.Cookie{
			Name:  "test_session",
			Value: session.Token,
		})
		w := httptest.NewRecorder()

		handler := server.requireAuth(contextCheckHandler)
		handler.ServeHTTP(w, req)

		resp := w.Result()
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("expected status 200, got %d", resp.StatusCode)
		}
	})
}

func TestRequireAdmin(t *testing.T) {
	server := setupTestServer(t)

	adminHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("admin content"))
	})

	t.Run("allows access for admin user", func(t *testing.T) {
		// Create admin user
		admin, _ := server.auth.CreateUser(context.Background(), "admin", "password", "", true)
		defer server.db.Users.Delete(context.Background(), admin.ID)

		session, _ := server.auth.Login(context.Background(), "admin", "password")
		defer server.auth.Logout(context.Background(), session.Token)

		req := httptest.NewRequest(http.MethodGet, "/admin", nil)
		req.AddCookie(&http.Cookie{
			Name:  "test_session",
			Value: session.Token,
		})
		w := httptest.NewRecorder()

		// Chain auth middleware first, then admin
		handler := server.requireAuth(server.requireAdmin(adminHandler))
		handler.ServeHTTP(w, req)

		resp := w.Result()
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("expected status 200, got %d", resp.StatusCode)
		}

		body := w.Body.String()
		if body != "admin content" {
			t.Errorf("expected 'admin content', got %s", body)
		}
	})

	t.Run("denies access for regular user", func(t *testing.T) {
		// Create regular user
		user, _ := server.auth.CreateUser(context.Background(), "regular", "password", "", false)
		defer server.db.Users.Delete(context.Background(), user.ID)

		session, _ := server.auth.Login(context.Background(), "regular", "password")
		defer server.auth.Logout(context.Background(), session.Token)

		req := httptest.NewRequest(http.MethodGet, "/admin", nil)
		req.AddCookie(&http.Cookie{
			Name:  "test_session",
			Value: session.Token,
		})
		w := httptest.NewRecorder()

		// Chain auth middleware first, then admin
		handler := server.requireAuth(server.requireAdmin(adminHandler))
		handler.ServeHTTP(w, req)

		resp := w.Result()
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusForbidden {
			t.Errorf("expected status 403, got %d", resp.StatusCode)
		}
	})

	t.Run("returns forbidden without user in context", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/admin", nil)
		w := httptest.NewRecorder()

		// Call requireAdmin directly without auth middleware
		handler := server.requireAdmin(adminHandler)
		handler.ServeHTTP(w, req)

		resp := w.Result()
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusForbidden {
			t.Errorf("expected status 403, got %d", resp.StatusCode)
		}
	})
}

func TestGetCurrentUser(t *testing.T) {
	server := setupTestServer(t)

	t.Run("returns user from context", func(t *testing.T) {
		user, _ := server.auth.CreateUser(context.Background(), "testuser", "password", "", false)
		defer server.db.Users.Delete(context.Background(), user.ID)

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		ctx := context.WithValue(req.Context(), userContextKey, user)
		req = req.WithContext(ctx)

		result := server.getCurrentUser(req)
		if result == nil {
			t.Fatal("expected user to be returned")
		}

		if result.ID != user.ID {
			t.Errorf("expected user ID %d, got %d", user.ID, result.ID)
		}
	})

	t.Run("returns nil when no user in context", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)

		result := server.getCurrentUser(req)
		if result != nil {
			t.Error("expected nil when no user in context")
		}
	})
}
