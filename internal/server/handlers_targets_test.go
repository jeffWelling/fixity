package server

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jeffanddom/fixity/internal/database"
)

func createAuthenticatedUser(t *testing.T, server *Server) (*database.User, string) {
	// Create a test user
	user, err := server.auth.CreateUser(context.Background(), "testuser", "password", "", false)
	if err != nil {
		t.Fatalf("failed to create user: %v", err)
	}

	// Login to get session
	session, err := server.auth.Login(context.Background(), "testuser", "password")
	if err != nil {
		t.Fatalf("failed to login: %v", err)
	}

	return user, session.Token
}

func makeAuthenticatedRequest(server *Server, method, path string, sessionToken string, form url.Values) (*httptest.ResponseRecorder, *http.Request) {
	var req *http.Request
	if form != nil {
		req = httptest.NewRequest(method, path, strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	} else {
		req = httptest.NewRequest(method, path, nil)
	}

	req.AddCookie(&http.Cookie{
		Name:  "test_session",
		Value: sessionToken,
	})

	w := httptest.NewRecorder()
	server.ServeHTTP(w, req)

	return w, req
}

func TestHandleListTargets(t *testing.T) {
	server := setupTestServer(t)

	t.Run("displays empty targets list", func(t *testing.T) {
		user, token := createAuthenticatedUser(t, server)
		defer server.db.Users.Delete(context.Background(), user.ID)

		w, _ := makeAuthenticatedRequest(server, http.MethodGet, "/targets", token, nil)

		resp := w.Result()
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("expected status 200, got %d", resp.StatusCode)
		}

		body := w.Body.String()
		if !strings.Contains(body, "Storage Targets") {
			t.Error("response should contain 'Storage Targets' title")
		}
		if !strings.Contains(body, "No storage targets configured") {
			t.Error("response should indicate no targets configured")
		}
	})

	t.Run("displays list of targets", func(t *testing.T) {
		// Create test targets
		target1 := &database.StorageTarget{
			Name:                "Test Target 1",
			Type:                database.StorageTypeLocal,
			Path:                "/tmp/test1",
			Enabled:             true,
			ParallelWorkers:     1,
			RandomSamplePercent: 1.0,
			ChecksumAlgorithm:   "md5",
			CheckpointInterval:  1000,
			BatchSize:           1000,
		}
		server.db.StorageTargets.Create(context.Background(), target1)
		defer server.db.StorageTargets.Delete(context.Background(), target1.ID)

		target2 := &database.StorageTarget{
			Name:                "Test Target 2",
			Type:                database.StorageTypeLocal,
			Path:                "/tmp/test2",
			Enabled:             false,
			ParallelWorkers:     1,
			RandomSamplePercent: 1.0,
			ChecksumAlgorithm:   "md5",
			CheckpointInterval:  1000,
			BatchSize:           1000,
		}
		server.db.StorageTargets.Create(context.Background(), target2)
		defer server.db.StorageTargets.Delete(context.Background(), target2.ID)

		user, token := createAuthenticatedUser(t, server)
		defer server.db.Users.Delete(context.Background(), user.ID)

		w, _ := makeAuthenticatedRequest(server, http.MethodGet, "/targets", token, nil)

		resp := w.Result()
		defer resp.Body.Close()

		body := w.Body.String()
		if !strings.Contains(body, "Test Target 1") {
			t.Error("response should contain target 1 name")
		}
		if !strings.Contains(body, "Test Target 2") {
			t.Error("response should contain target 2 name")
		}
		if !strings.Contains(body, "Enabled") {
			t.Error("response should show enabled status")
		}
		if !strings.Contains(body, "Disabled") {
			t.Error("response should show disabled status")
		}
	})

	t.Run("shows NFS target type", func(t *testing.T) {
		target := &database.StorageTarget{
			Name:                "NFS Target",
			Type:                database.StorageTypeNFS,
			Path:                "/mnt/nfs",
			Enabled:             true,
			ParallelWorkers:     1,
			RandomSamplePercent: 1.0,
			ChecksumAlgorithm:   "md5",
			CheckpointInterval:  1000,
			BatchSize:           1000,
		}
		server.db.StorageTargets.Create(context.Background(), target)
		defer server.db.StorageTargets.Delete(context.Background(), target.ID)

		user, token := createAuthenticatedUser(t, server)
		defer server.db.Users.Delete(context.Background(), user.ID)

		w, _ := makeAuthenticatedRequest(server, http.MethodGet, "/targets", token, nil)

		resp := w.Result()
		defer resp.Body.Close()

		body := w.Body.String()
		lowerBody := strings.ToLower(body)
		if !strings.Contains(lowerBody, "nfs") {
			t.Error("response should show NFS type")
		}
	})

	t.Run("shows SMB target type", func(t *testing.T) {
		target := &database.StorageTarget{
			Name:                "SMB Target",
			Type:                database.StorageTypeSMB,
			Path:                "/mnt/smb",
			Enabled:             true,
			ParallelWorkers:     1,
			RandomSamplePercent: 1.0,
			ChecksumAlgorithm:   "md5",
			CheckpointInterval:  1000,
			BatchSize:           1000,
		}
		server.db.StorageTargets.Create(context.Background(), target)
		defer server.db.StorageTargets.Delete(context.Background(), target.ID)

		user, token := createAuthenticatedUser(t, server)
		defer server.db.Users.Delete(context.Background(), user.ID)

		w, _ := makeAuthenticatedRequest(server, http.MethodGet, "/targets", token, nil)

		resp := w.Result()
		defer resp.Body.Close()

		body := w.Body.String()
		lowerBody := strings.ToLower(body)
		if !strings.Contains(lowerBody, "smb") {
			t.Error("response should show SMB type")
		}
	})
}

