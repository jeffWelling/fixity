package server

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/jeffanddom/fixity/internal/coordinator"
	"github.com/jeffanddom/fixity/internal/database"
)

// handleDashboard shows the main dashboard
func (s *Server) handleDashboard(w http.ResponseWriter, r *http.Request) {
	user := s.getCurrentUser(r)

	// Get running scans
	runningScans, _ := s.coordinator.GetRunningSans(r.Context())

	// Get recent scans
	recentScans, _ := s.db.Scans.List(r.Context(), database.ScanFilters{
		Limit: 10,
	})

	// Get all storage targets
	targets, _ := s.db.StorageTargets.ListAll(r.Context())

	// Count statistics
	enabledTargets := 0
	for _, t := range targets {
		if t.Enabled {
			enabledTargets++
		}
	}

	data := map[string]interface{}{
		"User":           user,
		"RunningScans":   runningScans,
		"RecentScans":    recentScans,
		"Targets":        targets,
		"EnabledTargets": enabledTargets,
		"TotalTargets":   len(targets),
	}

	// Render template or simple HTML
	if s.templates != nil {
		if err := s.templates.ExecuteTemplate(w, "dashboard.html", data); err != nil {
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
		return
	}

	// Fallback: simple HTML
	s.renderSimpleDashboard(w, data)
}

// renderSimpleDashboard renders a basic HTML dashboard
func (s *Server) renderSimpleDashboard(w http.ResponseWriter, data map[string]interface{}) {
	w.Header().Set("Content-Type", "text/html")

	user := data["User"].(*database.User)
	runningScans := data["RunningScans"]
	recentScans := data["RecentScans"].([]*database.Scan)
	targets := data["Targets"].([]*database.StorageTarget)

	html := `
<!DOCTYPE html>
<html>
<head>
    <title>Fixity - Dashboard</title>
    <style>
        body { font-family: Arial, sans-serif; margin: 0; padding: 0; }
        .header { background: #2c3e50; color: white; padding: 1rem 2rem; display: flex; justify-content: space-between; align-items: center; }
        .nav { display: flex; gap: 1rem; }
        .nav a { color: white; text-decoration: none; }
        .nav a:hover { text-decoration: underline; }
        .container { padding: 2rem; max-width: 1200px; margin: 0 auto; }
        .stats { display: grid; grid-template-columns: repeat(3, 1fr); gap: 1rem; margin-bottom: 2rem; }
        .stat-card { background: #f8f9fa; padding: 1.5rem; border-radius: 8px; border-left: 4px solid #007bff; }
        .stat-value { font-size: 2rem; font-weight: bold; color: #007bff; }
        .stat-label { color: #6c757d; margin-top: 0.5rem; }
        .section { margin: 2rem 0; }
        .section h2 { border-bottom: 2px solid #007bff; padding-bottom: 0.5rem; }
        table { width: 100%; border-collapse: collapse; background: white; }
        th, td { padding: 0.75rem; text-align: left; border-bottom: 1px solid #dee2e6; }
        th { background: #f8f9fa; font-weight: 600; }
        tr:hover { background: #f8f9fa; }
        .btn { padding: 0.5rem 1rem; background: #007bff; color: white; border: none; border-radius: 4px; text-decoration: none; display: inline-block; cursor: pointer; }
        .btn:hover { background: #0056b3; }
        .btn-sm { padding: 0.25rem 0.5rem; font-size: 0.875rem; }
        .status-running { color: #28a745; }
        .status-completed { color: #6c757d; }
        .status-failed { color: #dc3545; }
        .logout-form { display: inline; }
    </style>
</head>
<body>
    <div class="header">
        <h1>Fixity</h1>
        <div class="nav">
            <a href="/">Dashboard</a>
            <a href="/targets">Storage Targets</a>
            <a href="/scans">Scans</a>
            <a href="/files">Files</a>` + func() string {
		if user.IsAdmin {
			return `<a href="/users">Users</a>`
		}
		return ""
	}() + `
            <span>|</span>
            <span>` + user.Username + `</span>
            <form method="POST" action="/logout" class="logout-form">
                <button type="submit" class="btn btn-sm">Logout</button>
            </form>
        </div>
    </div>
    <div class="container">
        <div class="stats">
            <div class="stat-card">
                <div class="stat-value">` + strconv.Itoa(data["EnabledTargets"].(int)) + `</div>
                <div class="stat-label">Active Targets</div>
            </div>
            <div class="stat-card">
                <div class="stat-value">` + strconv.Itoa(len(runningScans.([]*coordinator.ScanStatus))) + `</div>
                <div class="stat-label">Running Scans</div>
            </div>
            <div class="stat-card">
                <div class="stat-value">` + strconv.Itoa(len(targets)) + `</div>
                <div class="stat-label">Total Targets</div>
            </div>
        </div>

        <div class="section">
            <h2>Recent Scans</h2>`

	if len(recentScans) == 0 {
		html += `<p>No scans yet. <a href="/targets">Configure a storage target</a> to get started.</p>`
	} else {
		html += `
            <table>
                <thead>
                    <tr>
                        <th>Target</th>
                        <th>Status</th>
                        <th>Started</th>
                        <th>Files</th>
                        <th>Changes</th>
                        <th>Actions</th>
                    </tr>
                </thead>
                <tbody>`

		for _, scan := range recentScans {
			// Get target name
			targetName := "Unknown"
			for _, t := range targets {
				if t.ID == scan.StorageTargetID {
					targetName = t.Name
					break
				}
			}

			statusClass := "status-" + string(scan.Status)
			html += fmt.Sprintf(`
                    <tr>
                        <td>%s</td>
                        <td class="%s">%s</td>
                        <td>%s</td>
                        <td>%d</td>
                        <td>+%d / ~%d / -%d</td>
                        <td><a href="/scans/%d" class="btn btn-sm">View</a></td>
                    </tr>`,
				targetName,
				statusClass,
				scan.Status,
				scan.StartedAt.Format("2006-01-02 15:04"),
				scan.FilesScanned,
				scan.FilesAdded,
				scan.FilesModified,
				scan.FilesDeleted,
				scan.ID,
			)
		}

		html += `
                </tbody>
            </table>`
	}

	html += `
        </div>

        <div class="section">
            <h2>Storage Targets</h2>
            <a href="/targets/new" class="btn">Add New Target</a>
            <table style="margin-top: 1rem;">
                <thead>
                    <tr>
                        <th>Name</th>
                        <th>Type</th>
                        <th>Path</th>
                        <th>Status</th>
                        <th>Actions</th>
                    </tr>
                </thead>
                <tbody>`

	if len(targets) == 0 {
		html += `<tr><td colspan="5">No storage targets configured.</td></tr>`
	} else {
		for _, target := range targets {
			status := "Disabled"
			if target.Enabled {
				status = "Enabled"
			}
			html += fmt.Sprintf(`
                    <tr>
                        <td>%s</td>
                        <td>%s</td>
                        <td>%s</td>
                        <td>%s</td>
                        <td>
                            <a href="/targets/%d" class="btn btn-sm">View</a>
                            <form method="POST" action="/targets/%d/scan" style="display:inline;">
                                <button type="submit" class="btn btn-sm">Scan Now</button>
                            </form>
                        </td>
                    </tr>`,
				target.Name,
				target.Type,
				target.Path,
				status,
				target.ID,
				target.ID,
			)
		}
	}

	html += `
                </tbody>
            </table>
        </div>
    </div>
</body>
</html>`

	w.Write([]byte(html))
}

// handleListTargets displays all storage targets
func (s *Server) handleListTargets(w http.ResponseWriter, r *http.Request) {
	user := s.getCurrentUser(r)
	targets, _ := s.db.StorageTargets.ListAll(r.Context())

	data := map[string]interface{}{
		"User":    user,
		"Targets": targets,
	}

	if s.templates != nil {
		if err := s.templates.ExecuteTemplate(w, "targets_list.html", data); err != nil {
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
		return
	}

	s.renderSimpleTargetsList(w, data)
}

func (s *Server) renderSimpleTargetsList(w http.ResponseWriter, data map[string]interface{}) {
	w.Header().Set("Content-Type", "text/html")
	user := data["User"].(*database.User)
	targets := data["Targets"].([]*database.StorageTarget)

	html := `
<!DOCTYPE html>
<html>
<head>
    <title>Fixity - Storage Targets</title>
    <style>
        body { font-family: Arial, sans-serif; margin: 0; padding: 0; }
        .header { background: #2c3e50; color: white; padding: 1rem 2rem; display: flex; justify-content: space-between; align-items: center; }
        .nav { display: flex; gap: 1rem; }
        .nav a { color: white; text-decoration: none; }
        .nav a:hover { text-decoration: underline; }
        .container { padding: 2rem; max-width: 1200px; margin: 0 auto; }
        .btn { padding: 0.5rem 1rem; background: #007bff; color: white; border: none; border-radius: 4px; text-decoration: none; display: inline-block; cursor: pointer; }
        .btn:hover { background: #0056b3; }
        .btn-sm { padding: 0.25rem 0.5rem; font-size: 0.875rem; }
        .btn-danger { background: #dc3545; }
        .btn-danger:hover { background: #c82333; }
        table { width: 100%; border-collapse: collapse; background: white; margin-top: 1rem; }
        th, td { padding: 0.75rem; text-align: left; border-bottom: 1px solid #dee2e6; }
        th { background: #f8f9fa; font-weight: 600; }
        tr:hover { background: #f8f9fa; }
        .status-enabled { color: #28a745; font-weight: bold; }
        .status-disabled { color: #6c757d; }
        .logout-form { display: inline; }
    </style>
</head>
<body>
    <div class="header">
        <h1>Fixity</h1>
        <div class="nav">
            <a href="/">Dashboard</a>
            <a href="/targets">Storage Targets</a>
            <a href="/scans">Scans</a>
            <a href="/files">Files</a>` + func() string {
		if user.IsAdmin {
			return `<a href="/users">Users</a>`
		}
		return ""
	}() + `
            <span>|</span>
            <span>` + user.Username + `</span>
            <form method="POST" action="/logout" class="logout-form">
                <button type="submit" class="btn btn-sm">Logout</button>
            </form>
        </div>
    </div>
    <div class="container">
        <h2>Storage Targets</h2>
        <a href="/targets/new" class="btn">Add New Target</a>
        <table>
            <thead>
                <tr>
                    <th>Name</th>
                    <th>Type</th>
                    <th>Path</th>
                    <th>Status</th>
                    <th>Actions</th>
                </tr>
            </thead>
            <tbody>`

	if len(targets) == 0 {
		html += `<tr><td colspan="5">No storage targets configured.</td></tr>`
	} else {
		for _, target := range targets {
			statusClass := "status-disabled"
			status := "Disabled"
			if target.Enabled {
				statusClass = "status-enabled"
				status = "Enabled"
			}
			html += fmt.Sprintf(`
                <tr>
                    <td>%s</td>
                    <td>%s</td>
                    <td>%s</td>
                    <td class="%s">%s</td>
                    <td>
                        <a href="/targets/%d" class="btn btn-sm">View</a>
                        <a href="/targets/%d/edit" class="btn btn-sm">Edit</a>
                        <form method="POST" action="/targets/%d/scan" style="display:inline;">
                            <button type="submit" class="btn btn-sm">Scan</button>
                        </form>
                    </td>
                </tr>`,
				target.Name,
				target.Type,
				target.Path,
				statusClass,
				status,
				target.ID,
				target.ID,
				target.ID,
			)
		}
	}

	html += `
            </tbody>
        </table>
    </div>
</body>
</html>`

	w.Write([]byte(html))
}

func (s *Server) handleNewTargetPage(w http.ResponseWriter, r *http.Request) {
	user := s.getCurrentUser(r)

	data := map[string]interface{}{
		"User":  user,
		"Error": "",
	}

	if s.templates != nil {
		if err := s.templates.ExecuteTemplate(w, "target_new.html", data); err != nil {
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
		return
	}

	s.renderSimpleTargetForm(w, data, nil)
}

func (s *Server) renderSimpleTargetForm(w http.ResponseWriter, data map[string]interface{}, target *database.StorageTarget) {
	w.Header().Set("Content-Type", "text/html")
	user := data["User"].(*database.User)
	errorMsg := data["Error"].(string)

	isEdit := target != nil
	title := "Add New Storage Target"
	action := "/targets"
	method := "POST"

	if isEdit {
		title = "Edit Storage Target"
		action = fmt.Sprintf("/targets/%d", target.ID)
		method = "POST" // Browser forms only support GET/POST, we'll use hidden field
	}

	name := ""
	targetType := "local"
	path := ""
	server := ""
	share := ""
	enabled := true

	if target != nil {
		name = target.Name
		targetType = string(target.Type)
		path = target.Path
		enabled = target.Enabled
		if target.Server != nil {
			server = *target.Server
		}
		if target.Share != nil {
			share = *target.Share
		}
	}

	html := `
<!DOCTYPE html>
<html>
<head>
    <title>Fixity - ` + title + `</title>
    <style>
        body { font-family: Arial, sans-serif; margin: 0; padding: 0; }
        .header { background: #2c3e50; color: white; padding: 1rem 2rem; display: flex; justify-content: space-between; align-items: center; }
        .nav { display: flex; gap: 1rem; }
        .nav a { color: white; text-decoration: none; }
        .nav a:hover { text-decoration: underline; }
        .container { padding: 2rem; max-width: 600px; margin: 0 auto; }
        .form-group { margin-bottom: 1.5rem; }
        .form-group label { display: block; font-weight: bold; margin-bottom: 0.5rem; }
        .form-group input[type="text"],
        .form-group select { width: 100%; padding: 0.5rem; border: 1px solid #dee2e6; border-radius: 4px; }
        .form-group input[type="checkbox"] { margin-right: 0.5rem; }
        .form-group small { display: block; margin-top: 0.25rem; color: #6c757d; font-size: 0.875rem; }
        .network-fields { display: none; }
        .btn { padding: 0.5rem 1rem; background: #007bff; color: white; border: none; border-radius: 4px; cursor: pointer; }
        .btn:hover { background: #0056b3; }
        .btn-secondary { background: #6c757d; margin-left: 0.5rem; }
        .btn-secondary:hover { background: #5a6268; }
        .error { color: #721c24; padding: 1rem; background: #f8d7da; margin-bottom: 1rem; border: 1px solid #f5c6cb; border-radius: 4px; }
        .logout-form { display: inline; }
        .btn-sm { padding: 0.25rem 0.5rem; font-size: 0.875rem; }
    </style>
</head>
<body>
    <div class="header">
        <h1>Fixity</h1>
        <div class="nav">
            <a href="/">Dashboard</a>
            <a href="/targets">Storage Targets</a>
            <a href="/scans">Scans</a>
            <a href="/files">Files</a>` + func() string {
		if user.IsAdmin {
			return `<a href="/users">Users</a>`
		}
		return ""
	}() + `
            <span>|</span>
            <span>` + user.Username + `</span>
            <form method="POST" action="/logout" class="logout-form">
                <button type="submit" class="btn btn-sm">Logout</button>
            </form>
        </div>
    </div>
    <div class="container">
        <h2>` + title + `</h2>`

	if errorMsg != "" {
		html += `<div class="error">` + errorMsg + `</div>`
	}

	html += `
        <form method="` + method + `" action="` + action + `">`

	if isEdit {
		html += `<input type="hidden" name="_method" value="PUT">`
	}

	html += `
            <div class="form-group">
                <label for="name">Name</label>
                <input type="text" id="name" name="name" value="` + name + `" required>
            </div>
            <div class="form-group">
                <label for="type">Type</label>
                <select id="type" name="type" required>
                    <option value="local"` + func() string {
		if targetType == "local" {
			return ` selected`
		}
		return ""
	}() + `>Local Filesystem</option>
                    <option value="nfs"` + func() string {
		if targetType == "nfs" {
			return ` selected`
		}
		return ""
	}() + `>NFS (Network File System)</option>
                    <option value="smb"` + func() string {
		if targetType == "smb" {
			return ` selected`
		}
		return ""
	}() + `>SMB/CIFS (Windows Share)</option>
                </select>
                <small>Local: Local filesystem path | NFS: NFS server mount | SMB: Windows/Samba share</small>
            </div>
            <div class="form-group network-fields" id="server-field">
                <label for="server">Server Address</label>
                <input type="text" id="server" name="server" value="` + server + `" placeholder="e.g., nfs.example.com or 192.168.1.100">
                <small>Hostname or IP address of the NFS/SMB server</small>
            </div>
            <div class="form-group network-fields" id="share-field">
                <label for="share">Share Path/Name</label>
                <input type="text" id="share" name="share" value="` + share + `" placeholder="e.g., /exports/data or ShareName">
                <small>NFS: export path (e.g., /exports/data) | SMB: share name (e.g., Documents)</small>
            </div>
            <div class="form-group">
                <label for="path">Mount Path</label>
                <input type="text" id="path" name="path" value="` + path + `" required placeholder="e.g., /mnt/nfs or /mnt/smb">
                <small>Local: directory path | NFS/SMB: local mount point path</small>
            </div>
            <script>
                function updateFieldVisibility() {
                    const type = document.getElementById('type').value;
                    const networkFields = document.querySelectorAll('.network-fields');
                    const pathField = document.getElementById('path');
                    const serverField = document.getElementById('server');
                    const shareField = document.getElementById('share');

                    if (type === 'local') {
                        networkFields.forEach(field => field.style.display = 'none');
                        serverField.removeAttribute('required');
                        shareField.removeAttribute('required');
                        pathField.placeholder = '/path/to/directory';
                    } else {
                        networkFields.forEach(field => field.style.display = 'block');
                        serverField.setAttribute('required', 'required');
                        shareField.setAttribute('required', 'required');
                        if (type === 'nfs') {
                            pathField.placeholder = '/mnt/nfs';
                            shareField.placeholder = '/exports/data';
                        } else {
                            pathField.placeholder = '/mnt/smb';
                            shareField.placeholder = 'ShareName';
                        }
                    }
                }

                // Run on page load
                updateFieldVisibility();

                // Run when type changes
                document.getElementById('type').addEventListener('change', updateFieldVisibility);
            </script>
            <div class="form-group">
                <label>
                    <input type="checkbox" name="enabled" value="true"` + func() string {
		if enabled {
			return ` checked`
		}
		return ""
	}() + `>
                    Enabled
                </label>
            </div>
            <button type="submit" class="btn">Save</button>
            <a href="/targets" class="btn btn-secondary">Cancel</a>
        </form>
    </div>
</body>
</html>`

	w.Write([]byte(html))
}

func (s *Server) handleCreateTarget(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	name := r.FormValue("name")
	targetType := r.FormValue("type")
	path := r.FormValue("path")
	server := r.FormValue("server")
	share := r.FormValue("share")
	enabled := r.FormValue("enabled") == "true"

	// Validate type
	if targetType != "local" && targetType != "nfs" && targetType != "smb" {
		user := s.getCurrentUser(r)
		data := map[string]interface{}{
			"User":  user,
			"Error": "Invalid storage type",
		}
		s.renderSimpleTargetForm(w, data, nil)
		return
	}

	// Validate required fields based on type
	if targetType == "nfs" || targetType == "smb" {
		if server == "" {
			user := s.getCurrentUser(r)
			data := map[string]interface{}{
				"User":  user,
				"Error": "Server address is required for NFS/SMB targets",
			}
			s.renderSimpleTargetForm(w, data, nil)
			return
		}
		if share == "" {
			user := s.getCurrentUser(r)
			data := map[string]interface{}{
				"User":  user,
				"Error": "Share path/name is required for NFS/SMB targets",
			}
			s.renderSimpleTargetForm(w, data, nil)
			return
		}
	}

	// Create target with default values for required fields
	target := &database.StorageTarget{
		Name:                name,
		Type:                database.StorageType(targetType),
		Path:                path,
		Enabled:             enabled,
		ParallelWorkers:     1,
		RandomSamplePercent: 1.0,
		ChecksumAlgorithm:   "md5",
		CheckpointInterval:  1000,
		BatchSize:           1000,
	}

	// Set server and share for NFS/SMB
	if targetType == "nfs" || targetType == "smb" {
		target.Server = &server
		target.Share = &share
	}

	err := s.db.StorageTargets.Create(r.Context(), target)
	if err != nil {
		user := s.getCurrentUser(r)
		data := map[string]interface{}{
			"User":  user,
			"Error": fmt.Sprintf("Failed to create target: %v", err),
		}
		s.renderSimpleTargetForm(w, data, nil)
		return
	}

	http.Redirect(w, r, "/targets", http.StatusSeeOther)
}

func (s *Server) handleViewTarget(w http.ResponseWriter, r *http.Request) {
	targetID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid target ID", http.StatusBadRequest)
		return
	}

	user := s.getCurrentUser(r)
	target, err := s.db.StorageTargets.GetByID(r.Context(), targetID)
	if err != nil || target == nil {
		http.Error(w, "Target not found", http.StatusNotFound)
		return
	}

	// Get recent scans for this target
	recentScans, _ := s.db.Scans.List(r.Context(), database.ScanFilters{
		StorageTargetID: &targetID,
		Limit:           20,
	})

	data := map[string]interface{}{
		"User":        user,
		"Target":      target,
		"RecentScans": recentScans,
	}

	if s.templates != nil {
		if err := s.templates.ExecuteTemplate(w, "target_view.html", data); err != nil {
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
		return
	}

	s.renderSimpleTargetView(w, data)
}

func (s *Server) renderSimpleTargetView(w http.ResponseWriter, data map[string]interface{}) {
	w.Header().Set("Content-Type", "text/html")
	user := data["User"].(*database.User)
	target := data["Target"].(*database.StorageTarget)
	recentScans := data["RecentScans"].([]*database.Scan)

	status := "Disabled"
	statusClass := "status-disabled"
	if target.Enabled {
		status = "Enabled"
		statusClass = "status-enabled"
	}

	html := `
<!DOCTYPE html>
<html>
<head>
    <title>Fixity - Storage Target</title>
    <style>
        body { font-family: Arial, sans-serif; margin: 0; padding: 0; }
        .header { background: #2c3e50; color: white; padding: 1rem 2rem; display: flex; justify-content: space-between; align-items: center; }
        .nav { display: flex; gap: 1rem; }
        .nav a { color: white; text-decoration: none; }
        .nav a:hover { text-decoration: underline; }
        .container { padding: 2rem; max-width: 1200px; margin: 0 auto; }
        .info-card { background: #f8f9fa; padding: 1.5rem; border-radius: 8px; margin-bottom: 2rem; }
        .info-row { display: flex; margin-bottom: 0.75rem; }
        .info-label { font-weight: bold; width: 150px; }
        .info-value { flex: 1; }
        .btn { padding: 0.5rem 1rem; background: #007bff; color: white; border: none; border-radius: 4px; text-decoration: none; display: inline-block; cursor: pointer; }
        .btn:hover { background: #0056b3; }
        .btn-sm { padding: 0.25rem 0.5rem; font-size: 0.875rem; }
        .btn-danger { background: #dc3545; }
        .btn-danger:hover { background: #c82333; }
        .btn-secondary { background: #6c757d; margin-left: 0.5rem; }
        .btn-secondary:hover { background: #5a6268; }
        .status-enabled { color: #28a745; font-weight: bold; }
        .status-disabled { color: #6c757d; }
        .status-running { color: #28a745; }
        .status-completed { color: #6c757d; }
        .status-failed { color: #dc3545; }
        table { width: 100%; border-collapse: collapse; background: white; margin-top: 1rem; }
        th, td { padding: 0.75rem; text-align: left; border-bottom: 1px solid #dee2e6; }
        th { background: #f8f9fa; font-weight: 600; }
        tr:hover { background: #f8f9fa; }
        .logout-form { display: inline; }
        .actions { margin-bottom: 2rem; }
    </style>
</head>
<body>
    <div class="header">
        <h1>Fixity</h1>
        <div class="nav">
            <a href="/">Dashboard</a>
            <a href="/targets">Storage Targets</a>
            <a href="/scans">Scans</a>
            <a href="/files">Files</a>` + func() string {
		if user.IsAdmin {
			return `<a href="/users">Users</a>`
		}
		return ""
	}() + `
            <span>|</span>
            <span>` + user.Username + `</span>
            <form method="POST" action="/logout" class="logout-form">
                <button type="submit" class="btn btn-sm">Logout</button>
            </form>
        </div>
    </div>
    <div class="container">
        <h2>Storage Target: ` + target.Name + `</h2>

        <div class="actions">
            <a href="/targets/` + strconv.FormatInt(target.ID, 10) + `/edit" class="btn">Edit</a>
            <form method="POST" action="/targets/` + strconv.FormatInt(target.ID, 10) + `/scan" style="display:inline;">
                <button type="submit" class="btn">Scan Now</button>
            </form>
            <a href="/targets" class="btn btn-secondary">Back to List</a>
            <form method="POST" action="/targets/` + strconv.FormatInt(target.ID, 10) + `" style="display:inline;">
                <input type="hidden" name="_method" value="DELETE">
                <button type="submit" class="btn btn-danger" onclick="return confirm('Are you sure you want to delete this target?')">Delete</button>
            </form>
        </div>

        <div class="info-card">
            <div class="info-row">
                <div class="info-label">Name:</div>
                <div class="info-value">` + target.Name + `</div>
            </div>
            <div class="info-row">
                <div class="info-label">Type:</div>
                <div class="info-value">` + string(target.Type) + `</div>
            </div>
            <div class="info-row">
                <div class="info-label">Path:</div>
                <div class="info-value">` + target.Path + `</div>
            </div>
            <div class="info-row">
                <div class="info-label">Status:</div>
                <div class="info-value ` + statusClass + `">` + status + `</div>
            </div>
            <div class="info-row">
                <div class="info-label">Created:</div>
                <div class="info-value">` + target.CreatedAt.Format("2006-01-02 15:04:05") + `</div>
            </div>
        </div>

        <h3>Recent Scans</h3>`

	if len(recentScans) == 0 {
		html += `<p>No scans yet for this target.</p>`
	} else {
		html += `
        <table>
            <thead>
                <tr>
                    <th>Status</th>
                    <th>Started</th>
                    <th>Completed</th>
                    <th>Files Scanned</th>
                    <th>Changes</th>
                    <th>Actions</th>
                </tr>
            </thead>
            <tbody>`

		for _, scan := range recentScans {
			completedStr := "In Progress"
			if scan.CompletedAt != nil {
				completedStr = scan.CompletedAt.Format("2006-01-02 15:04")
			}

			statusClass := "status-" + string(scan.Status)
			html += fmt.Sprintf(`
                <tr>
                    <td class="%s">%s</td>
                    <td>%s</td>
                    <td>%s</td>
                    <td>%d</td>
                    <td>+%d / ~%d / -%d</td>
                    <td><a href="/scans/%d" class="btn btn-sm">View</a></td>
                </tr>`,
				statusClass,
				scan.Status,
				scan.StartedAt.Format("2006-01-02 15:04"),
				completedStr,
				scan.FilesScanned,
				scan.FilesAdded,
				scan.FilesModified,
				scan.FilesDeleted,
				scan.ID,
			)
		}

		html += `
            </tbody>
        </table>`
	}

	html += `
    </div>
</body>
</html>`

	w.Write([]byte(html))
}

func (s *Server) handleEditTargetPage(w http.ResponseWriter, r *http.Request) {
	targetID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid target ID", http.StatusBadRequest)
		return
	}

	user := s.getCurrentUser(r)
	target, err := s.db.StorageTargets.GetByID(r.Context(), targetID)
	if err != nil || target == nil {
		http.Error(w, "Target not found", http.StatusNotFound)
		return
	}

	data := map[string]interface{}{
		"User":  user,
		"Error": "",
	}

	if s.templates != nil {
		data["Target"] = target
		if err := s.templates.ExecuteTemplate(w, "target_edit.html", data); err != nil {
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
		return
	}

	s.renderSimpleTargetForm(w, data, target)
}

func (s *Server) handleUpdateTarget(w http.ResponseWriter, r *http.Request) {
	targetID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid target ID", http.StatusBadRequest)
		return
	}

	// Get existing target
	target, err := s.db.StorageTargets.GetByID(r.Context(), targetID)
	if err != nil || target == nil {
		http.Error(w, "Target not found", http.StatusNotFound)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	// Check for method override (since browsers only support GET/POST)
	if r.FormValue("_method") == "DELETE" {
		s.handleDeleteTarget(w, r)
		return
	}

	name := r.FormValue("name")
	targetType := r.FormValue("type")
	path := r.FormValue("path")
	enabled := r.FormValue("enabled") == "true"

	// Validate type
	if targetType != "local" && targetType != "nfs" && targetType != "smb" {
		user := s.getCurrentUser(r)
		data := map[string]interface{}{
			"User":  user,
			"Error": "Invalid storage type",
		}
		s.renderSimpleTargetForm(w, data, target)
		return
	}

	// Update target
	target.Name = name
	target.Type = database.StorageType(targetType)
	target.Path = path
	target.Enabled = enabled

	err = s.db.StorageTargets.Update(r.Context(), target)
	if err != nil {
		user := s.getCurrentUser(r)
		data := map[string]interface{}{
			"User":  user,
			"Error": fmt.Sprintf("Failed to update target: %v", err),
		}
		s.renderSimpleTargetForm(w, data, target)
		return
	}

	http.Redirect(w, r, fmt.Sprintf("/targets/%d", targetID), http.StatusSeeOther)
}

func (s *Server) handleDeleteTarget(w http.ResponseWriter, r *http.Request) {
	targetID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid target ID", http.StatusBadRequest)
		return
	}

	// Check if target exists
	target, err := s.db.StorageTargets.GetByID(r.Context(), targetID)
	if err != nil || target == nil {
		http.Error(w, "Target not found", http.StatusNotFound)
		return
	}

	// Delete the target
	err = s.db.StorageTargets.Delete(r.Context(), targetID)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to delete target: %v", err), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/targets", http.StatusSeeOther)
}

func (s *Server) handleTriggerScan(w http.ResponseWriter, r *http.Request) {
	targetID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid target ID", http.StatusBadRequest)
		return
	}

	// Verify target exists
	_, err = s.db.StorageTargets.GetByID(r.Context(), targetID)
	if err != nil {
		http.Error(w, "Target not found", http.StatusNotFound)
		return
	}

	// Trigger scan in background
	go func() {
		s.coordinator.ScanTarget(r.Context(), targetID)
	}()

	// Redirect back to dashboard
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (s *Server) handleListScans(w http.ResponseWriter, r *http.Request) {
	user := s.getCurrentUser(r)

	// Get all scans
	scans, _ := s.db.Scans.List(r.Context(), database.ScanFilters{
		Limit: 100,
	})

	// Get all targets to map names
	targets, _ := s.db.StorageTargets.ListAll(r.Context())
	targetMap := make(map[int64]string)
	for _, t := range targets {
		targetMap[t.ID] = t.Name
	}

	data := map[string]interface{}{
		"User":      user,
		"Scans":     scans,
		"TargetMap": targetMap,
	}

	if s.templates != nil {
		if err := s.templates.ExecuteTemplate(w, "scans_list.html", data); err != nil {
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
		return
	}

	s.renderSimpleScansList(w, data)
}

func (s *Server) renderSimpleScansList(w http.ResponseWriter, data map[string]interface{}) {
	w.Header().Set("Content-Type", "text/html")
	user := data["User"].(*database.User)
	scans := data["Scans"].([]*database.Scan)
	targetMap := data["TargetMap"].(map[int64]string)

	html := `
<!DOCTYPE html>
<html>
<head>
    <title>Fixity - Scans</title>
    <style>
        body { font-family: Arial, sans-serif; margin: 0; padding: 0; }
        .header { background: #2c3e50; color: white; padding: 1rem 2rem; display: flex; justify-content: space-between; align-items: center; }
        .nav { display: flex; gap: 1rem; }
        .nav a { color: white; text-decoration: none; }
        .nav a:hover { text-decoration: underline; }
        .container { padding: 2rem; max-width: 1200px; margin: 0 auto; }
        .btn { padding: 0.5rem 1rem; background: #007bff; color: white; border: none; border-radius: 4px; text-decoration: none; display: inline-block; }
        .btn:hover { background: #0056b3; }
        .btn-sm { padding: 0.25rem 0.5rem; font-size: 0.875rem; }
        table { width: 100%; border-collapse: collapse; background: white; margin-top: 1rem; }
        th, td { padding: 0.75rem; text-align: left; border-bottom: 1px solid #dee2e6; }
        th { background: #f8f9fa; font-weight: 600; }
        tr:hover { background: #f8f9fa; }
        .status-running { color: #28a745; font-weight: bold; }
        .status-completed { color: #6c757d; }
        .status-failed { color: #dc3545; font-weight: bold; }
        .logout-form { display: inline; }
    </style>
</head>
<body>
    <div class="header">
        <h1>Fixity</h1>
        <div class="nav">
            <a href="/">Dashboard</a>
            <a href="/targets">Storage Targets</a>
            <a href="/scans">Scans</a>
            <a href="/files">Files</a>` + func() string {
		if user.IsAdmin {
			return `<a href="/users">Users</a>`
		}
		return ""
	}() + `
            <span>|</span>
            <span>` + user.Username + `</span>
            <form method="POST" action="/logout" class="logout-form">
                <button type="submit" class="btn btn-sm">Logout</button>
            </form>
        </div>
    </div>
    <div class="container">
        <h2>All Scans</h2>
        <table>
            <thead>
                <tr>
                    <th>ID</th>
                    <th>Target</th>
                    <th>Status</th>
                    <th>Started</th>
                    <th>Completed</th>
                    <th>Files</th>
                    <th>Changes</th>
                    <th>Actions</th>
                </tr>
            </thead>
            <tbody>`

	if len(scans) == 0 {
		html += `<tr><td colspan="8">No scans found.</td></tr>`
	} else {
		for _, scan := range scans {
			targetName := targetMap[scan.StorageTargetID]
			if targetName == "" {
				targetName = "Unknown"
			}

			completedStr := "In Progress"
			if scan.CompletedAt != nil {
				completedStr = scan.CompletedAt.Format("2006-01-02 15:04")
			}

			statusClass := "status-" + string(scan.Status)
			html += fmt.Sprintf(`
                <tr>
                    <td>%d</td>
                    <td>%s</td>
                    <td class="%s">%s</td>
                    <td>%s</td>
                    <td>%s</td>
                    <td>%d</td>
                    <td>+%d / ~%d / -%d</td>
                    <td><a href="/scans/%d" class="btn btn-sm">View</a></td>
                </tr>`,
				scan.ID,
				targetName,
				statusClass,
				scan.Status,
				scan.StartedAt.Format("2006-01-02 15:04"),
				completedStr,
				scan.FilesScanned,
				scan.FilesAdded,
				scan.FilesModified,
				scan.FilesDeleted,
				scan.ID,
			)
		}
	}

	html += `
            </tbody>
        </table>
    </div>
</body>
</html>`

	w.Write([]byte(html))
}

func (s *Server) handleViewScan(w http.ResponseWriter, r *http.Request) {
	scanID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid scan ID", http.StatusBadRequest)
		return
	}

	user := s.getCurrentUser(r)
	scan, err := s.db.Scans.GetByID(r.Context(), scanID)
	if err != nil || scan == nil {
		http.Error(w, "Scan not found", http.StatusNotFound)
		return
	}

	// Get target
	target, _ := s.db.StorageTargets.GetByID(r.Context(), scan.StorageTargetID)

	// Get change events for this scan
	changeEvents, _ := s.db.ChangeEvents.GetByScan(r.Context(), scanID)

	// Get file paths for change events
	filePathMap := make(map[int64]string)
	for _, event := range changeEvents {
		if _, exists := filePathMap[event.FileID]; !exists {
			if file, err := s.db.Files.GetByID(r.Context(), event.FileID); err == nil && file != nil {
				filePathMap[event.FileID] = file.Path
			} else {
				filePathMap[event.FileID] = "Unknown"
			}
		}
	}

	data := map[string]interface{}{
		"User":         user,
		"Scan":         scan,
		"Target":       target,
		"ChangeEvents": changeEvents,
		"FilePathMap":  filePathMap,
	}

	if s.templates != nil {
		if err := s.templates.ExecuteTemplate(w, "scan_view.html", data); err != nil {
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
		return
	}

	s.renderSimpleScanView(w, data)
}

func (s *Server) renderSimpleScanView(w http.ResponseWriter, data map[string]interface{}) {
	w.Header().Set("Content-Type", "text/html")
	user := data["User"].(*database.User)
	scan := data["Scan"].(*database.Scan)
	target := data["Target"].(*database.StorageTarget)
	changeEvents := data["ChangeEvents"].([]*database.ChangeEvent)
	filePathMap := data["FilePathMap"].(map[int64]string)

	targetName := "Unknown"
	if target != nil {
		targetName = target.Name
	}

	completedStr := "In Progress"
	duration := ""
	if scan.CompletedAt != nil {
		completedStr = scan.CompletedAt.Format("2006-01-02 15:04:05")
		duration = scan.CompletedAt.Sub(scan.StartedAt).Round(time.Second).String()
	}

	statusClass := "status-" + string(scan.Status)

	html := `
<!DOCTYPE html>
<html>
<head>
    <title>Fixity - Scan Details</title>
    <style>
        body { font-family: Arial, sans-serif; margin: 0; padding: 0; }
        .header { background: #2c3e50; color: white; padding: 1rem 2rem; display: flex; justify-content: space-between; align-items: center; }
        .nav { display: flex; gap: 1rem; }
        .nav a { color: white; text-decoration: none; }
        .nav a:hover { text-decoration: underline; }
        .container { padding: 2rem; max-width: 1200px; margin: 0 auto; }
        .info-card { background: #f8f9fa; padding: 1.5rem; border-radius: 8px; margin-bottom: 2rem; }
        .info-row { display: flex; margin-bottom: 0.75rem; }
        .info-label { font-weight: bold; width: 200px; }
        .info-value { flex: 1; }
        .status-running { color: #28a745; font-weight: bold; }
        .status-completed { color: #6c757d; }
        .status-failed { color: #dc3545; font-weight: bold; }
        .stats { display: grid; grid-template-columns: repeat(4, 1fr); gap: 1rem; margin-bottom: 2rem; }
        .stat-card { background: #f8f9fa; padding: 1.5rem; border-radius: 8px; text-align: center; }
        .stat-value { font-size: 2rem; font-weight: bold; color: #007bff; }
        .stat-label { color: #6c757d; margin-top: 0.5rem; }
        .btn { padding: 0.5rem 1rem; background: #007bff; color: white; border: none; border-radius: 4px; text-decoration: none; display: inline-block; }
        .btn:hover { background: #0056b3; }
        .btn-sm { padding: 0.25rem 0.5rem; font-size: 0.875rem; }
        .btn-secondary { background: #6c757d; margin-left: 0.5rem; }
        .btn-secondary:hover { background: #5a6268; }
        table { width: 100%; border-collapse: collapse; background: white; margin-top: 1rem; }
        th, td { padding: 0.75rem; text-align: left; border-bottom: 1px solid #dee2e6; }
        th { background: #f8f9fa; font-weight: 600; }
        tr:hover { background: #f8f9fa; }
        .change-added { color: #28a745; }
        .change-modified { color: #ffc107; }
        .change-deleted { color: #dc3545; }
        .change-verified { color: #17a2b8; }
        .logout-form { display: inline; }
    </style>
</head>
<body>
    <div class="header">
        <h1>Fixity</h1>
        <div class="nav">
            <a href="/">Dashboard</a>
            <a href="/targets">Storage Targets</a>
            <a href="/scans">Scans</a>
            <a href="/files">Files</a>` + func() string {
		if user.IsAdmin {
			return `<a href="/users">Users</a>`
		}
		return ""
	}() + `
            <span>|</span>
            <span>` + user.Username + `</span>
            <form method="POST" action="/logout" class="logout-form">
                <button type="submit" class="btn btn-sm">Logout</button>
            </form>
        </div>
    </div>
    <div class="container">
        <h2>Scan #` + strconv.FormatInt(scan.ID, 10) + `</h2>

        <div style="margin-bottom: 2rem;">
            <a href="/scans" class="btn btn-secondary">Back to Scans</a>
            <a href="/targets/` + strconv.FormatInt(scan.StorageTargetID, 10) + `" class="btn btn-secondary">View Target</a>
        </div>

        <div class="info-card">
            <div class="info-row">
                <div class="info-label">Target:</div>
                <div class="info-value">` + targetName + `</div>
            </div>
            <div class="info-row">
                <div class="info-label">Status:</div>
                <div class="info-value ` + statusClass + `">` + string(scan.Status) + `</div>
            </div>
            <div class="info-row">
                <div class="info-label">Started:</div>
                <div class="info-value">` + scan.StartedAt.Format("2006-01-02 15:04:05") + `</div>
            </div>
            <div class="info-row">
                <div class="info-label">Completed:</div>
                <div class="info-value">` + completedStr + `</div>
            </div>`

	if duration != "" {
		html += `
            <div class="info-row">
                <div class="info-label">Duration:</div>
                <div class="info-value">` + duration + `</div>
            </div>`
	}

	html += `
        </div>

        <div class="stats">
            <div class="stat-card">
                <div class="stat-value">` + strconv.FormatInt(scan.FilesScanned, 10) + `</div>
                <div class="stat-label">Files Scanned</div>
            </div>
            <div class="stat-card">
                <div class="stat-value change-added">+` + strconv.FormatInt(scan.FilesAdded, 10) + `</div>
                <div class="stat-label">Added</div>
            </div>
            <div class="stat-card">
                <div class="stat-value change-modified">~` + strconv.FormatInt(scan.FilesModified, 10) + `</div>
                <div class="stat-label">Modified</div>
            </div>
            <div class="stat-card">
                <div class="stat-value change-deleted">-` + strconv.FormatInt(scan.FilesDeleted, 10) + `</div>
                <div class="stat-label">Deleted</div>
            </div>
        </div>

        <h3>Change Events</h3>`

	if len(changeEvents) == 0 {
		html += `<p>No changes detected during this scan.</p>`
	} else {
		html += `
        <table>
            <thead>
                <tr>
                    <th>Type</th>
                    <th>File Path</th>
                    <th>Timestamp</th>
                </tr>
            </thead>
            <tbody>`

		for _, event := range changeEvents {
			filePath := filePathMap[event.FileID]
			if filePath == "" {
				filePath = "Unknown"
			}

			changeClass := "change-" + string(event.EventType)
			html += fmt.Sprintf(`
                <tr>
                    <td class="%s">%s</td>
                    <td>%s</td>
                    <td>%s</td>
                </tr>`,
				changeClass,
				event.EventType,
				filePath,
				event.DetectedAt.Format("2006-01-02 15:04:05"),
			)
		}

		html += `
            </tbody>
        </table>`
	}

	html += `
    </div>
</body>
</html>`

	w.Write([]byte(html))
}

func (s *Server) handleRunningScans(w http.ResponseWriter, r *http.Request) {
	user := s.getCurrentUser(r)

	// Get running scans
	runningScans, _ := s.coordinator.GetRunningSans(r.Context())

	// Get all targets to map names
	targets, _ := s.db.StorageTargets.ListAll(r.Context())
	targetMap := make(map[int64]string)
	for _, t := range targets {
		targetMap[t.ID] = t.Name
	}

	data := map[string]interface{}{
		"User":         user,
		"RunningScans": runningScans,
		"TargetMap":    targetMap,
	}

	if s.templates != nil {
		if err := s.templates.ExecuteTemplate(w, "scans_running.html", data); err != nil {
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
		return
	}

	s.renderSimpleRunningScans(w, data)
}

func (s *Server) renderSimpleRunningScans(w http.ResponseWriter, data map[string]interface{}) {
	w.Header().Set("Content-Type", "text/html")
	user := data["User"].(*database.User)
	runningScans := data["RunningScans"].([]*coordinator.ScanStatus)
	targetMap := data["TargetMap"].(map[int64]string)

	html := `
<!DOCTYPE html>
<html>
<head>
    <title>Fixity - Running Scans</title>
    <meta http-equiv="refresh" content="5">
    <style>
        body { font-family: Arial, sans-serif; margin: 0; padding: 0; }
        .header { background: #2c3e50; color: white; padding: 1rem 2rem; display: flex; justify-content: space-between; align-items: center; }
        .nav { display: flex; gap: 1rem; }
        .nav a { color: white; text-decoration: none; }
        .nav a:hover { text-decoration: underline; }
        .container { padding: 2rem; max-width: 1200px; margin: 0 auto; }
        .refresh-note { background: #d1ecf1; color: #0c5460; padding: 0.75rem; border-radius: 4px; margin-bottom: 1rem; }
        table { width: 100%; border-collapse: collapse; background: white; margin-top: 1rem; }
        th, td { padding: 0.75rem; text-align: left; border-bottom: 1px solid #dee2e6; }
        th { background: #f8f9fa; font-weight: 600; }
        tr:hover { background: #f8f9fa; }
        .btn { padding: 0.5rem 1rem; background: #007bff; color: white; border: none; border-radius: 4px; text-decoration: none; display: inline-block; }
        .btn:hover { background: #0056b3; }
        .btn-sm { padding: 0.25rem 0.5rem; font-size: 0.875rem; }
        .logout-form { display: inline; }
    </style>
</head>
<body>
    <div class="header">
        <h1>Fixity</h1>
        <div class="nav">
            <a href="/">Dashboard</a>
            <a href="/targets">Storage Targets</a>
            <a href="/scans">Scans</a>
            <a href="/files">Files</a>` + func() string {
		if user.IsAdmin {
			return `<a href="/users">Users</a>`
		}
		return ""
	}() + `
            <span>|</span>
            <span>` + user.Username + `</span>
            <form method="POST" action="/logout" class="logout-form">
                <button type="submit" class="btn btn-sm">Logout</button>
            </form>
        </div>
    </div>
    <div class="container">
        <h2>Running Scans</h2>
        <div class="refresh-note">This page auto-refreshes every 5 seconds</div>
        <table>
            <thead>
                <tr>
                    <th>ID</th>
                    <th>Target</th>
                    <th>Started</th>
                    <th>Files Scanned</th>
                    <th>Actions</th>
                </tr>
            </thead>
            <tbody>`

	if len(runningScans) == 0 {
		html += `<tr><td colspan="5">No scans currently running.</td></tr>`
	} else {
		for _, scan := range runningScans {
			targetName := targetMap[scan.TargetID]
			if targetName == "" {
				targetName = "Unknown"
			}

			html += fmt.Sprintf(`
                <tr>
                    <td>%d</td>
                    <td>%s</td>
                    <td>%s</td>
                    <td>%d</td>
                    <td><a href="/scans/%d" class="btn btn-sm">View</a></td>
                </tr>`,
				scan.ScanID,
				targetName,
				scan.StartedAt.Format("2006-01-02 15:04:05"),
				scan.Progress.FilesScanned,
				scan.ScanID,
			)
		}
	}

	html += `
            </tbody>
        </table>
    </div>
</body>
</html>`

	w.Write([]byte(html))
}

func (s *Server) handleBrowseFiles(w http.ResponseWriter, r *http.Request) {
	user := s.getCurrentUser(r)

	// Parse query parameters for filtering
	targetIDStr := r.URL.Query().Get("target")
	var targetID *int64
	if targetIDStr != "" {
		if id, err := strconv.ParseInt(targetIDStr, 10, 64); err == nil {
			targetID = &id
		}
	}

	// Get files with optional target filter
	files, _ := s.db.Files.List(r.Context(), database.FileFilters{
		StorageTargetID: targetID,
		Limit:           100,
	})

	// Get all targets for filter dropdown
	targets, _ := s.db.StorageTargets.ListAll(r.Context())
	targetMap := make(map[int64]string)
	for _, t := range targets {
		targetMap[t.ID] = t.Name
	}

	data := map[string]interface{}{
		"User":             user,
		"Files":            files,
		"Targets":          targets,
		"TargetMap":        targetMap,
		"SelectedTargetID": targetID,
	}

	if s.templates != nil {
		if err := s.templates.ExecuteTemplate(w, "files_browse.html", data); err != nil {
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
		return
	}

	s.renderSimpleFilesBrowse(w, data)
}

func (s *Server) renderSimpleFilesBrowse(w http.ResponseWriter, data map[string]interface{}) {
	w.Header().Set("Content-Type", "text/html")
	user := data["User"].(*database.User)
	files := data["Files"].([]*database.File)
	targets := data["Targets"].([]*database.StorageTarget)
	targetMap := data["TargetMap"].(map[int64]string)
	selectedTargetID := data["SelectedTargetID"].(*int64)

	html := `
<!DOCTYPE html>
<html>
<head>
    <title>Fixity - Browse Files</title>
    <style>
        body { font-family: Arial, sans-serif; margin: 0; padding: 0; }
        .header { background: #2c3e50; color: white; padding: 1rem 2rem; display: flex; justify-content: space-between; align-items: center; }
        .nav { display: flex; gap: 1rem; }
        .nav a { color: white; text-decoration: none; }
        .nav a:hover { text-decoration: underline; }
        .container { padding: 2rem; max-width: 1200px; margin: 0 auto; }
        .filter-bar { background: #f8f9fa; padding: 1rem; border-radius: 4px; margin-bottom: 1rem; }
        .filter-bar select { padding: 0.5rem; border: 1px solid #dee2e6; border-radius: 4px; margin-right: 0.5rem; }
        .btn { padding: 0.5rem 1rem; background: #007bff; color: white; border: none; border-radius: 4px; text-decoration: none; display: inline-block; cursor: pointer; }
        .btn:hover { background: #0056b3; }
        .btn-sm { padding: 0.25rem 0.5rem; font-size: 0.875rem; }
        table { width: 100%; border-collapse: collapse; background: white; margin-top: 1rem; }
        th, td { padding: 0.75rem; text-align: left; border-bottom: 1px solid #dee2e6; }
        th { background: #f8f9fa; font-weight: 600; }
        tr:hover { background: #f8f9fa; }
        .file-path { font-family: monospace; font-size: 0.9rem; }
        .logout-form { display: inline; }
    </style>
</head>
<body>
    <div class="header">
        <h1>Fixity</h1>
        <div class="nav">
            <a href="/">Dashboard</a>
            <a href="/targets">Storage Targets</a>
            <a href="/scans">Scans</a>
            <a href="/files">Files</a>` + func() string {
		if user.IsAdmin {
			return `<a href="/users">Users</a>`
		}
		return ""
	}() + `
            <span>|</span>
            <span>` + user.Username + `</span>
            <form method="POST" action="/logout" class="logout-form">
                <button type="submit" class="btn btn-sm">Logout</button>
            </form>
        </div>
    </div>
    <div class="container">
        <h2>Browse Files</h2>

        <div class="filter-bar">
            <form method="GET" action="/files">
                <label>Filter by Target:</label>
                <select name="target" onchange="this.form.submit()">
                    <option value="">All Targets</option>`

	for _, target := range targets {
		selected := ""
		if selectedTargetID != nil && *selectedTargetID == target.ID {
			selected = " selected"
		}
		html += fmt.Sprintf(`<option value="%d"%s>%s</option>`,
			target.ID, selected, target.Name)
	}

	html += `
                </select>
            </form>
        </div>

        <table>
            <thead>
                <tr>
                    <th>Target</th>
                    <th>Path</th>
                    <th>Size</th>
                    <th>Last Checksummed</th>
                    <th>Actions</th>
                </tr>
            </thead>
            <tbody>`

	if len(files) == 0 {
		html += `<tr><td colspan="5">No files found.</td></tr>`
	} else {
		for _, file := range files {
			targetName := targetMap[file.StorageTargetID]
			if targetName == "" {
				targetName = "Unknown"
			}

			lastChecksummed := "Never"
			if file.LastChecksummedAt != nil {
				lastChecksummed = file.LastChecksummedAt.Format("2006-01-02 15:04")
			}

			// Format size in human-readable format
			sizeStr := formatBytes(file.Size)

			html += fmt.Sprintf(`
                <tr>
                    <td>%s</td>
                    <td class="file-path">%s</td>
                    <td>%s</td>
                    <td>%s</td>
                    <td>
                        <a href="/files/%d" class="btn btn-sm">View</a>
                        <a href="/files/%d/history" class="btn btn-sm">History</a>
                    </td>
                </tr>`,
				targetName,
				file.Path,
				sizeStr,
				lastChecksummed,
				file.ID,
				file.ID,
			)
		}
	}

	html += `
            </tbody>
        </table>
    </div>
</body>
</html>`

	w.Write([]byte(html))
}

func (s *Server) handleViewFile(w http.ResponseWriter, r *http.Request) {
	fileID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid file ID", http.StatusBadRequest)
		return
	}

	user := s.getCurrentUser(r)
	file, err := s.db.Files.GetByID(r.Context(), fileID)
	if err != nil || file == nil {
		http.Error(w, "File not found", http.StatusNotFound)
		return
	}

	// Get target
	target, _ := s.db.StorageTargets.GetByID(r.Context(), file.StorageTargetID)

	data := map[string]interface{}{
		"User":   user,
		"File":   file,
		"Target": target,
	}

	if s.templates != nil {
		if err := s.templates.ExecuteTemplate(w, "file_view.html", data); err != nil {
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
		return
	}

	s.renderSimpleFileView(w, data)
}

func (s *Server) renderSimpleFileView(w http.ResponseWriter, data map[string]interface{}) {
	w.Header().Set("Content-Type", "text/html")
	user := data["User"].(*database.User)
	file := data["File"].(*database.File)
	target := data["Target"].(*database.StorageTarget)

	targetName := "Unknown"
	if target != nil {
		targetName = target.Name
	}

	currentChecksum := "None"
	checksumType := ""
	if file.CurrentChecksum != nil {
		currentChecksum = *file.CurrentChecksum
		if file.ChecksumType != nil {
			checksumType = *file.ChecksumType
		}
	}

	lastChecksummed := "Never"
	if file.LastChecksummedAt != nil {
		lastChecksummed = file.LastChecksummedAt.Format("2006-01-02 15:04:05")
	}

	html := `
<!DOCTYPE html>
<html>
<head>
    <title>Fixity - File Details</title>
    <style>
        body { font-family: Arial, sans-serif; margin: 0; padding: 0; }
        .header { background: #2c3e50; color: white; padding: 1rem 2rem; display: flex; justify-content: space-between; align-items: center; }
        .nav { display: flex; gap: 1rem; }
        .nav a { color: white; text-decoration: none; }
        .nav a:hover { text-decoration: underline; }
        .container { padding: 2rem; max-width: 1200px; margin: 0 auto; }
        .info-card { background: #f8f9fa; padding: 1.5rem; border-radius: 8px; margin-bottom: 2rem; }
        .info-row { display: flex; margin-bottom: 0.75rem; }
        .info-label { font-weight: bold; width: 200px; }
        .info-value { flex: 1; font-family: monospace; word-break: break-all; }
        .btn { padding: 0.5rem 1rem; background: #007bff; color: white; border: none; border-radius: 4px; text-decoration: none; display: inline-block; }
        .btn:hover { background: #0056b3; }
        .btn-sm { padding: 0.25rem 0.5rem; font-size: 0.875rem; }
        .btn-secondary { background: #6c757d; margin-left: 0.5rem; }
        .btn-secondary:hover { background: #5a6268; }
        .logout-form { display: inline; }
    </style>
</head>
<body>
    <div class="header">
        <h1>Fixity</h1>
        <div class="nav">
            <a href="/">Dashboard</a>
            <a href="/targets">Storage Targets</a>
            <a href="/scans">Scans</a>
            <a href="/files">Files</a>` + func() string {
		if user.IsAdmin {
			return `<a href="/users">Users</a>`
		}
		return ""
	}() + `
            <span>|</span>
            <span>` + user.Username + `</span>
            <form method="POST" action="/logout" class="logout-form">
                <button type="submit" class="btn btn-sm">Logout</button>
            </form>
        </div>
    </div>
    <div class="container">
        <h2>File Details</h2>

        <div style="margin-bottom: 2rem;">
            <a href="/files" class="btn btn-secondary">Back to Files</a>
            <a href="/files/` + strconv.FormatInt(file.ID, 10) + `/history" class="btn">View History</a>
            <a href="/targets/` + strconv.FormatInt(file.StorageTargetID, 10) + `" class="btn btn-secondary">View Target</a>
        </div>

        <div class="info-card">
            <div class="info-row">
                <div class="info-label">Target:</div>
                <div class="info-value">` + targetName + `</div>
            </div>
            <div class="info-row">
                <div class="info-label">Path:</div>
                <div class="info-value">` + file.Path + `</div>
            </div>
            <div class="info-row">
                <div class="info-label">Size:</div>
                <div class="info-value">` + formatBytes(file.Size) + ` (` + strconv.FormatInt(file.Size, 10) + ` bytes)</div>
            </div>
            <div class="info-row">
                <div class="info-label">Checksum (` + checksumType + `):</div>
                <div class="info-value">` + currentChecksum + `</div>
            </div>
            <div class="info-row">
                <div class="info-label">Last Checksummed:</div>
                <div class="info-value">` + lastChecksummed + `</div>
            </div>
            <div class="info-row">
                <div class="info-label">First Seen:</div>
                <div class="info-value">` + file.CreatedAt.Format("2006-01-02 15:04:05") + `</div>
            </div>
        </div>
    </div>
</body>
</html>`

	w.Write([]byte(html))
}

func (s *Server) handleFileHistory(w http.ResponseWriter, r *http.Request) {
	fileID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid file ID", http.StatusBadRequest)
		return
	}

	user := s.getCurrentUser(r)
	file, err := s.db.Files.GetByID(r.Context(), fileID)
	if err != nil || file == nil {
		http.Error(w, "File not found", http.StatusNotFound)
		return
	}

	// Get target
	target, _ := s.db.StorageTargets.GetByID(r.Context(), file.StorageTargetID)

	// Get change events for this file
	changeEvents, _ := s.db.ChangeEvents.GetByFile(r.Context(), fileID)

	data := map[string]interface{}{
		"User":         user,
		"File":         file,
		"Target":       target,
		"ChangeEvents": changeEvents,
	}

	if s.templates != nil {
		if err := s.templates.ExecuteTemplate(w, "file_history.html", data); err != nil {
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
		return
	}

	s.renderSimpleFileHistory(w, data)
}

func (s *Server) renderSimpleFileHistory(w http.ResponseWriter, data map[string]interface{}) {
	w.Header().Set("Content-Type", "text/html")
	user := data["User"].(*database.User)
	file := data["File"].(*database.File)
	target := data["Target"].(*database.StorageTarget)
	changeEvents := data["ChangeEvents"].([]*database.ChangeEvent)

	targetName := "Unknown"
	if target != nil {
		targetName = target.Name
	}

	html := `
<!DOCTYPE html>
<html>
<head>
    <title>Fixity - File History</title>
    <style>
        body { font-family: Arial, sans-serif; margin: 0; padding: 0; }
        .header { background: #2c3e50; color: white; padding: 1rem 2rem; display: flex; justify-content: space-between; align-items: center; }
        .nav { display: flex; gap: 1rem; }
        .nav a { color: white; text-decoration: none; }
        .nav a:hover { text-decoration: underline; }
        .container { padding: 2rem; max-width: 1200px; margin: 0 auto; }
        .file-info { background: #f8f9fa; padding: 1rem; border-radius: 4px; margin-bottom: 2rem; font-family: monospace; }
        table { width: 100%; border-collapse: collapse; background: white; margin-top: 1rem; }
        th, td { padding: 0.75rem; text-align: left; border-bottom: 1px solid #dee2e6; }
        th { background: #f8f9fa; font-weight: 600; }
        tr:hover { background: #f8f9fa; }
        .change-added { color: #28a745; font-weight: bold; }
        .change-modified { color: #ffc107; font-weight: bold; }
        .change-deleted { color: #dc3545; font-weight: bold; }
        .change-verified { color: #17a2b8; }
        .btn { padding: 0.5rem 1rem; background: #007bff; color: white; border: none; border-radius: 4px; text-decoration: none; display: inline-block; }
        .btn:hover { background: #0056b3; }
        .btn-sm { padding: 0.25rem 0.5rem; font-size: 0.875rem; }
        .btn-secondary { background: #6c757d; margin-left: 0.5rem; }
        .btn-secondary:hover { background: #5a6268; }
        .logout-form { display: inline; }
    </style>
</head>
<body>
    <div class="header">
        <h1>Fixity</h1>
        <div class="nav">
            <a href="/">Dashboard</a>
            <a href="/targets">Storage Targets</a>
            <a href="/scans">Scans</a>
            <a href="/files">Files</a>` + func() string {
		if user.IsAdmin {
			return `<a href="/users">Users</a>`
		}
		return ""
	}() + `
            <span>|</span>
            <span>` + user.Username + `</span>
            <form method="POST" action="/logout" class="logout-form">
                <button type="submit" class="btn btn-sm">Logout</button>
            </form>
        </div>
    </div>
    <div class="container">
        <h2>File History</h2>

        <div style="margin-bottom: 2rem;">
            <a href="/files/` + strconv.FormatInt(file.ID, 10) + `" class="btn btn-secondary">Back to File</a>
            <a href="/files" class="btn btn-secondary">Browse Files</a>
        </div>

        <div class="file-info">
            <strong>Target:</strong> ` + targetName + `<br>
            <strong>Path:</strong> ` + file.Path + `
        </div>

        <h3>Change History</h3>`

	if len(changeEvents) == 0 {
		html += `<p>No change events recorded for this file.</p>`
	} else {
		html += `
        <table>
            <thead>
                <tr>
                    <th>Change Type</th>
                    <th>Detected</th>
                    <th>Scan ID</th>
                    <th>Previous Checksum</th>
                    <th>New Checksum</th>
                </tr>
            </thead>
            <tbody>`

		for _, event := range changeEvents {
			changeClass := "change-" + string(event.EventType)

			oldChecksum := "N/A"
			if event.OldChecksum != nil && len(*event.OldChecksum) > 16 {
				oldChecksum = (*event.OldChecksum)[:16] + "..."
			} else if event.OldChecksum != nil {
				oldChecksum = *event.OldChecksum
			}

			newChecksum := "N/A"
			if event.NewChecksum != nil && len(*event.NewChecksum) > 16 {
				newChecksum = (*event.NewChecksum)[:16] + "..."
			} else if event.NewChecksum != nil {
				newChecksum = *event.NewChecksum
			}

			html += fmt.Sprintf(`
                <tr>
                    <td class="%s">%s</td>
                    <td>%s</td>
                    <td><a href="/scans/%d" class="btn btn-sm">Scan #%d</a></td>
                    <td style="font-family: monospace; font-size: 0.85rem;">%s</td>
                    <td style="font-family: monospace; font-size: 0.85rem;">%s</td>
                </tr>`,
				changeClass,
				event.EventType,
				event.DetectedAt.Format("2006-01-02 15:04:05"),
				event.ScanID,
				event.ScanID,
				oldChecksum,
				newChecksum,
			)
		}

		html += `
            </tbody>
        </table>`
	}

	html += `
    </div>
</body>
</html>`

	w.Write([]byte(html))
}

// formatBytes formats a byte count into a human-readable string
func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

func (s *Server) handleListUsers(w http.ResponseWriter, r *http.Request) {
	user := s.getCurrentUser(r)
	users, _ := s.db.Users.ListAll(r.Context())

	data := map[string]interface{}{
		"User":  user,
		"Users": users,
	}

	if s.templates != nil {
		if err := s.templates.ExecuteTemplate(w, "users_list.html", data); err != nil {
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
		return
	}

	s.renderSimpleUsersList(w, data)
}

func (s *Server) renderSimpleUsersList(w http.ResponseWriter, data map[string]interface{}) {
	w.Header().Set("Content-Type", "text/html")
	user := data["User"].(*database.User)
	users := data["Users"].([]*database.User)

	html := `
<!DOCTYPE html>
<html>
<head>
    <title>Fixity - Users</title>
    <style>
        body { font-family: Arial, sans-serif; margin: 0; padding: 0; }
        .header { background: #2c3e50; color: white; padding: 1rem 2rem; display: flex; justify-content: space-between; align-items: center; }
        .nav { display: flex; gap: 1rem; }
        .nav a { color: white; text-decoration: none; }
        .nav a:hover { text-decoration: underline; }
        .container { padding: 2rem; max-width: 1200px; margin: 0 auto; }
        .btn { padding: 0.5rem 1rem; background: #007bff; color: white; border: none; border-radius: 4px; text-decoration: none; display: inline-block; cursor: pointer; }
        .btn:hover { background: #0056b3; }
        .btn-sm { padding: 0.25rem 0.5rem; font-size: 0.875rem; }
        .btn-danger { background: #dc3545; }
        .btn-danger:hover { background: #c82333; }
        table { width: 100%; border-collapse: collapse; background: white; margin-top: 1rem; }
        th, td { padding: 0.75rem; text-align: left; border-bottom: 1px solid #dee2e6; }
        th { background: #f8f9fa; font-weight: 600; }
        tr:hover { background: #f8f9fa; }
        .badge { padding: 0.25rem 0.5rem; border-radius: 4px; font-size: 0.75rem; font-weight: bold; }
        .badge-admin { background: #dc3545; color: white; }
        .badge-user { background: #6c757d; color: white; }
        .logout-form { display: inline; }
    </style>
</head>
<body>
    <div class="header">
        <h1>Fixity</h1>
        <div class="nav">
            <a href="/">Dashboard</a>
            <a href="/targets">Storage Targets</a>
            <a href="/scans">Scans</a>
            <a href="/files">Files</a>
            <a href="/users">Users</a>
            <span>|</span>
            <span>` + user.Username + `</span>
            <form method="POST" action="/logout" class="logout-form">
                <button type="submit" class="btn btn-sm">Logout</button>
            </form>
        </div>
    </div>
    <div class="container">
        <h2>Users</h2>
        <a href="/users/new" class="btn">Add New User</a>
        <table>
            <thead>
                <tr>
                    <th>Username</th>
                    <th>Email</th>
                    <th>Role</th>
                    <th>Created</th>
                    <th>Last Login</th>
                    <th>Actions</th>
                </tr>
            </thead>
            <tbody>`

	if len(users) == 0 {
		html += `<tr><td colspan="6">No users found.</td></tr>`
	} else {
		for _, u := range users {
			role := "User"
			badgeClass := "badge-user"
			if u.IsAdmin {
				role = "Admin"
				badgeClass = "badge-admin"
			}

			email := ""
			if u.Email != nil {
				email = *u.Email
			}

			lastLogin := "Never"
			if u.LastLogin != nil {
				lastLogin = u.LastLogin.Format("2006-01-02 15:04")
			}

			deleteBtn := ""
			if u.ID != user.ID {
				deleteBtn = fmt.Sprintf(`
                        <form method="POST" action="/users/%d" style="display:inline;">
                            <input type="hidden" name="_method" value="DELETE">
                            <button type="submit" class="btn btn-sm btn-danger" onclick="return confirm('Are you sure you want to delete this user?')">Delete</button>
                        </form>`, u.ID)
			}

			html += fmt.Sprintf(`
                <tr>
                    <td>%s</td>
                    <td>%s</td>
                    <td><span class="badge %s">%s</span></td>
                    <td>%s</td>
                    <td>%s</td>
                    <td>
                        <a href="/users/%d" class="btn btn-sm">View</a>
                        %s
                    </td>
                </tr>`,
				u.Username,
				email,
				badgeClass,
				role,
				u.CreatedAt.Format("2006-01-02"),
				lastLogin,
				u.ID,
				deleteBtn,
			)
		}
	}

	html += `
            </tbody>
        </table>
    </div>
</body>
</html>`

	w.Write([]byte(html))
}

func (s *Server) handleNewUserPage(w http.ResponseWriter, r *http.Request) {
	user := s.getCurrentUser(r)

	data := map[string]interface{}{
		"User":  user,
		"Error": "",
	}

	if s.templates != nil {
		if err := s.templates.ExecuteTemplate(w, "user_new.html", data); err != nil {
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
		return
	}

	s.renderSimpleUserForm(w, data)
}

func (s *Server) renderSimpleUserForm(w http.ResponseWriter, data map[string]interface{}) {
	w.Header().Set("Content-Type", "text/html")
	user := data["User"].(*database.User)
	errorMsg := data["Error"].(string)

	html := `
<!DOCTYPE html>
<html>
<head>
    <title>Fixity - Add New User</title>
    <style>
        body { font-family: Arial, sans-serif; margin: 0; padding: 0; }
        .header { background: #2c3e50; color: white; padding: 1rem 2rem; display: flex; justify-content: space-between; align-items: center; }
        .nav { display: flex; gap: 1rem; }
        .nav a { color: white; text-decoration: none; }
        .nav a:hover { text-decoration: underline; }
        .container { padding: 2rem; max-width: 600px; margin: 0 auto; }
        .form-group { margin-bottom: 1.5rem; }
        .form-group label { display: block; font-weight: bold; margin-bottom: 0.5rem; }
        .form-group input[type="text"],
        .form-group input[type="email"],
        .form-group input[type="password"] { width: 100%; padding: 0.5rem; border: 1px solid #dee2e6; border-radius: 4px; }
        .form-group input[type="checkbox"] { margin-right: 0.5rem; }
        .btn { padding: 0.5rem 1rem; background: #007bff; color: white; border: none; border-radius: 4px; cursor: pointer; }
        .btn:hover { background: #0056b3; }
        .btn-secondary { background: #6c757d; margin-left: 0.5rem; text-decoration: none; display: inline-block; }
        .btn-secondary:hover { background: #5a6268; }
        .error { color: #721c24; padding: 1rem; background: #f8d7da; margin-bottom: 1rem; border: 1px solid #f5c6cb; border-radius: 4px; }
        .logout-form { display: inline; }
        .btn-sm { padding: 0.25rem 0.5rem; font-size: 0.875rem; }
    </style>
</head>
<body>
    <div class="header">
        <h1>Fixity</h1>
        <div class="nav">
            <a href="/">Dashboard</a>
            <a href="/targets">Storage Targets</a>
            <a href="/scans">Scans</a>
            <a href="/files">Files</a>
            <a href="/users">Users</a>
            <span>|</span>
            <span>` + user.Username + `</span>
            <form method="POST" action="/logout" class="logout-form">
                <button type="submit" class="btn btn-sm">Logout</button>
            </form>
        </div>
    </div>
    <div class="container">
        <h2>Add New User</h2>`

	if errorMsg != "" {
		html += `<div class="error">` + errorMsg + `</div>`
	}

	html += `
        <form method="POST" action="/users">
            <div class="form-group">
                <label for="username">Username</label>
                <input type="text" id="username" name="username" required autofocus>
            </div>
            <div class="form-group">
                <label for="email">Email (optional)</label>
                <input type="email" id="email" name="email">
            </div>
            <div class="form-group">
                <label for="password">Password</label>
                <input type="password" id="password" name="password" required>
            </div>
            <div class="form-group">
                <label for="password_confirm">Confirm Password</label>
                <input type="password" id="password_confirm" name="password_confirm" required>
            </div>
            <div class="form-group">
                <label>
                    <input type="checkbox" name="is_admin" value="true">
                    Administrator
                </label>
            </div>
            <button type="submit" class="btn">Create User</button>
            <a href="/users" class="btn btn-secondary">Cancel</a>
        </form>
    </div>
</body>
</html>`

	w.Write([]byte(html))
}

func (s *Server) handleCreateUser(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	username := r.FormValue("username")
	email := r.FormValue("email")
	password := r.FormValue("password")
	passwordConfirm := r.FormValue("password_confirm")
	isAdmin := r.FormValue("is_admin") == "true"

	// Validate passwords match
	if password != passwordConfirm {
		user := s.getCurrentUser(r)
		data := map[string]interface{}{
			"User":  user,
			"Error": "Passwords do not match",
		}
		s.renderSimpleUserForm(w, data)
		return
	}

	// Create user
	_, err := s.auth.CreateUser(r.Context(), username, password, email, isAdmin)
	if err != nil {
		user := s.getCurrentUser(r)
		data := map[string]interface{}{
			"User":  user,
			"Error": fmt.Sprintf("Failed to create user: %v", err),
		}
		s.renderSimpleUserForm(w, data)
		return
	}

	http.Redirect(w, r, "/users", http.StatusSeeOther)
}

func (s *Server) handleViewUser(w http.ResponseWriter, r *http.Request) {
	userID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid user ID", http.StatusBadRequest)
		return
	}

	currentUser := s.getCurrentUser(r)
	viewUser, err := s.db.Users.GetByID(r.Context(), userID)
	if err != nil || viewUser == nil {
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}

	data := map[string]interface{}{
		"User":     currentUser,
		"ViewUser": viewUser,
	}

	if s.templates != nil {
		if err := s.templates.ExecuteTemplate(w, "user_view.html", data); err != nil {
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
		return
	}

	s.renderSimpleUserView(w, data)
}

func (s *Server) renderSimpleUserView(w http.ResponseWriter, data map[string]interface{}) {
	w.Header().Set("Content-Type", "text/html")
	user := data["User"].(*database.User)
	viewUser := data["ViewUser"].(*database.User)

	role := "User"
	badgeClass := "badge-user"
	if viewUser.IsAdmin {
		role = "Admin"
		badgeClass = "badge-admin"
	}

	email := ""
	if viewUser.Email != nil {
		email = *viewUser.Email
	}

	lastLogin := "Never"
	if viewUser.LastLogin != nil {
		lastLogin = viewUser.LastLogin.Format("2006-01-02 15:04:05")
	}

	html := `
<!DOCTYPE html>
<html>
<head>
    <title>Fixity - User Details</title>
    <style>
        body { font-family: Arial, sans-serif; margin: 0; padding: 0; }
        .header { background: #2c3e50; color: white; padding: 1rem 2rem; display: flex; justify-content: space-between; align-items: center; }
        .nav { display: flex; gap: 1rem; }
        .nav a { color: white; text-decoration: none; }
        .nav a:hover { text-decoration: underline; }
        .container { padding: 2rem; max-width: 1200px; margin: 0 auto; }
        .info-card { background: #f8f9fa; padding: 1.5rem; border-radius: 8px; margin-bottom: 2rem; }
        .info-row { display: flex; margin-bottom: 0.75rem; }
        .info-label { font-weight: bold; width: 150px; }
        .info-value { flex: 1; }
        .badge { padding: 0.25rem 0.5rem; border-radius: 4px; font-size: 0.75rem; font-weight: bold; }
        .badge-admin { background: #dc3545; color: white; }
        .badge-user { background: #6c757d; color: white; }
        .btn { padding: 0.5rem 1rem; background: #007bff; color: white; border: none; border-radius: 4px; text-decoration: none; display: inline-block; cursor: pointer; }
        .btn:hover { background: #0056b3; }
        .btn-sm { padding: 0.25rem 0.5rem; font-size: 0.875rem; }
        .btn-secondary { background: #6c757d; margin-left: 0.5rem; }
        .btn-secondary:hover { background: #5a6268; }
        .btn-danger { background: #dc3545; }
        .btn-danger:hover { background: #c82333; }
        table { width: 100%; border-collapse: collapse; background: white; margin-top: 1rem; }
        th, td { padding: 0.75rem; text-align: left; border-bottom: 1px solid #dee2e6; }
        th { background: #f8f9fa; font-weight: 600; }
        tr:hover { background: #f8f9fa; }
        .logout-form { display: inline; }
        .actions { margin-bottom: 2rem; }
    </style>
</head>
<body>
    <div class="header">
        <h1>Fixity</h1>
        <div class="nav">
            <a href="/">Dashboard</a>
            <a href="/targets">Storage Targets</a>
            <a href="/scans">Scans</a>
            <a href="/files">Files</a>
            <a href="/users">Users</a>
            <span>|</span>
            <span>` + user.Username + `</span>
            <form method="POST" action="/logout" class="logout-form">
                <button type="submit" class="btn btn-sm">Logout</button>
            </form>
        </div>
    </div>
    <div class="container">
        <h2>User: ` + viewUser.Username + `</h2>

        <div class="actions">
            <a href="/users" class="btn btn-secondary">Back to Users</a>`

	if viewUser.ID != user.ID {
		html += `
            <form method="POST" action="/users/` + strconv.FormatInt(viewUser.ID, 10) + `" style="display:inline;">
                <input type="hidden" name="_method" value="DELETE">
                <button type="submit" class="btn btn-danger" onclick="return confirm('Are you sure you want to delete this user?')">Delete User</button>
            </form>`
	}

	html += `
        </div>

        <div class="info-card">
            <div class="info-row">
                <div class="info-label">Username:</div>
                <div class="info-value">` + viewUser.Username + `</div>
            </div>
            <div class="info-row">
                <div class="info-label">Email:</div>
                <div class="info-value">` + email + `</div>
            </div>
            <div class="info-row">
                <div class="info-label">Role:</div>
                <div class="info-value"><span class="badge ` + badgeClass + `">` + role + `</span></div>
            </div>
            <div class="info-row">
                <div class="info-label">Created:</div>
                <div class="info-value">` + viewUser.CreatedAt.Format("2006-01-02 15:04:05") + `</div>
            </div>
            <div class="info-row">
                <div class="info-label">Last Login:</div>
                <div class="info-value">` + lastLogin + `</div>
            </div>
        </div>`

	html += `
    </div>
</body>
</html>`

	w.Write([]byte(html))
}

func (s *Server) handleDeleteUser(w http.ResponseWriter, r *http.Request) {
	userID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid user ID", http.StatusBadRequest)
		return
	}

	// Check if user exists
	viewUser, err := s.db.Users.GetByID(r.Context(), userID)
	if err != nil || viewUser == nil {
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}

	// Prevent self-deletion
	currentUser := s.getCurrentUser(r)
	if viewUser.ID == currentUser.ID {
		http.Error(w, "Cannot delete your own account", http.StatusForbidden)
		return
	}

	// Delete the user
	err = s.db.Users.Delete(r.Context(), userID)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to delete user: %v", err), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/users", http.StatusSeeOther)
}
