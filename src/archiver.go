package main

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type VaultManifest struct {
	Version     string         `json:"version"`
	CreatedAt   string         `json:"created_at"`
	OS          string         `json:"source_os"`
	Games       []GameSaveMeta `json:"games"`
	OnlineGames []OnlineGame   `json:"online_games,omitempty"`
}

type GameSaveMeta struct {
	ID                 string            `json:"id"`
	GameName           string            `json:"game_name"`
	AppID              string            `json:"appid"`
	SaveType           SaveType          `json:"save_type"`
	StorageType          string            `json:"storage_type"`
	TargetRelativePath string            `json:"target_relative_path"`
	OriginalSourcePath string            `json:"original_source_path,omitempty"`
	SuggestedLinuxPath string            `json:"suggested_linux_path,omitempty"`
	BackupDate         string            `json:"backup_date"`
	FileCount          int               `json:"file_count"`
	SizeBytes          int64             `json:"size_bytes"`
	IconURL            string            `json:"icon_url,omitempty"`
	ProtonTier         string            `json:"proton_tier,omitempty"`
	TargetStatus       string            `json:"target_status,omitempty"`
	TargetStatusText   string            `json:"target_status_text,omitempty"`
	SelectedLauncher   string            `json:"selected_launcher,omitempty"`
	LauncherPaths      map[string]string `json:"launcher_paths,omitempty"`
	LauncherStatuses   map[string]string `json:"launcher_statuses,omitempty"`
}

func CreateBackup(games []DetectedGame, destDir string, createSingleZip bool) (*VaultManifest, error) {
	return CreateBackupWithOptions(games, destDir, createSingleZip, false)
}

func CreateBackupWithOptions(games []DetectedGame, destDir string, createSingleZip bool, optimizeRPG bool) (*VaultManifest, error) {
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create destination directory: %w", err)
	}

	manifest := &VaultManifest{
		Version:   "1.0",
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
		OS:        "windows",
		Games:     []GameSaveMeta{},
	}

	// Stage inside an isolated temp directory so no loose folders are dropped in the destination
	SetProgress("backup", 5, "Preparing backup vault staging...", 0, len(games)+2, "")
	stageDir, err := os.MkdirTemp("", "save_migrator_vault_*")
	if err != nil {
		return nil, fmt.Errorf("failed to create staging directory: %w", err)
	}
	defer os.RemoveAll(stageDir)

	for i, g := range games {
		pct := 5.0 + (float64(i)/float64(len(games)))*75.0
		status := fmt.Sprintf("Compressing %s (%d of %d)...", g.Name, i+1, len(games))
		detail := fmt.Sprintf("%d files, %.2f MB", g.FileCount, float64(g.SizeBytes)/(1024*1024))
		SetProgress("backup", pct, status, i+1, len(games), detail)

		gameFolder := filepath.Join(stageDir, g.ID)
		if err := os.MkdirAll(gameFolder, 0755); err != nil {
			return nil, err
		}

		meta := GameSaveMeta{
			ID:                 g.ID,
			GameName:           g.Name,
			AppID:              g.AppID,
			SaveType:           g.SaveType,
			StorageType:          g.StorageType,
			TargetRelativePath: g.TargetRelativePath,
			OriginalSourcePath: g.SourcePath,
			SuggestedLinuxPath: ComputeSuggestedLinuxPath(g),
			BackupDate:         manifest.CreatedAt,
			FileCount:          g.FileCount,
			SizeBytes:          g.SizeBytes,
		}

		// Write meta.json
		metaBytes, err := json.MarshalIndent(meta, "", "  ")
		if err != nil {
			return nil, err
		}
		if err := os.WriteFile(filepath.Join(gameFolder, "meta.json"), metaBytes, 0644); err != nil {
			return nil, err
		}

		// Compress files into data.tar.gz with optional RPG pruning
		tarPath := filepath.Join(gameFolder, "data.tar.gz")
		var excluded map[string]bool
		if optimizeRPG {
			excluded, _ = BuildRPGExcludedPaths(g.SourcePath, g.Name, g.TargetRelativePath)
		}
		if err := createTarGzFiltered(g.SourcePath, tarPath, excluded); err != nil {
			return nil, fmt.Errorf("failed archiving %s: %w", g.Name, err)
		}

		manifest.Games = append(manifest.Games, meta)
	}

	// Master index.json inside staging
	idxBytes, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(filepath.Join(stageDir, "index.json"), idxBytes, 0644); err != nil {
		return nil, err
	}

	// locations.json storing full source paths and suggested Linux targets
	locations := make(map[string]map[string]string)
	for _, g := range games {
		locations[g.ID] = map[string]string{
			"game_name":            g.Name,
			"appid":                g.AppID,
			"save_type":            string(g.SaveType),
			"storage_type":           g.StorageType,
			"original_source_path": g.SourcePath,
			"target_relative_path": g.TargetRelativePath,
			"suggested_linux_path": ComputeSuggestedLinuxPath(g),
		}
	}
	locBytes, _ := json.MarshalIndent(locations, "", "  ")
	_ = os.WriteFile(filepath.Join(stageDir, "locations.json"), locBytes, 0644)

	// Package staging directly into a single zip in destDir
	SetProgress("backup", 80, fmt.Sprintf("Packaging %d game saves into archive bundle...", len(games)), len(games), len(games), "Creating game_saves_bundle.zip")
	zipPath := filepath.Join(destDir, "game_saves_bundle.zip")
	if strings.HasSuffix(strings.ToLower(destDir), ".zip") {
		zipPath = destDir
	}
	if err := zipDirectory(stageDir, zipPath, ""); err != nil {
		return nil, fmt.Errorf("failed to create zip bundle: %w", err)
	}

	// Immediate post-backup self-test
	rep, err := VerifyZipIntegrity(zipPath)
	if err != nil {
		Log("Warning: zip integrity check failed: %v", err)
	} else {
		Log("Backup verified cleanly: %d games, %d entries, %d bytes uncompressed", rep.GameCount, rep.EntryCount, rep.TotalBytes)
		ClearProgress(fmt.Sprintf("Backup complete. %d games verified (100%% CRC32 valid).", rep.GameCount))
	}

	return manifest, nil
}

