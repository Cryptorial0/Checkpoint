package main

import (
	"embed"
	"encoding/json"
	"flag"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"syscall"
	"time"
)

//go:embed web/*
var webFiles embed.FS

type ScanRequest struct {
	CustomPath  string   `json:"custom_path"`
	CustomPaths []string `json:"custom_paths"`
}

type BackupRequest struct {
	Games           []DetectedGame `json:"games"`
	DestDir         string         `json:"dest_dir"`
	CreateSingleZip bool           `json:"create_single_zip"`
}

type InspectRequest struct {
	SourcePath string `json:"source_path"`
}

type RestoreTask struct {
	Meta      GameSaveMeta `json:"meta"`
	CustomDir string       `json:"custom_dir"`
}

type RestoreRequest struct {
	SourcePath string        `json:"source_path"`
	Tasks      []RestoreTask `json:"tasks"`
}

type ExportReportRequest struct {
	Games []DetectedGame `json:"games"`
}

func main() {
	InitLogger()
	Log("Checkpoint starting up (v0.9 Beta)...")

	versionFlag := flag.Bool("v", false, "Show version and exit")
	versionLongFlag := flag.Bool("version", false, "Show version and exit")
	cliFlag := flag.Bool("cli", false, "Run in terminal CLI mode without GUI")
	nativeFlag := flag.Bool("native", false, "Force raw Win32 fallback GUI instead of modern standalone window")
	restoreFlag := flag.String("restore", "", "Path to game_saves_bundle.zip or vault directory to restore")
	autoFlag := flag.Bool("auto", false, "Automatically restore all resolvable games in CLI mode")
	flag.Parse()

	if *versionFlag || *versionLongFlag {
		fmt.Println("Checkpoint v0.9 Beta")
		return
	}

	if *cliFlag || *restoreFlag != "" {
		Log("Running in CLI mode")
		if *restoreFlag != "" {
			runCLIRestore(*restoreFlag, *autoFlag)
		} else {
			runCLI()
		}
		return
	}

	// Prefer predictable port 9119 first
	var listener net.Listener
	var err error
	var port int
	listener, err = net.Listen("tcp", "127.0.0.1:9119")
	if err == nil {
		port = 9119
	} else {
		// Fallback to random free port if 9119 is already in use
		listener, err = net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			Log("Failed to bind port: %v", err)
			os.Exit(1)
		}
		port = listener.Addr().(*net.TCPAddr).Port
	}
	url := fmt.Sprintf("http://127.0.0.1:%d", port)
	Log("Web backend configured at %s", url)

	mux := http.NewServeMux()

	// API Endpoints
	mux.HandleFunc("/api/progress", func(w http.ResponseWriter, r *http.Request) {
		p := GetProgress()
		writeJSON(w, p)
	})

	mux.HandleFunc("/api/system", func(w http.ResponseWriter, r *http.Request) {
		status := CheckSystem()
		writeJSON(w, status)
	})

	mux.HandleFunc("/api/disable-fast-startup", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		msg, err := DisableFastStartup()
		if err != nil {
			http.Error(w, fmt.Sprintf("%s (%v)", msg, err), http.StatusInternalServerError)
			return
		}
		w.Write([]byte(msg))
	})

	mux.HandleFunc("/api/enable-fast-startup", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		msg, err := EnableFastStartup()
		if err != nil {
			http.Error(w, fmt.Sprintf("%s (%v)", msg, err), http.StatusInternalServerError)
			return
		}
		w.Write([]byte(msg))
	})

	mux.HandleFunc("/api/pick-folder", func(w http.ResponseWriter, r *http.Request) {
		title := r.URL.Query().Get("title")
		if title == "" {
			title = "Select Folder"
		}
		path, err := OpenFolderPicker(title)
		if err != nil {
			writeJSON(w, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, map[string]string{"path": path})
	})

	mux.HandleFunc("/api/pick-file", func(w http.ResponseWriter, r *http.Request) {
		title := r.URL.Query().Get("title")
		if title == "" {
			title = "Select File"
		}
		filter := "ZIP Archives (*.zip)|*.zip|All Files (*.*)|*.*"
		path, err := OpenFilePicker(title, filter)
		if err != nil {
			writeJSON(w, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, map[string]string{"path": path})
	})

	mux.HandleFunc("/api/find-vaults", func(w http.ResponseWriter, r *http.Request) {
		vaults := FindCandidateVaults()
		writeJSON(w, map[string]interface{}{"vaults": vaults})
	})

	mux.HandleFunc("/api/online-games", func(w http.ResponseWriter, r *http.Request) {
		online := ScanOnlineGames()
		writeJSON(w, online)
	})

	mux.HandleFunc("/api/validate-prefix", func(w http.ResponseWriter, r *http.Request) {
		p := r.URL.Query().Get("path")
		driveC, valid, errDesc := ValidateWinePrefix(p)
		writeJSON(w, map[string]interface{}{
			"valid":   valid,
			"drive_c": driveC,
			"error":   errDesc,
		})
	})

	mux.HandleFunc("/api/drives", func(w http.ResponseWriter, r *http.Request) {
		drives := GetAvailableDrives()
		writeJSON(w, drives)
	})

	mux.HandleFunc("/api/disk-space", func(w http.ResponseWriter, r *http.Request) {
		p := r.URL.Query().Get("path")
		if p == "" {
			writeJSON(w, map[string]interface{}{"error": "missing path"})
			return
		}
		cleanPath := filepath.Clean(p)
		for {
			if fi, err := os.Stat(cleanPath); err == nil && fi.IsDir() {
				break
			}
			parent := filepath.Dir(cleanPath)
			if parent == cleanPath || parent == "" || parent == "." {
				break
			}
			cleanPath = parent
		}
		free, total, err := GetDiskFreeSpace(cleanPath)
		if err != nil {
			writeJSON(w, map[string]interface{}{"error": err.Error()})
			return
		}
		writeJSON(w, map[string]interface{}{
			"free_bytes":  free,
			"total_bytes": total,
			"free_gb":     fmt.Sprintf("%.2f", float64(free)/(1024*1024*1024)),
			"total_gb":    fmt.Sprintf("%.2f", float64(total)/(1024*1024*1024)),
		})
	})

	mux.HandleFunc("/api/scan", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var req ScanRequest
		json.NewDecoder(r.Body).Decode(&req)

		var customRoots []string
		for _, p := range req.CustomPaths {
			p = strings.TrimSpace(p)
			if p != "" {
				customRoots = append(customRoots, p)
			}
		}
		if req.CustomPath != "" {
			parts := strings.FieldsFunc(req.CustomPath, func(r rune) bool {
				return r == ';' || r == ','
			})
			for _, part := range parts {
				part = strings.TrimSpace(part)
				if part != "" {
					customRoots = append(customRoots, part)
				}
			}
		}

		// Deduplicate paths
		seen := make(map[string]bool)
		var uniqueRoots []string
		for _, p := range customRoots {
			cleaned := filepath.Clean(p)
			key := strings.ToLower(cleaned)
			if !seen[key] {
				seen[key] = true
				uniqueRoots = append(uniqueRoots, cleaned)
			}
		}

		games, err := ScanInstalledGames(uniqueRoots)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// Auto-save scan report text file for diagnostics and sharing
		reportPath, reportContent := writeScanReportFile(games)

		online := ScanOnlineGames()

		// Background telemetry submission if opted-in
		DispatchTelemetryReport(games, online)

		writeJSON(w, map[string]interface{}{
			"games":          games,
			"online_games":   online,
			"report_path":    reportPath,
			"report_content": reportContent,
		})
	})

	mux.HandleFunc("/api/telemetry/status", func(w http.ResponseWriter, r *http.Request) {
		cfg := LoadConfig()
		writeJSON(w, map[string]interface{}{
			"prompted": cfg.TelemetryPrompted,
			"enabled":  cfg.TelemetryEnabled,
		})
	})

	mux.HandleFunc("/api/telemetry/set", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var req struct {
			Enabled bool `json:"enabled"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		cfg := LoadConfig()
		cfg.TelemetryPrompted = true
		cfg.TelemetryEnabled = req.Enabled
		_ = SaveConfig(cfg)
		writeJSON(w, map[string]interface{}{
			"status":  "ok",
			"enabled": cfg.TelemetryEnabled,
		})
	})

	mux.HandleFunc("/api/export-report", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var req ExportReportRequest
		json.NewDecoder(r.Body).Decode(&req)

		reportPath, reportContent := writeScanReportFile(req.Games)
		writeJSON(w, map[string]string{
			"path":    reportPath,
			"content": reportContent,
		})
	})

	mux.HandleFunc("/api/resolve-game", func(w http.ResponseWriter, r *http.Request) {
		name := ""
		if r.Method == http.MethodPost {
			var body struct {
				Name string `json:"name"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err == nil {
				name = body.Name
			}
		} else {
			name = r.URL.Query().Get("name")
		}

		res := ResolveGameInfo(name)
		writeJSON(w, res)
	})

	mux.HandleFunc("/api/backup", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var req BackupRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		dest := filepath.Clean(req.DestDir)
		if runtime.GOOS == "windows" && len(dest) == 2 && dest[1] == ':' {
			dest += `\`
		}

		manifest, err := CreateBackup(req.Games, dest, req.CreateSingleZip)
		if err != nil {
			writeJSON(w, map[string]string{"error": err.Error()})
			return
		}

		// Remember save locations in persistent config so they can be reapplied anywhere
		cfg := LoadConfig()
		for _, g := range req.Games {
			cfg.RememberLocation(SavedLocation{
				GameID:             g.ID,
				GameName:           g.Name,
				AppID:              g.AppID,
				OriginalPath:       g.SourcePath,
				TargetRelativePath: g.TargetRelativePath,
				SaveType:           string(g.SaveType),
				StorageType:          g.StorageType,
			})
		}
		_ = SaveConfig(cfg)

		zipPath := filepath.Join(dest, "game_saves_bundle.zip")
		if strings.HasSuffix(strings.ToLower(dest), ".zip") {
			zipPath = dest
		}
		verReport, _ := VerifyZipIntegrity(zipPath)

		writeJSON(w, map[string]interface{}{
			"manifest":     manifest,
			"verification": verReport,
			"zip_path":     zipPath,
		})
	})

	mux.HandleFunc("/api/open-folder", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var req struct {
			Path string `json:"path"`
		}
		json.NewDecoder(r.Body).Decode(&req)
		if err := OpenInExplorer(req.Path); err != nil {
			writeJSON(w, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, map[string]string{"status": "ok"})
	})

	mux.HandleFunc("/api/eject-usb", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var req struct {
			Path string `json:"path"`
		}
		json.NewDecoder(r.Body).Decode(&req)
		msg, err := SafeEjectDrive(req.Path)
		if err != nil {
			writeJSON(w, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, map[string]string{"message": msg})
	})

	mux.HandleFunc("/api/config", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			cfg := LoadConfig()
			writeJSON(w, cfg)
			return
		} else if r.Method == http.MethodPost {
			var cfg AppConfig
			if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			if err := SaveConfig(cfg); err != nil {
				writeJSON(w, map[string]string{"error": err.Error()})
				return
			}
			writeJSON(w, map[string]string{"status": "saved"})
			return
		}
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	})

	mux.HandleFunc("/api/verify-backup", func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Query().Get("path")
		if path == "" {
			http.Error(w, "missing path", http.StatusBadRequest)
			return
		}
		rep, err := VerifyZipIntegrity(path)
		if err != nil {
			writeJSON(w, map[string]interface{}{"valid": false, "error": err.Error()})
			return
		}
		writeJSON(w, rep)
	})

	mux.HandleFunc("/api/inspect", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var req InspectRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		manifest, err := InspectVault(req.SourcePath)
		if err != nil {
			writeJSON(w, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, manifest)
	})

	mux.HandleFunc("/api/restore", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var req RestoreRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		var reports []RestoreReport
		total := len(req.Tasks)
		SetProgress("restore", 0, "Starting restore process...", 0, total, "")
		for i, task := range req.Tasks {
			pct := (float64(i) / float64(total)) * 100.0
			status := fmt.Sprintf("Restoring %s (%d of %d)...", task.Meta.GameName, i+1, total)
			detail := fmt.Sprintf("Target: %s", task.CustomDir)
			if task.CustomDir == "" {
				detail = fmt.Sprintf("Target: %s", task.Meta.SuggestedLinuxPath)
			}
			SetProgress("restore", pct, status, i+1, total, detail)

			rep := RestoreGame(req.SourcePath, task.Meta, task.CustomDir)
			reports = append(reports, rep)
		}
		SetProgress("idle", 100, "Restore complete.", total, total, "")
		writeJSON(w, reports)
	})

	// Static Assets from embedded FS
	subFS, err := fs.Sub(webFiles, "web")
	if err != nil {
		fmt.Printf("Failed to sub embedded web FS: %v\n", err)
		os.Exit(1)
	}
	mux.Handle("/", http.FileServer(http.FS(subFS)))

	server := &http.Server{Handler: securityMiddleware(mux)}

	// On Windows, if user explicitly requested --native, launch raw Win32
	if runtime.GOOS == "windows" && *nativeFlag {
		go func() {
			if err := server.Serve(listener); err != nil && err != http.ErrServerClosed {
				Log("Background server error: %v", err)
			}
		}()
		Log("Starting native Win32 desktop GUI (forced by --native)...")
		runNativeWindowsGUI()
		return
	}

	// Start internal server
	go func() {
		if err := server.Serve(listener); err != nil && err != http.ErrServerClosed {
			Log("Background server error: %v", err)
		}
	}()

	Log("Checkpoint local backend running at: %s", url)
	fmt.Printf("Checkpoint running at: %s\n", url)

	// Launch dedicated native desktop application window
	Log("Launching dedicated native desktop application window...")
	if runEmbeddedAppWindow(url) {
		Log("Application exited normally.")
		return
	}

	Log("Native window runtime missing. Attempting browser fallback...")
	go launchAppWindow(url)

	// Handle graceful shutdown
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	<-stop
	Log("Shutting down Checkpoint.")
}

func writeJSON(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}

func launchAppWindow(url string) {
	if runtime.GOOS == "windows" {
		tempDir := os.Getenv("TEMP")
		if tempDir == "" {
			tempDir = `C:\Temp`
		}
		userDataDir := filepath.Join(tempDir, "save_migrator_profile")

		localApp := os.Getenv("LOCALAPPDATA")
		progFiles := os.Getenv("ProgramFiles")
		progFiles86 := os.Getenv("ProgramFiles(x86)")

		type browserTarget struct {
			path string
		}
		var candidates []browserTarget

		addCandidate := func(dir, subpath string) {
			if dir != "" {
				candidates = append(candidates, browserTarget{path: filepath.Join(dir, subpath)})
			}
		}

		// 1. Edge (Pre-installed on virtually all Windows 10 & 11)
		addCandidate(progFiles86, `Microsoft\Edge\Application\msedge.exe`)
		addCandidate(progFiles, `Microsoft\Edge\Application\msedge.exe`)
		addCandidate(localApp, `Microsoft\Edge\Application\msedge.exe`)

		// 2. Brave (Popular on debloated Windows / gaming rigs)
		addCandidate(localApp, `BraveSoftware\Brave-Browser\Application\brave.exe`)
		addCandidate(progFiles, `BraveSoftware\Brave-Browser\Application\brave.exe`)

		// 3. Chrome
		addCandidate(localApp, `Google\Chrome\Application\chrome.exe`)
		addCandidate(progFiles, `Google\Chrome\Application\chrome.exe`)
		addCandidate(progFiles86, `Google\Chrome\Application\chrome.exe`)

		// 4. Vivaldi, Opera
		addCandidate(localApp, `Vivaldi\Application\vivaldi.exe`)
		addCandidate(localApp, `Programs\Opera\launcher.exe`)

		for _, b := range candidates {
			if _, err := os.Stat(b.path); err == nil {
				cmd := exec.Command(b.path, "--app="+url, "--user-data-dir="+userDataDir, "--window-size=980,820")
				if err := cmd.Start(); err == nil {
					Log("Launched standalone modern app window via %s", b.path)
					return
				}
			}
		}

		// Fallback for stripped Windows with zero Chromium browsers
		Log("No Chromium browser found for app mode. Falling back to native Win32 GUI.")
		runNativeWindowsGUI()
	} else if runtime.GOOS == "linux" {
		tempDir := os.TempDir()
		userDataDir := filepath.Join(tempDir, "save_migrator_profile")

		for _, browser := range []string{"google-chrome", "chromium", "brave", "microsoft-edge"} {
			if path, err := exec.LookPath(browser); err == nil {
				cmd := exec.Command(path, "--app="+url, "--user-data-dir="+userDataDir, "--window-size=960,820")
				if err := cmd.Start(); err == nil {
					return
				}
			}
		}
		exec.Command("xdg-open", url).Start()
	}
}

func runCLIRestore(vaultPath string, auto bool) {
	fmt.Println("=======================================================")
	fmt.Println(" SAVE MIGRATOR - LINUX RESTORER [CLI]")
	fmt.Println("=======================================================")
	fmt.Printf("Inspecting vault: %s\n", vaultPath)

	manifest, err := InspectVault(vaultPath)
	if err != nil {
		fmt.Printf("Error opening vault: %v\n", err)
		return
	}

	fmt.Printf("Loaded vault with %d game save(s), created at %s\n\n", len(manifest.Games), manifest.CreatedAt)

	resolvableCount := 0
	for i, g := range manifest.Games {
		status := fmt.Sprintf("[%s]", g.TargetStatusText)
		target := g.SuggestedLinuxPath
		if target == "" {
			target = "None detected"
		} else {
			resolvableCount++
		}
		fmt.Printf("[%2d] %s (AppID: %s, %s)\n     Status: %s\n     Target: %s\n", i+1, g.GameName, g.AppID, g.StorageType, status, target)
	}

	if len(manifest.OnlineGames) > 0 {
		fmt.Println("\nOnline Games & Anti-Cheat Compatibility:")
		for _, o := range manifest.OnlineGames {
			fmt.Printf(" - %s: %s (%s) [%s]\n", o.Name, o.Status, strings.Join(o.AntiCheats, ", "), o.InstallSource)
		}
		fmt.Println()
	}

	if resolvableCount == 0 {
		fmt.Println("\nNotice: No target game directories or Proton prefixes were auto-detected.")
		fmt.Println("Launch games once on Linux or use the graphical interface to map paths.")
		return
	}

	if !auto {
		fmt.Printf("\nReady to restore %d resolvable game(s). Proceed? [y/N]: ", resolvableCount)
		var response string
		fmt.Scanln(&response)
		if strings.ToLower(strings.TrimSpace(response)) != "y" && strings.ToLower(strings.TrimSpace(response)) != "yes" {
			fmt.Println("Restore cancelled by user.")
			return
		}
	}

	fmt.Println("\nRestoring saves with Steam Cloud Timestamp Protection...")
	successCount := 0
	for _, g := range manifest.Games {
		if g.SuggestedLinuxPath == "" {
			continue
		}
		fmt.Printf(" -> Restoring %s to %s ... ", g.GameName, g.SuggestedLinuxPath)
		report := RestoreGame(vaultPath, g, g.SuggestedLinuxPath)
		if report.Success {
			fmt.Println("SUCCESS")
			successCount++
		} else {
			fmt.Printf("FAILED: %s\n", report.Error)
		}
	}

	fmt.Printf("\nRestore complete: %d of %d games restored successfully.\n", successCount, resolvableCount)
}

func runCLI() {
	fmt.Println("=======================================================")
	fmt.Println(" SAVE MIGRATOR v0.5.9.5 [BETA TEST BUILD]")
	fmt.Println("=======================================================")
	fmt.Println("Notice: Early preview build. Verify saves before wiping.")
	fmt.Println()

	status := CheckSystem()
	fmt.Printf("OS: %s\n", status.Platform)
	if status.FastStartupActive {
		fmt.Println("Warning: Fast Startup is active. You can disable it with: powercfg -h off")
	}
	if status.BitLockerWarning {
		fmt.Printf("Warning: %s\n", status.BitLockerDetails)
	}

	fmt.Println()
	fmt.Println("Scanning game saves...")
	games, err := ScanInstalledGames(nil)
	if err != nil {
		fmt.Printf("Scan error: %v\n", err)
		return
	}
	fmt.Printf("Found %d game save(s):\n", len(games))
	for i, g := range games {
		tier := ""
		if g.ProtonTier != "" {
			tier = " [ProtonDB: " + g.ProtonTier + "]"
		}
		fmt.Printf(" [%d] %s (AppID: %s, %s, %s)%s\n", i+1, g.Name, g.AppID, g.StorageType, formatBytes(g.SizeBytes), tier)
	}

	online := ScanOnlineGames()
	if len(online) > 0 {
		fmt.Println()
		fmt.Println("Online Games (Cloud Saved):")
		for _, o := range online {
			fmt.Printf(" - %s: %s (%s)\n", o.Name, o.Status, strings.Join(o.AntiCheats, ", "))
		}
	}
}

func formatBytes(b int64) string {
	if b < 1024*1024 {
		kb := float64(b) / 1024.0
		return fmt.Sprintf("%.2f KB", kb)
	}
	if b >= 1024*1024*1024 {
		gb := float64(b) / (1024.0 * 1024.0 * 1024.0)
		return fmt.Sprintf("%.2f GB", gb)
	}
	mb := float64(b) / (1024.0 * 1024.0)
	return fmt.Sprintf("%.2f MB", mb)
}

func writeScanReportFile(games []DetectedGame) (string, string) {
	var sb strings.Builder
	sb.WriteString("=== SAVE MIGRATOR SCAN REPORT ===\n")
	sb.WriteString(fmt.Sprintf("Timestamp: %s\n", time.Now().Format("2006-01-02 15:04:05")))
	sb.WriteString(fmt.Sprintf("Total Titles Detected: %d\n", len(games)))
	sb.WriteString("==================================================\n\n")

	for i, g := range games {
		sizeStr := formatBytes(g.SizeBytes)
		sb.WriteString(fmt.Sprintf("[%d] Title: %s\n", i+1, g.Name))
		if g.AppID != "" {
			sb.WriteString(fmt.Sprintf("    AppID: %s\n", g.AppID))
		}
		if g.ProtonTier != "" {
			sb.WriteString(fmt.Sprintf("    ProtonDB: %s\n", strings.ToUpper(g.ProtonTier)))
		}
		sb.WriteString(fmt.Sprintf("    Save Type: %s (%s)\n", g.SaveType, g.StorageType))
		sb.WriteString(fmt.Sprintf("    Location: %s\n", g.SourcePath))
		if g.TargetRelativePath != "" {
			sb.WriteString(fmt.Sprintf("    Target Relative: %s\n", g.TargetRelativePath))
		}
		sb.WriteString(fmt.Sprintf("    Files: %d (%s)\n", g.FileCount, sizeStr))
		if g.IsSteamCloud {
			sb.WriteString("    Steam Cloud: Yes (Managed by official Steam Cloud - Skipped)\n")
		}
		if g.Notes != "" {
			sb.WriteString(fmt.Sprintf("    Notes: %s\n", g.Notes))
		}
		sb.WriteString("\n")
	}

	content := sb.String()
	reportName := "save_migrator_scan_report.txt"

	// 1. Primary path: executable directory or current working directory
	var primaryPath string
	execPath, err := os.Executable()
	if err == nil {
		primaryPath = filepath.Join(filepath.Dir(execPath), reportName)
	} else {
		primaryPath = reportName
	}

	_ = os.WriteFile(primaryPath, []byte(content), 0644)

	// 2. Secondary fallback path: OS Temp directory
	tempPath := filepath.Join(os.TempDir(), reportName)
	_ = os.WriteFile(tempPath, []byte(content), 0644)

	Log("Scan report written to %s and %s", primaryPath, tempPath)
	return primaryPath, content
}

type SavedLocation struct {
	GameID             string `json:"game_id"`
	GameName           string `json:"game_name"`
	AppID              string `json:"appid,omitempty"`
	OriginalPath       string `json:"original_path"`
	TargetRelativePath string `json:"target_relative_path"`
	LinuxResolvedPath  string `json:"linux_resolved_path,omitempty"`
	SaveType           string `json:"save_type,omitempty"`
	StorageType          string `json:"storage_type,omitempty"`
}

type AppConfig struct {
	CustomPaths       []string                 `json:"custom_paths"`
	BackupDest        string                   `json:"backup_dest"`
	SaveLocations     map[string]SavedLocation `json:"save_locations,omitempty"`
	TelemetryPrompted bool                     `json:"telemetry_prompted"`
	TelemetryEnabled  bool                     `json:"telemetry_enabled"`
}

func (c *AppConfig) RememberLocation(loc SavedLocation) {
	if c.SaveLocations == nil {
		c.SaveLocations = make(map[string]SavedLocation)
	}
	if loc.GameID != "" {
		c.SaveLocations[loc.GameID] = loc
	}
	if loc.AppID != "" {
		c.SaveLocations["appid_"+loc.AppID] = loc
	}
	cleanName := strings.ToLower(strings.TrimSpace(loc.GameName))
	if cleanName != "" {
		c.SaveLocations["name_"+cleanName] = loc
	}
}

func (c *AppConfig) GetLocation(gameID, appID, gameName string) (SavedLocation, bool) {
	if c.SaveLocations == nil {
		return SavedLocation{}, false
	}
	if gameID != "" {
		if loc, ok := c.SaveLocations[gameID]; ok {
			return loc, true
		}
	}
	if appID != "" {
		if loc, ok := c.SaveLocations["appid_"+appID]; ok {
			return loc, true
		}
	}
	cleanName := strings.ToLower(strings.TrimSpace(gameName))
	if cleanName != "" {
		if loc, ok := c.SaveLocations["name_"+cleanName]; ok {
			return loc, true
		}
	}
	return SavedLocation{}, false
}

func getConfigPath() string {
	execPath, err := os.Executable()
	if err == nil {
		return filepath.Join(filepath.Dir(execPath), "save_migrator_config.json")
	}
	return "save_migrator_config.json"
}

func LoadConfig() AppConfig {
	cfgPath := getConfigPath()
	data, err := os.ReadFile(cfgPath)
	if err != nil {
		return AppConfig{CustomPaths: []string{}, SaveLocations: make(map[string]SavedLocation)}
	}
	var cfg AppConfig
	_ = json.Unmarshal(data, &cfg)
	if cfg.CustomPaths == nil {
		cfg.CustomPaths = []string{}
	}
	if cfg.SaveLocations == nil {
		cfg.SaveLocations = make(map[string]SavedLocation)
	}
	return cfg
}

func SaveConfig(cfg AppConfig) error {
	cfgPath := getConfigPath()
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(cfgPath, data, 0644)
}

func OpenInExplorer(targetPath string) error {
	cleanPath := filepath.Clean(targetPath)
	if _, err := os.Stat(cleanPath); err != nil {
		return err
	}
	if runtime.GOOS == "windows" {
		cmd := exec.Command("explorer.exe", cleanPath)
		return cmd.Start()
	} else if runtime.GOOS == "linux" {
		cmd := exec.Command("xdg-open", cleanPath)
		return cmd.Start()
	}
	return nil
}

func securityMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin != "" {
			u, err := url.Parse(origin)
			if err != nil || (u.Hostname() != "127.0.0.1" && u.Hostname() != "localhost") {
				http.Error(w, "Forbidden: Cross-origin request not permitted", http.StatusForbidden)
				return
			}
		}
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		next.ServeHTTP(w, r)
	})
}

func SafeEjectDrive(drivePath string) (string, error) {
	if runtime.GOOS == "windows" {
		drive := strings.TrimRight(drivePath, `\/`)
		if len(drive) >= 2 && drive[1] == ':' {
			drive = strings.ToUpper(drive[:2])
		}
		if matched, _ := regexp.MatchString(`^[A-Z]:$`, drive); !matched {
			return "", fmt.Errorf("invalid drive identifier: %s", drivePath)
		}
		psScript := fmt.Sprintf(`$sh = New-Object -ComObject Shell.Application; $drive = $sh.Namespace(17).ParseName('%s'); if ($drive) { $drive.InvokeVerb('Eject'); 'Ejected' } else { 'NotFound' }`, drive)
		cmd := exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-Command", psScript)
		cmd.SysProcAttr = hideWindowAttr()
		out, err := cmd.Output()
		if err != nil {
			return "", fmt.Errorf("failed to eject drive: %w", err)
		}
		if strings.Contains(string(out), "Ejected") {
			return fmt.Sprintf("USB Drive %s safely ejected. You can now unplug it.", drive), nil
		}
		return fmt.Sprintf("Drive %s processed for removal.", drive), nil
	} else if runtime.GOOS == "linux" {
		cmd := exec.Command("sync")
		_ = cmd.Run()
		return "Disk write buffers flushed (sync completed). Safe to unplug.", nil
	}
	return "Drive processed for removal.", nil
}