func TestHandleNewTargetPage(t *testing.T) {
	server := setupTestServer(t)

	t.Run("displays target creation form", func(t *testing.T) {
		user, token := createAuthenticatedUser(t, server)
		defer server.db.Users.Delete(context.Background(), user.ID)

		w, _ := makeAuthenticatedRequest(server, http.MethodGet, "/targets/new", token, nil)

		resp := w.Result()
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("expected status 200, got %d", resp.StatusCode)
		}

		body := w.Body.String()
		if !strings.Contains(body, "Add New Storage Target") {
			t.Error("response should contain form title")
		}
		if !strings.Contains(body, `name="name"`) {
			t.Error("response should contain name input")
		}
		if !strings.Contains(body, `name="type"`) {
			t.Error("response should contain type select")
		}
		if !strings.Contains(body, `name="path"`) {
			t.Error("response should contain path input")
		}
		if !strings.Contains(body, `name="enabled"`) {
			t.Error("response should contain enabled checkbox")
		}
	})
}

func TestHandleCreateTarget(t *testing.T) {
	server := setupTestServer(t)

	t.Run("creates target successfully", func(t *testing.T) {
		user, token := createAuthenticatedUser(t, server)
		defer server.db.Users.Delete(context.Background(), user.ID)

		form := url.Values{}
		form.Add("name", "New Target")
		form.Add("type", "local")
		form.Add("path", "/tmp/newtarget")
		form.Add("enabled", "true")

		w, _ := makeAuthenticatedRequest(server, http.MethodPost, "/targets", token, form)

		resp := w.Result()
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusSeeOther {
			body := w.Body.String()
			t.Fatalf("expected status 303, got %d. Response body:\n%s", resp.StatusCode, body)
		}

		location := resp.Header.Get("Location")
		if location != "/targets" {
			t.Errorf("expected redirect to /targets, got %s", location)
		}

		// Verify target was created
		targets, _ := server.db.StorageTargets.ListAll(context.Background())
		found := false
		for _, target := range targets {
			if target.Name == "New Target" {
				found = true
				if target.Path != "/tmp/newtarget" {
					t.Errorf("expected path /tmp/newtarget, got %s", target.Path)
				}
				if !target.Enabled {
					t.Error("expected target to be enabled")
				}
				server.db.StorageTargets.Delete(context.Background(), target.ID)
				break
			}
		}
		if !found {
			t.Error("target was not created in database")
		}
	})

	t.Run("creates disabled target", func(t *testing.T) {
		user, token := createAuthenticatedUser(t, server)
		defer server.db.Users.Delete(context.Background(), user.ID)

		form := url.Values{}
		form.Add("name", "Disabled Target")
		form.Add("type", "local")
		form.Add("path", "/tmp/disabled")
		// enabled checkbox not checked (not in form)

		w, _ := makeAuthenticatedRequest(server, http.MethodPost, "/targets", token, form)

		resp := w.Result()
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusSeeOther {
			t.Errorf("expected status 303, got %d", resp.StatusCode)
		}

		// Verify target was created as disabled
		targets, _ := server.db.StorageTargets.ListAll(context.Background())
		for _, target := range targets {
			if target.Name == "Disabled Target" {
				if target.Enabled {
					t.Error("expected target to be disabled")
				}
				server.db.StorageTargets.Delete(context.Background(), target.ID)
				break
			}
		}
	})

	t.Run("rejects invalid storage type", func(t *testing.T) {
		user, token := createAuthenticatedUser(t, server)
		defer server.db.Users.Delete(context.Background(), user.ID)

		form := url.Values{}
		form.Add("name", "Invalid Target")
		form.Add("type", "invalid_type")
		form.Add("path", "/tmp/invalid")
		form.Add("enabled", "true")

		w, _ := makeAuthenticatedRequest(server, http.MethodPost, "/targets", token, form)

		resp := w.Result()
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("expected status 200 (form redisplay), got %d", resp.StatusCode)
		}

		body := w.Body.String()
		if !strings.Contains(body, "Invalid storage type") {
			t.Error("response should contain error message")
		}
	})

	t.Run("creates target with empty name", func(t *testing.T) {
		user, token := createAuthenticatedUser(t, server)
		defer server.db.Users.Delete(context.Background(), user.ID)

		form := url.Values{}
		form.Add("name", "")
		form.Add("type", "local")
		form.Add("path", "/tmp/test")
		form.Add("enabled", "true")

		w, _ := makeAuthenticatedRequest(server, http.MethodPost, "/targets", token, form)

		resp := w.Result()
		defer resp.Body.Close()

		// Should either succeed or fail gracefully
		if resp.StatusCode != http.StatusSeeOther && resp.StatusCode != http.StatusOK {
			t.Errorf("expected status 303 or 200, got %d", resp.StatusCode)
		}
	})

	t.Run("creates target with empty path", func(t *testing.T) {
		user, token := createAuthenticatedUser(t, server)
		defer server.db.Users.Delete(context.Background(), user.ID)

		form := url.Values{}
		form.Add("name", "Empty Path Target")
		form.Add("type", "local")
		form.Add("path", "")
		form.Add("enabled", "true")

		w, _ := makeAuthenticatedRequest(server, http.MethodPost, "/targets", token, form)

		resp := w.Result()
		defer resp.Body.Close()

		// Should either succeed or fail gracefully
		if resp.StatusCode != http.StatusSeeOther && resp.StatusCode != http.StatusOK {
			t.Errorf("expected status 303 or 200, got %d", resp.StatusCode)
		}
	})
}

