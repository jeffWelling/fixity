package server

import (
	"context"
	"net/http"

	"github.com/jeffanddom/fixity/internal/database"
)

// contextKey is a custom type for context keys to avoid collisions
type contextKey string

const (
	userContextKey contextKey = "user"
)

// requireAuth middleware ensures the user is authenticated
func (s *Server) requireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Get session cookie
		cookie, err := r.Cookie(s.config.SessionCookieName)
		if err != nil {
			// No session cookie, redirect to login
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}

		// Validate session
		user, err := s.auth.ValidateSession(r.Context(), cookie.Value)
		if err != nil {
			// Invalid session, clear cookie and redirect to login
			http.SetCookie(w, &http.Cookie{
				Name:   s.config.SessionCookieName,
				Value:  "",
				Path:   "/",
				MaxAge: -1,
			})
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}

		// Add user to context
		ctx := context.WithValue(r.Context(), userContextKey, user)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// requireAdmin middleware ensures the user is an admin
func (s *Server) requireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user := s.getCurrentUser(r)
		if user == nil || !user.IsAdmin {
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// getCurrentUser retrieves the current user from the request context
func (s *Server) getCurrentUser(r *http.Request) *database.User {
	user, ok := r.Context().Value(userContextKey).(*database.User)
	if !ok {
		return nil
	}
	return user
}
