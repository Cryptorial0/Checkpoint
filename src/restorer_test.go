package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestCheckLocalSaveConflict(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "checkpoint_conflict_test_*")
	if err != nil {
		t.Fatalf("Failed creating temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Scenario 1: Target directory does not exist or has no files
	hasConflict, _, _, _ := checkLocalSaveConflict(filepath.Join(tempDir, "non_existent"), "2026-10-25T12:00:00Z")
	if hasConflict {
		t.Errorf("Expected no conflict for non-existent target directory")
	}

	targetDir := filepath.Join(tempDir, "EldenRingSaves")
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		t.Fatalf("Failed creating target dir: %v", err)
	}

	hasConflict, _, _, _ = checkLocalSaveConflict(targetDir, "2026-10-25T12:00:00Z")
	if hasConflict {
		t.Errorf("Expected no conflict for empty target directory")
	}

	// Scenario 2: Target directory has files that are NEWER than the archive backup date
	saveFile := filepath.Join(targetDir, "ER0000.sl2")
	if err := os.WriteFile(saveFile, []byte("newer_local_save_data"), 0644); err != nil {
		t.Fatalf("Failed creating save file: %v", err)
	}

	// Set local file mod time to 2 days after the backup date
	localPlayedTime := time.Date(2026, 10, 29, 18, 0, 0, 0, time.UTC)
	_ = os.Chtimes(saveFile, localPlayedTime, localPlayedTime)

	archiveBackupDate := "2026-10-27T12:00:00Z"
	hasConflict, localStr, archiveStr, notice := checkLocalSaveConflict(targetDir, archiveBackupDate)
	if !hasConflict {
		t.Errorf("Expected conflict when local save is newer than archive")
	}
	if !strings.Contains(notice, "Local save is newer") {
		t.Errorf("Expected notice to describe newer local save, got: %s", notice)
	}
	if localStr == "" || archiveStr == "" {
		t.Errorf("Expected formatted date strings, got local=%q archive=%q", localStr, archiveStr)
	}

	// Scenario 3: Target directory has files that are OLDER than the archive backup date
	olderPlayedTime := time.Date(2026, 10, 20, 10, 0, 0, 0, time.UTC)
	_ = os.Chtimes(saveFile, olderPlayedTime, olderPlayedTime)

	hasConflict, _, _, _ = checkLocalSaveConflict(targetDir, archiveBackupDate)
	if hasConflict {
		t.Errorf("Expected no conflict when local save is older than archive")
	}
}

func TestRestoreGameConflictResolution(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "checkpoint_restore_conflict_*")
	if err != nil {
		t.Fatalf("Failed creating temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	targetDir := filepath.Join(tempDir, "GameDestination")
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		t.Fatalf("Failed creating target dir: %v", err)
	}

	localSaveFile := filepath.Join(targetDir, "save.dat")
	if err := os.WriteFile(localSaveFile, []byte("local_progress_v2"), 0644); err != nil {
		t.Fatalf("Failed writing initial save: %v", err)
	}

	meta := GameSaveMeta{
		ID:                 "test_game_123",
		GameName:           "Test RPG",
		TargetRelativePath: "users/steamuser/AppData/Local/TestRPG",
		BackupDate:         "2026-10-20T12:00:00Z",
	}

	// 1. Conflict Resolution: "skip" should preserve existing files untouched
	report := RestoreGame(tempDir, meta, targetDir, "skip")
	if !report.Success || !report.Skipped {
		t.Errorf("Expected report to succeed with Skipped=true, got %+v", report)
	}
	data, _ := os.ReadFile(localSaveFile)
	if string(data) != "local_progress_v2" {
		t.Errorf("Expected local save to remain untouched on skip, got: %s", string(data))
	}

	// 2. Conflict Resolution: "backup_local" should create a safety backup before continuing
	_ = RestoreGame(tempDir, meta, targetDir, "backup_local")
	// Verify that a backup directory was created
	entries, _ := os.ReadDir(tempDir)
	foundBackup := false
	for _, entry := range entries {
		if strings.Contains(entry.Name(), "GameDestination.checkpoint_bak_") {
			foundBackup = true
			break
		}
	}
	if !foundBackup {
		t.Errorf("Expected backup directory matching GameDestination.checkpoint_bak_* to be created")
	}
}
