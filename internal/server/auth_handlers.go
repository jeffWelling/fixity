package server

import (
	"net/http"
	"time"
)

// handleLoginPage displays the login form
func (s *Server) handleLoginPage(w http.ResponseWriter, r *http.Request) {
	// Check if already logged in
	if cookie, err := r.Cookie(s.config.SessionCookieName); err == nil {
		if _, err := s.auth.ValidateSession(r.Context(), cookie.Value); err == nil {
			// Already logged in, redirect to dashboard
			http.Redirect(w, r, "/", http.StatusSeeOther)
			return
		}
	}

	// Render login page
	if s.templates != nil {
		if err := s.templates.ExecuteTemplate(w, "login.html", nil); err != nil {
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
		return
	}

	// Fallback: simple HTML form
	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(`
<!DOCTYPE html>
<html>
<head>
    <title>Fixity - Login</title>
    <style>
        body { font-family: Arial, sans-serif; max-width: 400px; margin: 100px auto; padding: 20px; }
        input { display: block; width: 100%; padding: 10px; margin: 10px 0; }
        button { width: 100%; padding: 10px; background: #007bff; color: white; border: none; cursor: pointer; }
        button:hover { background: #0056b3; }
        .error { color: red; padding: 10px; background: #ffe6e6; margin-bottom: 10px; }
    </style>
</head>
<body>
    <h1>Fixity Login</h1>
    <form method="POST" action="/login">
        <input type="text" name="username" placeholder="Username" required autofocus>
        <input type="password" name="password" placeholder="Password" required>
        <button type="submit">Login</button>
    </form>
</body>
</html>
	`))
}

// handleLogin processes login form submission
func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	username := r.FormValue("username")
	password := r.FormValue("password")

	// Attempt login
	session, err := s.auth.Login(r.Context(), username, password)
	if err != nil {
		// Login failed - show error
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`
<!DOCTYPE html>
<html>
<head>
    <title>Fixity - Login</title>
    <style>
        body { font-family: Arial, sans-serif; max-width: 400px; margin: 100px auto; padding: 20px; }
        input { display: block; width: 100%; padding: 10px; margin: 10px 0; }
        button { width: 100%; padding: 10px; background: #007bff; color: white; border: none; cursor: pointer; }
        button:hover { background: #0056b3; }
        .error { color: #721c24; padding: 10px; background: #f8d7da; margin-bottom: 10px; border: 1px solid #f5c6cb; border-radius: 4px; }
    </style>
</head>
<body>
    <h1>Fixity Login</h1>
    <div class="error">Invalid username or password</div>
    <form method="POST" action="/login">
        <input type="text" name="username" placeholder="Username" required autofocus value="` + username + `">
        <input type="password" name="password" placeholder="Password" required>
        <button type="submit">Login</button>
    </form>
</body>
</html>
		`))
		return
	}

	// Set session cookie
	http.SetCookie(w, &http.Cookie{
		Name:     s.config.SessionCookieName,
		Value:    session.Token,
		Path:     "/",
		Expires:  session.ExpiresAt,
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
		Secure:   false, // Set to true in production with HTTPS
	})

	// Redirect to dashboard
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// handleLogout logs out the current user
func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	// Get session cookie
	cookie, err := r.Cookie(s.config.SessionCookieName)
	if err == nil {
		// Delete session from database
		s.auth.Logout(r.Context(), cookie.Value)
	}

	// Clear session cookie
	http.SetCookie(w, &http.Cookie{
		Name:   s.config.SessionCookieName,
		Value:  "",
		Path:   "/",
		MaxAge: -1,
	})

	// Redirect to login
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

// handleHealth returns server health status
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"status":"ok","timestamp":"` + time.Now().Format(time.RFC3339) + `"}`))
}
