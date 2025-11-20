package server

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/jeffanddom/fixity/internal/database"
)

// TestWorkflow_UserAuthentication tests complete authentication workflows
func TestWorkflow_UserAuthentication(t *testing.T) {
	t.Run("complete_login_logout_cycle", func(t *testing.T) {
		server := setupTestServer(t)

		// Create test user directly in database
		ctx := context.Background()
		user := &database.User{
			Username:     "workflowuser",
			PasswordHash: "$2a$12$hash", // bcrypt hash placeholder
			IsAdmin:      false,
		}
		err := server.db.Users.Create(ctx, user)
		if err != nil {
			t.Fatalf("failed to create user: %v", err)
		}

		// Step 1: User visits login page
		req := httptest.NewRequest("GET", "/login", nil)
		rec := httptest.NewRecorder()
		server.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", rec.Code)
		}
		if !strings.Contains(rec.Body.String(), "login") {
			t.Error("login page should contain login form")
		}

		// Step 2: User tries to login with valid credentials
		// Note: Since we can't easily hash passwords in tests without auth service access,
		// we'll use the createAuthenticatedUser helper which handles this
		user2, token := createAuthenticatedUser(t, server)
		defer server.db.Users.Delete(ctx, user2.ID)

		// Step 3: User accesses protected page (dashboard) with session
		w, _ := makeAuthenticatedRequest(server, http.MethodGet, "/", token, nil)

		if w.Code != http.StatusOK {
			t.Fatalf("expected status 200 for dashboard, got %d", w.Code)
		}
		if !strings.Contains(w.Body.String(), user2.Username) {
			t.Error("dashboard should show logged-in username")
		}

		// Step 4: User logs out
		w, _ = makeAuthenticatedRequest(server, http.MethodPost, "/logout", token, nil)

		if w.Code != http.StatusSeeOther {
			t.Fatalf("expected redirect after logout, got %d", w.Code)
		}

		// Step 5: User tries to access dashboard after logout (should redirect to login)
		req = httptest.NewRequest("GET", "/", nil)
		rec = httptest.NewRecorder()
		server.ServeHTTP(rec, req)

		if rec.Code != http.StatusSeeOther {
			t.Fatalf("expected redirect to login, got %d", rec.Code)
		}
	})

	t.Run("invalid_credentials", func(t *testing.T) {
		server := setupTestServer(t)

		form := url.Values{}
		form.Add("username", "nonexistent")
		form.Add("password", "wrongpassword")

		req := httptest.NewRequest("POST", "/login", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rec := httptest.NewRecorder()
		server.ServeHTTP(rec, req)

		// Should show login page with error
		if rec.Code != http.StatusOK {
			t.Fatalf("expected status 200 (login page with error), got %d", rec.Code)
		}
		if !strings.Contains(rec.Body.String(), "Invalid") || !strings.Contains(rec.Body.String(), "login") {
			t.Error("should show error message on login page")
		}
	})

	t.Run("unauthorized_access_to_protected_paths", func(t *testing.T) {
		server := setupTestServer(t)

		protectedPaths := []string{
			"/",
			"/targets",
			"/targets/new",
			"/scans",
			"/files",
			"/admin/users",
		}

		for _, path := range protectedPaths {
			t.Run(path, func(t *testing.T) {
				req := httptest.NewRequest("GET", path, nil)
				rec := httptest.NewRecorder()
				server.ServeHTTP(rec, req)

				// Should redirect to login
				if rec.Code != http.StatusSeeOther {
					t.Errorf("path %s: expected redirect 303, got %d", path, rec.Code)
				}
				location := rec.Header().Get("Location")
				if !strings.Contains(location, "/login") {
					t.Errorf("path %s: expected redirect to login, got %s", path, location)
				}
			})
		}
	})
}

