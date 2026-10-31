package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

// AppVersion defines the current semantic release version of Checkpoint
const AppVersion = "1.1.2"

// Repository configuration for GitHub Releases API
const (
	githubRepoOwner   = "Cryptorial0"
	githubRepoName    = "Checkpoint"
	githubReleasesURL = "https://api.github.com/repos/Cryptorial0/Checkpoint/releases/latest"
	httpUserAgent     = "Checkpoint-Migrator/1.1.2"
)

// ReleaseAsset describes a downloadable artifact attached to a GitHub release
type ReleaseAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
	Size               int64  `json:"size"`
}

// GitHubReleaseResponse maps the relevant fields returned by the GitHub Releases API
type GitHubReleaseResponse struct {
	TagName     string         `json:"tag_name"`
	Name        string         `json:"name"`
	Body        string         `json:"body"`
	Draft       bool           `json:"draft"`
	Prerelease  bool           `json:"prerelease"`
	PublishedAt time.Time      `json:"published_at"`
	Assets      []ReleaseAsset `json:"assets"`
}

// UpdateInfo contains the parsed update status presented to the user
type UpdateInfo struct {
	CurrentVersion  string `json:"current_version"`
	LatestVersion   string `json:"latest_version"`
	UpdateAvailable bool   `json:"update_available"`
	ReleaseTitle    string `json:"release_title"`
	ReleaseNotes    string `json:"release_notes"`
	DownloadURL     string `json:"download_url"`
	AssetSize       int64  `json:"asset_size"`
	PublishedDate   string `json:"published_date"`
}

// updateCache stores the last check result to avoid exceeding GitHub API rate limits
type updateCache struct {
	sync.Mutex
	lastChecked time.Time
	cachedInfo  *UpdateInfo
}

var updateCheckerCache updateCache

// isNewerVersion compares two semantic version strings (e.g. "1.1.0" vs "1.0.0").
// Returns true only if the latest version is strictly newer than current.
func isNewerVersion(latest, current string) bool {
	latest = strings.TrimPrefix(strings.TrimSpace(latest), "v")
	current = strings.TrimPrefix(strings.TrimSpace(current), "v")

	latestParts := parseVersionSegments(latest)
	currentParts := parseVersionSegments(current)

	for i := 0; i < len(latestParts) && i < len(currentParts); i++ {
		if latestParts[i] > currentParts[i] {
			return true
		}
		if latestParts[i] < currentParts[i] {
			return false
		}
	}
	return len(latestParts) > len(currentParts)
}

// parseVersionSegments breaks down a version string into individual integer components
func parseVersionSegments(v string) []int {
	rawSegments := strings.Split(v, ".")
	segments := make([]int, 0, len(rawSegments))
	for _, segment := range rawSegments {
		cleanSegment := ""
		for _, r := range segment {
			if r >= '0' && r <= '9' {
				cleanSegment += string(r)
			} else {
				break
			}
		}
		if val, err := strconv.Atoi(cleanSegment); err == nil {
			segments = append(segments, val)
		} else {
			segments = append(segments, 0)
		}
	}
	return segments
}

// isRunningAsAppImage detects if the application is running packaged as an AppImage on Linux
func isRunningAsAppImage() (bool, string) {
	if runtime.GOOS != "linux" {
		return false, ""
	}
	appImage := os.Getenv("APPIMAGE")
	if appImage != "" {
		return true, appImage
	}
	execPath, err := os.Executable()
	if err == nil {
		if strings.Contains(execPath, "/.mount_") || strings.HasPrefix(execPath, "/tmp/.mount_") {
			if argv0 := os.Getenv("ARGV0"); argv0 != "" {
				if _, statErr := os.Stat(argv0); statErr == nil {
					return true, argv0
				}
			}
			return true, ""
		}
	}
	return false, ""
}