func TestHandleViewTarget(t *testing.T) {
	server := setupTestServer(t)

	t.Run("displays target details", func(t *testing.T) {
		// Create test target
		target := &database.StorageTarget{
			Name:                "View Test Target",
			Type:                database.StorageTypeLocal,
			Path:                "/tmp/viewtest",
			Enabled:             true,
			ParallelWorkers:     1,
			RandomSamplePercent: 1.0,
			ChecksumAlgorithm:   "md5",
			CheckpointInterval:  1000,
			BatchSize:           1000,
		}
		server.db.StorageTargets.Create(context.Background(), target)
		defer server.db.StorageTargets.Delete(context.Background(), target.ID)

		user, token := createAuthenticatedUser(t, server)
		defer server.db.Users.Delete(context.Background(), user.ID)

		w, _ := makeAuthenticatedRequest(server, http.MethodGet, fmt.Sprintf("/targets/%d", target.ID), token, nil)

		resp := w.Result()
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("expected status 200, got %d", resp.StatusCode)
		}

		body := w.Body.String()
		if !strings.Contains(body, "View Test Target") {
			t.Error("response should contain target name")
		}
		if !strings.Contains(body, "/tmp/viewtest") {
			t.Error("response should contain target path")
		}
		if !strings.Contains(body, "Enabled") {
			t.Error("response should show enabled status")
		}
	})

	t.Run("returns 404 for non-existent target", func(t *testing.T) {
		user, token := createAuthenticatedUser(t, server)
		defer server.db.Users.Delete(context.Background(), user.ID)

		w, _ := makeAuthenticatedRequest(server, http.MethodGet, "/targets/999999", token, nil)

		resp := w.Result()
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("expected status 404, got %d", resp.StatusCode)
		}
	})

	t.Run("returns 400 for invalid target ID", func(t *testing.T) {
		user, token := createAuthenticatedUser(t, server)
		defer server.db.Users.Delete(context.Background(), user.ID)

		w, _ := makeAuthenticatedRequest(server, http.MethodGet, "/targets/invalid", token, nil)

		resp := w.Result()
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("expected status 400, got %d", resp.StatusCode)
		}
	})
}