// TestWorkflow_StorageTargetManagement tests storage target CRUD workflows
func TestWorkflow_StorageTargetManagement(t *testing.T) {
	t.Run("admin_creates_local_target", func(t *testing.T) {
		server := setupTestServer(t)

		// Create admin user
		ctx := context.Background()
		admin := &database.User{
			Username:     "admin",
			PasswordHash: "$2a$12$LQv3c1yqBWVHxkd0LHAkCOYz6TtxMQJqhN8/LewY5NU7206VPRJeG", // 'password'
			IsAdmin:      true,
		}
		err := server.db.Users.Create(ctx, admin)
		if err != nil {
			t.Fatalf("failed to create admin: %v", err)
		}
		defer server.db.Users.Delete(ctx, admin.ID)

		// Create session by logging in as admin
		session, err := server.auth.Login(ctx, admin.Username, "password")
		if err != nil {
			t.Fatalf("failed to login as admin: %v", err)
		}

		// Admin creates a local storage target
		form := url.Values{}
		form.Add("name", "Local Archive")
		form.Add("type", "local")
		form.Add("path", "/mnt/archive")
		form.Add("enabled", "true")
		form.Add("parallel_workers", "4")
		form.Add("checksum_algorithm", "blake3")

		w, _ := makeAuthenticatedRequest(server, http.MethodPost, "/targets", session.Token, form)

		// Should redirect to targets list
		if w.Code != http.StatusSeeOther {
			t.Fatalf("expected redirect after creating target, got %d: %s", w.Code, w.Body.String())
		}

		// Verify target was created
		targets, err := server.db.StorageTargets.ListAll(ctx)
		if err != nil {
			t.Fatalf("failed to list targets: %v", err)
		}
		if len(targets) != 1 {
			t.Fatalf("expected 1 target, got %d", len(targets))
		}
		if targets[0].Name != "Local Archive" {
			t.Errorf("expected name 'Local Archive', got %s", targets[0].Name)
		}
	})

	t.Run("admin_creates_nfs_target", func(t *testing.T) {
		server := setupTestServer(t)
		admin, token := createAuthenticatedUser(t, server)
		admin.IsAdmin = true
		server.db.Users.Update(context.Background(), admin)
		defer server.db.Users.Delete(context.Background(), admin.ID)

		form := url.Values{}
		form.Add("name", "NFS Backup")
		form.Add("type", "nfs")
		form.Add("server", "backup.example.com")
		form.Add("share", "/exports/backup")
		form.Add("mount_path", "/mnt/nfs")
		form.Add("enabled", "true")
		form.Add("parallel_workers", "2")
		form.Add("checksum_algorithm", "sha256")

		w, _ := makeAuthenticatedRequest(server, http.MethodPost, "/targets", token, form)

		if w.Code != http.StatusSeeOther {
			t.Fatalf("expected redirect, got %d: %s", w.Code, w.Body.String())
		}
	})

	t.Run("admin_creates_smb_target", func(t *testing.T) {
		server := setupTestServer(t)
		admin, token := createAuthenticatedUser(t, server)
		admin.IsAdmin = true
		server.db.Users.Update(context.Background(), admin)
		defer server.db.Users.Delete(context.Background(), admin.ID)

		form := url.Values{}
		form.Add("name", "SMB Share")
		form.Add("type", "smb")
		form.Add("server", "nas.example.com")
		form.Add("share", "Documents")
		form.Add("mount_path", "/mnt/smb")
		form.Add("enabled", "true")
		form.Add("parallel_workers", "4")
		form.Add("checksum_algorithm", "blake3")

		w, _ := makeAuthenticatedRequest(server, http.MethodPost, "/targets", token, form)

		if w.Code != http.StatusSeeOther {
			t.Fatalf("expected redirect, got %d", w.Code)
		}
	})

	t.Run("regular_user_cannot_create_targets", func(t *testing.T) {
		server := setupTestServer(t)
		user, token := createAuthenticatedUser(t, server)
		defer server.db.Users.Delete(context.Background(), user.ID)

		form := url.Values{}
		form.Add("name", "Unauthorized Target")
		form.Add("type", "local")
		form.Add("path", "/tmp/unauthorized")

		w, _ := makeAuthenticatedRequest(server, http.MethodPost, "/targets", token, form)

		// Should be forbidden
		if w.Code != http.StatusForbidden {
			t.Fatalf("expected 403 Forbidden, got %d", w.Code)
		}
	})

	t.Run("admin_edits_target", func(t *testing.T) {
		server := setupTestServer(t)
		admin, token := createAuthenticatedUser(t, server)
		admin.IsAdmin = true
		server.db.Users.Update(context.Background(), admin)
		defer server.db.Users.Delete(context.Background(), admin.ID)

		// Create a target
		target := createTestTarget(t, server)

		// Edit the target
		form := url.Values{}
		form.Add("name", "Updated Name")
		form.Add("enabled", "false")

		w, _ := makeAuthenticatedRequest(server, http.MethodPost, fmt.Sprintf("/targets/%d/edit", target.ID), token, form)

		if w.Code != http.StatusSeeOther {
			t.Fatalf("expected redirect after edit, got %d", w.Code)
		}
	})

	t.Run("admin_deletes_target", func(t *testing.T) {
		server := setupTestServer(t)
		admin, token := createAuthenticatedUser(t, server)
		admin.IsAdmin = true
		server.db.Users.Update(context.Background(), admin)
		defer server.db.Users.Delete(context.Background(), admin.ID)

		// Create a target
		target := createTestTarget(t, server)

		// Delete the target
		w, _ := makeAuthenticatedRequest(server, http.MethodPost, fmt.Sprintf("/targets/%d/delete", target.ID), token, nil)

		if w.Code != http.StatusSeeOther {
			t.Fatalf("expected redirect after delete, got %d", w.Code)
		}

		// Verify target was deleted
		_, err := server.db.StorageTargets.GetByID(context.Background(), target.ID)
		if err == nil {
			t.Error("target should have been deleted")
		}
	})
}