// findMatchingAsset selects the binary artifact matching the host operating system and architecture
func findMatchingAsset(assets []ReleaseAsset) (ReleaseAsset, error) {
	osName := runtime.GOOS
	arch := runtime.GOARCH

	isAppImg, _ := isRunningAsAppImage()

	for _, asset := range assets {
		name := strings.ToLower(asset.Name)

		if osName == "windows" {
			if strings.HasSuffix(name, ".exe") {
				if arch == "amd64" && (strings.Contains(name, "amd64") || strings.Contains(name, "x64") || name == "checkpoint.exe") {
					return asset, nil
				}
			}
		} else if osName == "linux" {
			if strings.HasSuffix(name, ".exe") || strings.HasSuffix(name, ".zip") || strings.HasSuffix(name, ".txt") {
				continue
			}

			if isAppImg {
				if strings.HasSuffix(name, ".appimage") {
					return asset, nil
				}
			} else {
				if !strings.HasSuffix(name, ".appimage") && (name == "checkpoint" || strings.Contains(name, "linux")) {
					return asset, nil
				}
			}
		}
	}

	// Fallback for Linux if preferred flavor was not matched
	if osName == "linux" {
		for _, asset := range assets {
			name := strings.ToLower(asset.Name)
			if !strings.HasSuffix(name, ".exe") && !strings.HasSuffix(name, ".zip") && !strings.HasSuffix(name, ".txt") {
				if strings.HasSuffix(name, ".appimage") || name == "checkpoint" {
					return asset, nil
				}
			}
		}
	}

	return ReleaseAsset{}, fmt.Errorf("no compatible release asset found for %s/%s", osName, arch)
}

// CheckForUpdates queries the GitHub API for the latest release metadata
func CheckForUpdates(force bool) (*UpdateInfo, error) {
	updateCheckerCache.Lock()
	defer updateCheckerCache.Unlock()

	// Respect rate limits by serving from cache if checked within the past 15 minutes
	if !force && updateCheckerCache.cachedInfo != nil && time.Since(updateCheckerCache.lastChecked) < 15*time.Minute {
		return updateCheckerCache.cachedInfo, nil
	}

	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequest(http.MethodGet, githubReleasesURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", httpUserAgent)
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed connecting to update server: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		// Repository has no public releases yet
		info := &UpdateInfo{
			CurrentVersion:  AppVersion,
			LatestVersion:   AppVersion,
			UpdateAvailable: false,
		}
		updateCheckerCache.cachedInfo = info
		updateCheckerCache.lastChecked = time.Now()
		return info, nil
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("update server responded with status %d", resp.StatusCode)
	}

	var release GitHubReleaseResponse
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, fmt.Errorf("failed parsing release metadata: %w", err)
	}

	tagVersion := strings.TrimPrefix(release.TagName, "v")
	isAvailable := isNewerVersion(tagVersion, AppVersion)

	var downloadURL string
	var assetSize int64
	if isAvailable {
		if matched, err := findMatchingAsset(release.Assets); err == nil {
			downloadURL = matched.BrowserDownloadURL
			assetSize = matched.Size
		}
	}

	info := &UpdateInfo{
		CurrentVersion:  AppVersion,
		LatestVersion:   tagVersion,
		UpdateAvailable: isAvailable,
		ReleaseTitle:    release.Name,
		ReleaseNotes:    release.Body,
		DownloadURL:     downloadURL,
		AssetSize:       assetSize,
		PublishedDate:   release.PublishedAt.Format("2006-01-02"),
	}

	updateCheckerCache.cachedInfo = info
	updateCheckerCache.lastChecked = time.Now()
	return info, nil
}

// CleanupOldBinary removes leftover backup binaries from previous updates (Windows .old files)
func CleanupOldBinary() {
	var execPath string
	isAppImg, appImgPath := isRunningAsAppImage()
	if isAppImg && appImgPath != "" {
		execPath = appImgPath
	} else {
		var err error
		execPath, err = os.Executable()
		if err != nil {
			return
		}
		execPath, err = filepath.EvalSymlinks(execPath)
		if err != nil {
			return
		}
	}
	oldPath := execPath + ".old"
	if _, err := os.Stat(oldPath); err == nil {
		_ = os.Remove(oldPath)
	}
}