func TestHandleEditTargetPage(t *testing.T) {
	server := setupTestServer(t)

	t.Run("displays edit form", func(t *testing.T) {
		target := &database.StorageTarget{
			Name:                "Edit Test Target",
			Type:                database.StorageTypeLocal,
			Path:                "/tmp/editest",
			Enabled:             true,
			ParallelWorkers:     1,
			RandomSamplePercent: 1.0,
			ChecksumAlgorithm:   "md5",
			CheckpointInterval:  1000,
			BatchSize:           1000,
		}
		server.db.StorageTargets.Create(context.Background(), target)
		defer server.db.StorageTargets.Delete(context.Background(), target.ID)

		user, token := createAuthenticatedUser(t, server)
		defer server.db.Users.Delete(context.Background(), user.ID)

		w, _ := makeAuthenticatedRequest(server, http.MethodGet, fmt.Sprintf("/targets/%d/edit", target.ID), token, nil)

		resp := w.Result()
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("expected status 200, got %d", resp.StatusCode)
		}

		body := w.Body.String()
		if !strings.Contains(body, "Edit Storage Target") && !strings.Contains(body, "Edit Target") {
			t.Error("response should contain edit form title")
		}
		if !strings.Contains(body, "Edit Test Target") {
			t.Error("response should contain current target name")
		}
		if !strings.Contains(body, `name="name"`) {
			t.Error("response should contain name input")
		}
	})

	t.Run("returns 404 for non-existent target", func(t *testing.T) {
		user, token := createAuthenticatedUser(t, server)
		defer server.db.Users.Delete(context.Background(), user.ID)

		w, _ := makeAuthenticatedRequest(server, http.MethodGet, "/targets/999999/edit", token, nil)

		resp := w.Result()
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("expected status 404, got %d", resp.StatusCode)
		}
	})

	t.Run("returns 400 for invalid target ID", func(t *testing.T) {
		user, token := createAuthenticatedUser(t, server)
		defer server.db.Users.Delete(context.Background(), user.ID)

		w, _ := makeAuthenticatedRequest(server, http.MethodGet, "/targets/invalid/edit", token, nil)

		resp := w.Result()
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("expected status 400, got %d", resp.StatusCode)
		}
	})
}