type ZipVerificationReport struct {
	Valid      bool   `json:"valid"`
	GameCount  int    `json:"game_count"`
	EntryCount int    `json:"entry_count"`
	TotalBytes int64  `json:"total_bytes"`
	ZipSize    int64  `json:"zip_size"`
	Error      string `json:"error,omitempty"`
}

func VerifyZipIntegrity(zipPath string) (*ZipVerificationReport, error) {
	fi, err := os.Stat(zipPath)
	if err != nil {
		return &ZipVerificationReport{Valid: false, Error: err.Error()}, err
	}

	zr, err := zip.OpenReader(zipPath)
	if err != nil {
		return &ZipVerificationReport{Valid: false, Error: "corrupt zip file: " + err.Error()}, err
	}
	defer zr.Close()

	var totalBytes int64
	entryCount := 0
	hasIndex := false

	// Test each file's decompression and CRC32 checksums
	totalEntries := len(zr.File)
	buf := make([]byte, 32*1024)
	for _, f := range zr.File {
		entryCount++
		pct := 82.0 + (float64(entryCount)/float64(totalEntries))*18.0
		SetProgress("verify", pct, fmt.Sprintf("Verifying CRC32 checksums (%d of %d files)...", entryCount, totalEntries), entryCount, totalEntries, f.Name)
		if f.Name == "index.json" {
			hasIndex = true
		}
		rc, err := f.Open()
		if err != nil {
			return &ZipVerificationReport{Valid: false, Error: fmt.Sprintf("failed to read %s: %v", f.Name, err)}, err
		}
		n, err := io.CopyBuffer(io.Discard, rc, buf)
		rc.Close()
		if err != nil {
			return &ZipVerificationReport{Valid: false, Error: fmt.Sprintf("CRC error in %s: %v", f.Name, err)}, err
		}
		totalBytes += n
	}

	if !hasIndex {
		return &ZipVerificationReport{Valid: false, Error: "missing index.json manifest"}, fmt.Errorf("missing index.json")
	}

	manifest, _ := InspectVault(zipPath)
	gameCount := 0
	if manifest != nil {
		gameCount = len(manifest.Games)
	}

	return &ZipVerificationReport{
		Valid:      true,
		GameCount:  gameCount,
		EntryCount: entryCount,
		TotalBytes: totalBytes,
		ZipSize:    fi.Size(),
	}, nil
}

func createTarGz(sourceDir, tarGzPath string) error {
	return createTarGzFiltered(sourceDir, tarGzPath, nil)
}

func createTarGzFiltered(sourceDir, tarGzPath string, excluded map[string]bool) error {
	tarFile, err := os.Create(tarGzPath)
	if err != nil {
		return err
	}
	defer tarFile.Close()

	gw := gzip.NewWriter(tarFile)
	defer gw.Close()

	tw := tar.NewWriter(gw)
	defer tw.Close()

	return filepath.Walk(sourceDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			// Skip unreadable files or permission errors without failing the archive
			return nil
		}

		if info.IsDir() {
			low := strings.ToLower(info.Name())
			if low == "cache" || low == "webcache" || low == "logs" || low == "crashdumps" || low == "dumps" || low == "temp" {
				return filepath.SkipDir
			}
		}

		relPath, err := filepath.Rel(sourceDir, path)
		if err != nil {
			return nil
		}
		if relPath == "." {
			return nil
		}

		slashRel := filepath.ToSlash(relPath)
		if excluded != nil && excluded[slashRel] {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		header, err := tar.FileInfoHeader(info, info.Name())
		if err != nil {
			return nil
		}
		header.Name = filepath.ToSlash(relPath)

		if err := tw.WriteHeader(header); err != nil {
			return nil
		}

		if !info.IsDir() {
			file, err := os.Open(path)
			if err != nil {
				return nil
			}
			defer file.Close()
			_, _ = io.Copy(tw, file)
		}
		return nil
	})
}

func zipDirectory(sourceDir, zipFilePath, skipFilename string) error {
	zipFile, err := os.Create(zipFilePath)
	if err != nil {
		return err
	}
	defer zipFile.Close()

	w := zip.NewWriter(zipFile)
	defer w.Close()

	return filepath.Walk(sourceDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if skipFilename != "" && filepath.Base(path) == skipFilename {
			return nil
		}
		relPath, err := filepath.Rel(sourceDir, path)
		if err != nil || relPath == "." {
			return nil
		}

		header, err := zip.FileInfoHeader(info)
		if err != nil {
			return err
		}
		header.Name = filepath.ToSlash(relPath)
		if info.IsDir() {
			header.Name += "/"
		} else {
			header.Method = zip.Deflate
		}

		writer, err := w.CreateHeader(header)
		if err != nil {
			return err
		}

		if !info.IsDir() {
			file, err := os.Open(path)
			if err != nil {
				return err
			}
			defer file.Close()
			_, err = io.Copy(writer, file)
			return err
		}
		return nil
	})
}