// ApplyUpdate downloads the new release binary and replaces the running executable on disk
func ApplyUpdate(downloadURL string, progressCallback func(percent float64, message string)) error {
	if downloadURL == "" {
		return errors.New("empty download URL")
	}

	var currentExec string
	isAppImg, appImgPath := isRunningAsAppImage()
	if isAppImg {
		if appImgPath == "" {
			return errors.New("cannot auto-update AppImage: APPIMAGE environment variable is missing, please download the latest AppImage from GitHub Releases")
		}
		currentExec = appImgPath
	} else {
		exec, err := os.Executable()
		if err != nil {
			return fmt.Errorf("unable to determine current executable path: %w", err)
		}
		exec, err = filepath.EvalSymlinks(exec)
		if err != nil {
			return fmt.Errorf("unable to resolve executable path: %w", err)
		}
		currentExec = exec
	}

	// Guard against read-only squashfs mount paths if APPIMAGE was missing
	if runtime.GOOS == "linux" && (strings.Contains(currentExec, "/.mount_") || strings.HasPrefix(currentExec, "/tmp/.mount_")) {
		return errors.New("cannot write update into read-only AppImage mount path; please download the latest release")
	}

	execDir := filepath.Dir(currentExec)

	// Try creating temp file in execDir first (for atomic rename). Fall back to os.TempDir() if execDir is read-only.
	tempFile, err := os.CreateTemp(execDir, "checkpoint_update_*.tmp")
	if err != nil {
		tempFile, err = os.CreateTemp(os.TempDir(), "checkpoint_update_*.tmp")
		if err != nil {
			return fmt.Errorf("failed creating temporary update file: %w", err)
		}
	}
	tempPath := tempFile.Name()
	defer func() {
		// Clean up temporary file if it was not renamed into place
		if _, err := os.Stat(tempPath); err == nil {
			_ = os.Remove(tempPath)
		}
	}()

	client := &http.Client{Timeout: 5 * time.Minute}
	req, err := http.NewRequest(http.MethodGet, downloadURL, nil)
	if err != nil {
		tempFile.Close()
		return err
	}
	req.Header.Set("User-Agent", httpUserAgent)

	resp, err := client.Do(req)
	if err != nil {
		tempFile.Close()
		return fmt.Errorf("failed downloading update: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		tempFile.Close()
		return fmt.Errorf("download server returned HTTP %d", resp.StatusCode)
	}

	totalBytes := resp.ContentLength
	var downloadedBytes int64
	buffer := make([]byte, 64*1024)

	for {
		n, readErr := resp.Body.Read(buffer)
		if n > 0 {
			if _, writeErr := tempFile.Write(buffer[:n]); writeErr != nil {
				tempFile.Close()
				return fmt.Errorf("failed writing update payload: %w", writeErr)
			}
			downloadedBytes += int64(n)
			if totalBytes > 0 && progressCallback != nil {
				pct := (float64(downloadedBytes) / float64(totalBytes)) * 100.0
				progressCallback(pct, fmt.Sprintf("Downloading update: %.1f MB / %.1f MB", float64(downloadedBytes)/(1024*1024), float64(totalBytes)/(1024*1024)))
			}
		}
		if readErr != nil {
			if readErr == io.EOF {
				break
			}
			tempFile.Close()
			return fmt.Errorf("network error during update download: %w", readErr)
		}
	}
	tempFile.Close()

	// Ensure the new binary has execute permissions on Linux / POSIX systems
	if runtime.GOOS != "windows" {
		if err := os.Chmod(tempPath, 0755); err != nil {
			return fmt.Errorf("failed setting executable permissions: %w", err)
		}
	}

	if progressCallback != nil {
		progressCallback(95.0, "Applying updated binary...")
	}

	if err := replaceExecutable(tempPath, currentExec); err != nil {
		return err
	}

	if progressCallback != nil {
		progressCallback(100.0, "Update installed successfully.")
	}

	return nil
}

// replaceExecutable swaps the current running binary on disk with the new binary payload
func replaceExecutable(tempPath, targetPath string) error {
	if runtime.GOOS == "windows" {
		// Windows locks running binaries against write access, but permits renaming open files.
		// We rename the running executable to .old, then place the new binary at the original path.
		oldPath := targetPath + ".old"
		_ = os.Remove(oldPath)

		if err := os.Rename(targetPath, oldPath); err != nil {
			return fmt.Errorf("failed renaming current executable: %w", err)
		}

		if err := os.Rename(tempPath, targetPath); err != nil {
			_ = os.Rename(oldPath, targetPath)
			return fmt.Errorf("failed placing new binary in position: %w", err)
		}
		return nil
	}

	// On Linux / POSIX:
	// 1. Try atomic rename first (instant if on the same filesystem)
	if err := os.Rename(tempPath, targetPath); err == nil {
		return nil
	}

	// 2. If atomic rename failed (e.g. EXDEV across filesystems), copy to target.
	// Unlink target first so open running process does not trigger ETXTBSY.
	backupPath := targetPath + ".old"
	_ = os.Remove(backupPath)
	if err := os.Rename(targetPath, backupPath); err != nil {
		_ = os.Remove(targetPath)
	}

	in, err := os.Open(tempPath)
	if err != nil {
		_ = os.Rename(backupPath, targetPath)
		return fmt.Errorf("failed opening downloaded update payload: %w", err)
	}
	defer in.Close()

	out, err := os.OpenFile(targetPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0755)
	if err != nil {
		_ = os.Rename(backupPath, targetPath)
		return fmt.Errorf("failed creating target binary: %w", err)
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		_ = os.Rename(backupPath, targetPath)
		return fmt.Errorf("failed copying update binary into place: %w", err)
	}
	out.Close()
	_ = os.Remove(backupPath)
	return nil
}