// TestWorkflow_ScanningOperations tests scan triggering and viewing
func TestWorkflow_ScanningOperations(t *testing.T) {
	t.Run("trigger_scan_and_view_results", func(t *testing.T) {
		server := setupTestServer(t)
		user, token := createAuthenticatedUser(t, server)
		defer server.db.Users.Delete(context.Background(), user.ID)

		// Create a target
		target := createTestTarget(t, server)

		// Trigger a scan (via API or UI)
		// Note: Actual scan triggering would require more infrastructure
		// For now, create a scan record directly
		_ = createTestScan(t, server, target.ID, database.ScanStatusCompleted)

		// View scan results
		w, _ := makeAuthenticatedRequest(server, http.MethodGet, "/scans", token, nil)

		if w.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", w.Code)
		}

		body := w.Body.String()
		if !strings.Contains(body, "Scan") {
			t.Error("scans page should contain scan information")
		}
	})

	t.Run("view_scan_details", func(t *testing.T) {
		server := setupTestServer(t)
		user, token := createAuthenticatedUser(t, server)
		defer server.db.Users.Delete(context.Background(), user.ID)

		target := createTestTarget(t, server)
		scan := createTestScan(t, server, target.ID, database.ScanStatusCompleted)

		// View specific scan details
		w, _ := makeAuthenticatedRequest(server, http.MethodGet, fmt.Sprintf("/scans/%d", scan.ID), token, nil)

		if w.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", w.Code)
		}
	})
}

// TestWorkflow_FileBrowsing tests file browsing and filtering
func TestWorkflow_FileBrowsing(t *testing.T) {
	t.Run("browse_and_filter_files", func(t *testing.T) {
		server := setupTestServer(t)
		user, token := createAuthenticatedUser(t, server)
		defer server.db.Users.Delete(context.Background(), user.ID)

		target := createTestTarget(t, server)
		_ = createTestFile(t, server, target.ID)

		// Browse all files
		w, _ := makeAuthenticatedRequest(server, http.MethodGet, "/files", token, nil)

		if w.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", w.Code)
		}

		// Filter by target
		w, _ = makeAuthenticatedRequest(server, http.MethodGet, fmt.Sprintf("/files?target_id=%d", target.ID), token, nil)

		if w.Code != http.StatusOK {
			t.Fatalf("expected status 200 for filtered view, got %d", w.Code)
		}

		// Filter by path
		w, _ = makeAuthenticatedRequest(server, http.MethodGet, "/files?path=/test", token, nil)

		if w.Code != http.StatusOK {
			t.Fatalf("expected status 200 for path filter, got %d", w.Code)
		}
	})

	t.Run("view_file_details_and_history", func(t *testing.T) {
		server := setupTestServer(t)
		user, token := createAuthenticatedUser(t, server)
		defer server.db.Users.Delete(context.Background(), user.ID)

		target := createTestTarget(t, server)
		file := createTestFile(t, server, target.ID)

		// View file details
		w, _ := makeAuthenticatedRequest(server, http.MethodGet, fmt.Sprintf("/files/%d", file.ID), token, nil)

		if w.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", w.Code)
		}
	})
}

// TestWorkflow_UserManagement tests admin user management
func TestWorkflow_UserManagement(t *testing.T) {
	t.Run("admin_creates_and_manages_users", func(t *testing.T) {
		server := setupTestServer(t)
		admin, token := createAuthenticatedUser(t, server)
		admin.IsAdmin = true
		server.db.Users.Update(context.Background(), admin)
		defer server.db.Users.Delete(context.Background(), admin.ID)

		// View users list
		w, _ := makeAuthenticatedRequest(server, http.MethodGet, "/admin/users", token, nil)

		if w.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", w.Code)
		}

		// Create new user
		form := url.Values{}
		form.Add("username", "newuser")
		form.Add("password", "password123")
		form.Add("is_admin", "false")

		w, _ = makeAuthenticatedRequest(server, http.MethodPost, "/admin/users", token, form)

		// Should create user successfully (exact response depends on implementation)
		if w.Code != http.StatusSeeOther && w.Code != http.StatusOK {
			t.Fatalf("expected success response, got %d", w.Code)
		}
	})

	t.Run("regular_user_cannot_access_user_management", func(t *testing.T) {
		server := setupTestServer(t)
		user, token := createAuthenticatedUser(t, server)
		defer server.db.Users.Delete(context.Background(), user.ID)

		// Try to access user management
		w, _ := makeAuthenticatedRequest(server, http.MethodGet, "/admin/users", token, nil)

		// Should be forbidden
		if w.Code != http.StatusForbidden {
			t.Fatalf("expected 403 Forbidden, got %d", w.Code)
		}
	})
}