func TestHandleUpdateTarget(t *testing.T) {
	server := setupTestServer(t)

	t.Run("updates target successfully", func(t *testing.T) {
		// Create test target
		target := &database.StorageTarget{
			Name:                "Original Name",
			Type:                database.StorageTypeLocal,
			Path:                "/tmp/original",
			Enabled:             true,
			ParallelWorkers:     1,
			RandomSamplePercent: 1.0,
			ChecksumAlgorithm:   "md5",
			CheckpointInterval:  1000,
			BatchSize:           1000,
		}
		server.db.StorageTargets.Create(context.Background(), target)
		defer server.db.StorageTargets.Delete(context.Background(), target.ID)

		user, token := createAuthenticatedUser(t, server)
		defer server.db.Users.Delete(context.Background(), user.ID)

		form := url.Values{}
		form.Add("name", "Updated Name")
		form.Add("type", "local")
		form.Add("path", "/tmp/updated")
		form.Add("enabled", "false")

		w, _ := makeAuthenticatedRequest(server, http.MethodPut, fmt.Sprintf("/targets/%d", target.ID), token, form)

		resp := w.Result()
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusSeeOther {
			t.Errorf("expected status 303, got %d", resp.StatusCode)
		}

		// Verify target was updated
		updated, _ := server.db.StorageTargets.GetByID(context.Background(), target.ID)
		if updated.Name != "Updated Name" {
			t.Errorf("expected name 'Updated Name', got %s", updated.Name)
		}
		if updated.Path != "/tmp/updated" {
			t.Errorf("expected path /tmp/updated, got %s", updated.Path)
		}
		if updated.Enabled {
			t.Error("expected target to be disabled")
		}
	})

	t.Run("returns 404 for non-existent target", func(t *testing.T) {
		user, token := createAuthenticatedUser(t, server)
		defer server.db.Users.Delete(context.Background(), user.ID)

		form := url.Values{}
		form.Add("name", "Updated Name")
		form.Add("type", "local")
		form.Add("path", "/tmp/updated")
		form.Add("enabled", "true")

		w, _ := makeAuthenticatedRequest(server, http.MethodPut, "/targets/999999", token, form)

		resp := w.Result()
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("expected status 404, got %d", resp.StatusCode)
		}
	})

	t.Run("returns 400 for invalid target ID", func(t *testing.T) {
		user, token := createAuthenticatedUser(t, server)
		defer server.db.Users.Delete(context.Background(), user.ID)

		form := url.Values{}
		form.Add("name", "Updated Name")
		form.Add("type", "local")
		form.Add("path", "/tmp/updated")

		w, _ := makeAuthenticatedRequest(server, http.MethodPut, "/targets/invalid", token, form)

		resp := w.Result()
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("expected status 400, got %d", resp.StatusCode)
		}
	})

	t.Run("rejects invalid storage type on update", func(t *testing.T) {
		target := &database.StorageTarget{
			Name:                "Update Test",
			Type:                database.StorageTypeLocal,
			Path:                "/tmp/test",
			Enabled:             true,
			ParallelWorkers:     1,
			RandomSamplePercent: 1.0,
			ChecksumAlgorithm:   "md5",
			CheckpointInterval:  1000,
			BatchSize:           1000,
		}
		server.db.StorageTargets.Create(context.Background(), target)
		defer server.db.StorageTargets.Delete(context.Background(), target.ID)

		user, token := createAuthenticatedUser(t, server)
		defer server.db.Users.Delete(context.Background(), user.ID)

		form := url.Values{}
		form.Add("name", "Updated Name")
		form.Add("type", "invalid_type")
		form.Add("path", "/tmp/updated")
		form.Add("enabled", "true")

		w, _ := makeAuthenticatedRequest(server, http.MethodPut, fmt.Sprintf("/targets/%d", target.ID), token, form)

		resp := w.Result()
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("expected status 200 (form redisplay), got %d", resp.StatusCode)
		}

		body := w.Body.String()
		if !strings.Contains(body, "Invalid storage type") {
			t.Error("response should contain error message")
		}
	})

	t.Run("handles method override for delete", func(t *testing.T) {
		target := &database.StorageTarget{
			Name:                "Method Override Test",
			Type:                database.StorageTypeLocal,
			Path:                "/tmp/methodtest",
			Enabled:             true,
			ParallelWorkers:     1,
			RandomSamplePercent: 1.0,
			ChecksumAlgorithm:   "md5",
			CheckpointInterval:  1000,
			BatchSize:           1000,
		}
		server.db.StorageTargets.Create(context.Background(), target)

		user, token := createAuthenticatedUser(t, server)
		defer server.db.Users.Delete(context.Background(), user.ID)

		// Send PUT request with _method=DELETE to trigger delete via method override
		form := url.Values{}
		form.Add("_method", "DELETE")

		w, _ := makeAuthenticatedRequest(server, http.MethodPut, fmt.Sprintf("/targets/%d", target.ID), token, form)

		resp := w.Result()
		defer resp.Body.Close()

		// Should redirect to targets list after deletion
		if resp.StatusCode != http.StatusSeeOther {
			t.Errorf("expected status 303, got %d", resp.StatusCode)
		}

		// Verify target was deleted
		_, err := server.db.StorageTargets.GetByID(context.Background(), target.ID)
		if err == nil {
			t.Error("target should have been deleted via method override")
		}
	})
}

