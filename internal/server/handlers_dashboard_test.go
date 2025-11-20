package server

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/jeffanddom/fixity/internal/database"
)

func TestHandleDashboard(t *testing.T) {
	server := setupTestServer(t)

	t.Run("displays dashboard with no data", func(t *testing.T) {
		user, token := createAuthenticatedUser(t, server)
		defer server.db.Users.Delete(context.Background(), user.ID)

		w, _ := makeAuthenticatedRequest(server, http.MethodGet, "/", token, nil)

		resp := w.Result()
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("expected status 200, got %d", resp.StatusCode)
		}

		body := w.Body.String()
		if !strings.Contains(body, "Dashboard") || !strings.Contains(body, "Fixity") {
			t.Error("response should contain dashboard title")
		}
	})

	t.Run("displays dashboard with targets", func(t *testing.T) {
		target := &database.StorageTarget{
			Name:                "Dashboard Test Target",
			Type:                database.StorageTypeLocal,
			Path:                "/tmp/dashtest",
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

		w, _ := makeAuthenticatedRequest(server, http.MethodGet, "/", token, nil)

		resp := w.Result()
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("expected status 200, got %d", resp.StatusCode)
		}

		body := w.Body.String()
		lowerBody := strings.ToLower(body)
		if !strings.Contains(lowerBody, "target") {
			t.Error("response should mention targets")
		}
	})

	t.Run("displays recent scans", func(t *testing.T) {
		target := &database.StorageTarget{
			Name:                "Scan Test Target",
			Type:                database.StorageTypeLocal,
			Path:                "/tmp/scantest",
			Enabled:             true,
			ParallelWorkers:     1,
			RandomSamplePercent: 1.0,
			ChecksumAlgorithm:   "md5",
			CheckpointInterval:  1000,
			BatchSize:           1000,
		}
		server.db.StorageTargets.Create(context.Background(), target)
		defer server.db.StorageTargets.Delete(context.Background(), target.ID)

		scan := &database.Scan{
			StorageTargetID: target.ID,
			Status:          database.ScanStatusCompleted,
			FilesScanned:    100,
			FilesAdded:      10,
			FilesModified:   5,
			FilesDeleted:    2,
			FilesVerified:   83,
			ErrorsCount:     0,
			IsLargeChange:   false,
			StartedAt:       time.Now().Add(-1 * time.Hour),
		}
		completedAt := time.Now()
		scan.CompletedAt = &completedAt
		server.db.Scans.Create(context.Background(), scan)

		user, token := createAuthenticatedUser(t, server)
		defer server.db.Users.Delete(context.Background(), user.ID)

		w, _ := makeAuthenticatedRequest(server, http.MethodGet, "/", token, nil)

		resp := w.Result()
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("expected status 200, got %d", resp.StatusCode)
		}

		body := w.Body.String()
		lowerBody := strings.ToLower(body)
		if !strings.Contains(lowerBody, "scan") {
			t.Error("response should mention scans")
		}
	})
}

func TestHandleRunningScans(t *testing.T) {
	server := setupTestServer(t)

	t.Run("displays empty running scans list", func(t *testing.T) {
		user, token := createAuthenticatedUser(t, server)
		defer server.db.Users.Delete(context.Background(), user.ID)

		w, _ := makeAuthenticatedRequest(server, http.MethodGet, "/scans/running", token, nil)

		resp := w.Result()
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("expected status 200, got %d", resp.StatusCode)
		}

		body := w.Body.String()
		if !strings.Contains(body, "Running Scans") {
			t.Error("response should contain 'Running Scans' title")
		}
		lowerBody := strings.ToLower(body)
		if !strings.Contains(lowerBody, "no scans") || !strings.Contains(lowerBody, "running") {
			t.Error("response should indicate no scans running")
		}
	})

	t.Run("displays page with auto-refresh", func(t *testing.T) {
		user, token := createAuthenticatedUser(t, server)
		defer server.db.Users.Delete(context.Background(), user.ID)

		w, _ := makeAuthenticatedRequest(server, http.MethodGet, "/scans/running", token, nil)

		resp := w.Result()
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("expected status 200, got %d", resp.StatusCode)
		}

		body := w.Body.String()
		if !strings.Contains(body, "refresh") {
			t.Error("response should mention auto-refresh")
		}
	})
}