func TestHandleDeleteTarget(t *testing.T) {
	server := setupTestServer(t)

	t.Run("deletes target successfully", func(t *testing.T) {
		// Create test target
		target := &database.StorageTarget{
			Name:                "Delete Me",
			Type:                database.StorageTypeLocal,
			Path:                "/tmp/deleteme",
			Enabled:             false,
			ParallelWorkers:     1,
			RandomSamplePercent: 1.0,
			ChecksumAlgorithm:   "md5",
			CheckpointInterval:  1000,
			BatchSize:           1000,
		}
		server.db.StorageTargets.Create(context.Background(), target)

		user, token := createAuthenticatedUser(t, server)
		defer server.db.Users.Delete(context.Background(), user.ID)

		w, _ := makeAuthenticatedRequest(server, http.MethodDelete, fmt.Sprintf("/targets/%d", target.ID), token, nil)

		resp := w.Result()
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusSeeOther {
			t.Errorf("expected status 303, got %d", resp.StatusCode)
		}

		location := resp.Header.Get("Location")
		if location != "/targets" {
			t.Errorf("expected redirect to /targets, got %s", location)
		}

		// Verify target was deleted
		_, err := server.db.StorageTargets.GetByID(context.Background(), target.ID)
		if err == nil {
			t.Error("target should have been deleted")
		}
	})

	t.Run("returns 404 for non-existent target", func(t *testing.T) {
		user, token := createAuthenticatedUser(t, server)
		defer server.db.Users.Delete(context.Background(), user.ID)

		w, _ := makeAuthenticatedRequest(server, http.MethodDelete, "/targets/999999", token, nil)

		resp := w.Result()
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("expected status 404, got %d", resp.StatusCode)
		}
	})
}

func TestHandleTriggerScan(t *testing.T) {
	server := setupTestServer(t)

	t.Run("triggers scan successfully", func(t *testing.T) {
		// Create test directory
		tmpDir := t.TempDir()
		testFile := filepath.Join(tmpDir, "test.txt")
		os.WriteFile(testFile, []byte("test content"), 0644)

		// Create test target
		target := &database.StorageTarget{
			Name:                "Scan Target",
			Type:                database.StorageTypeLocal,
			Path:                tmpDir,
			Enabled:             true,
			ParallelWorkers:     1,
			RandomSamplePercent: 1.0,
			ChecksumAlgorithm:   "md5",
			CheckpointInterval:  1000,
			BatchSize:           1000,
		}
		server.db.StorageTargets.Create(context.Background(), target)
		defer server.db.StorageTargets.Delete(context.Background(), target.ID)

		user, token := createAuthenticatedUser(t, server)
		defer server.db.Users.Delete(context.Background(), user.ID)

		w, _ := makeAuthenticatedRequest(server, http.MethodPost, fmt.Sprintf("/targets/%d/scan", target.ID), token, nil)

		resp := w.Result()
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusSeeOther {
			t.Errorf("expected status 303, got %d", resp.StatusCode)
		}

		location := resp.Header.Get("Location")
		if location != "/" {
			t.Errorf("expected redirect to /, got %s", location)
		}
	})

	t.Run("returns 404 for non-existent target", func(t *testing.T) {
		user, token := createAuthenticatedUser(t, server)
		defer server.db.Users.Delete(context.Background(), user.ID)

		w, _ := makeAuthenticatedRequest(server, http.MethodPost, "/targets/999999/scan", token, nil)

		resp := w.Result()
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("expected status 404, got %d", resp.StatusCode)
		}
	})

	t.Run("returns 400 for invalid target ID", func(t *testing.T) {
		user, token := createAuthenticatedUser(t, server)
		defer server.db.Users.Delete(context.Background(), user.ID)

		w, _ := makeAuthenticatedRequest(server, http.MethodPost, "/targets/invalid/scan", token, nil)

		resp := w.Result()
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("expected status 400, got %d", resp.StatusCode)
		}
	})
}
