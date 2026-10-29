package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSmartRPGOptimizer(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "rpg_optimizer_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	storyDir := filepath.Join(tmpDir, "PlayerProfiles", "Public", "Savegames", "Story")
	os.MkdirAll(storyDir, 0755)

	// Create 2 manual saves
	os.MkdirAll(filepath.Join(storyDir, "Tav_Manual_Act1_Camp"), 0755)
	os.WriteFile(filepath.Join(storyDir, "Tav_Manual_Act1_Camp", "save.lsv"), []byte("manual_save_1"), 0644)

	os.MkdirAll(filepath.Join(storyDir, "Tav_Manual_Act2_Towers"), 0755)
	os.WriteFile(filepath.Join(storyDir, "Tav_Manual_Act2_Towers", "save.lsv"), []byte("manual_save_2"), 0644)

	// Create 10 quicksaves with incrementing mod times
	now := time.Now()
	for i := 1; i <= 10; i++ {
		name := filepath.Join(storyDir, "Quicksave_"+string(rune('A'+i-1)))
		os.MkdirAll(name, 0755)
		lsv := filepath.Join(name, "save.lsv")
		os.WriteFile(lsv, []byte("quicksave_content"), 0644)
		modTime := now.Add(time.Duration(i) * time.Hour)
		_ = os.Chtimes(name, modTime, modTime)
	}

	// Create 6 autosaves with incrementing mod times
	for i := 1; i <= 6; i++ {
		name := filepath.Join(storyDir, "AutoSave_"+string(rune('A'+i-1)))
		os.MkdirAll(name, 0755)
		lsv := filepath.Join(name, "save.lsv")
		os.WriteFile(lsv, []byte("autosave_content"), 0644)
		modTime := now.Add(time.Duration(i) * time.Hour)
		_ = os.Chtimes(name, modTime, modTime)
	}

	excluded, savedBytes := BuildRPGExcludedPaths(tmpDir, "Baldur's Gate 3", "users/steamuser/AppData/Local/Larian Studios/Baldur's Gate 3")

	if savedBytes <= 0 {
		t.Errorf("Expected savedBytes > 0, got %d", savedBytes)
	}

	// 10 quicksaves - 5 kept = 5 excluded
	// 6 autosaves - 3 kept = 3 excluded
	// Total excluded should be 8 directories
	if len(excluded) != 8 {
		t.Errorf("Expected 8 excluded save directories, got %d", len(excluded))
	}

	// Verify manual saves are NOT excluded
	for p := range excluded {
		if strings.Contains(p, "Manual") {
			t.Errorf("Manual save path %s was incorrectly marked for exclusion", p)
		}
	}

	// Test Starfield
	sfDir := filepath.Join(tmpDir, "StarfieldSaves")
	os.MkdirAll(sfDir, 0755)
	for i := 1; i <= 8; i++ {
		f := filepath.Join(sfDir, "quicksave"+string(rune('0'+i))+".sfs")
		os.WriteFile(f, []byte("starfield_quick"), 0644)
		modTime := now.Add(time.Duration(i) * time.Hour)
		_ = os.Chtimes(f, modTime, modTime)
	}
	os.WriteFile(filepath.Join(sfDir, "Save1_A1B2_Manual.sfs"), []byte("starfield_manual"), 0644)

	sfExcluded, _ := BuildRPGExcludedPaths(sfDir, "Starfield", "users/steamuser/Documents/My Games/Starfield/Saves")
	// 8 quicksaves - 5 kept = 3 excluded
	if len(sfExcluded) != 3 {
		t.Errorf("Expected 3 excluded Starfield quicksaves, got %d", len(sfExcluded))
	}
	if sfExcluded["Save1_A1B2_Manual.sfs"] {
		t.Errorf("Starfield manual save was incorrectly excluded")
	}

	// Test Cyberpunk 2077
	cpDir := filepath.Join(tmpDir, "CyberpunkSaves")
	os.MkdirAll(cpDir, 0755)
	for i := 1; i <= 10; i++ {
		sub := filepath.Join(cpDir, "AutoSave-"+string(rune('0'+i)))
		os.MkdirAll(sub, 0755)
		os.WriteFile(filepath.Join(sub, "sav.dat"), []byte("cp_auto"), 0644)
		modTime := now.Add(time.Duration(i) * time.Hour)
		_ = os.Chtimes(sub, modTime, modTime)
	}
	subManual := filepath.Join(cpDir, "ManualSave-0")
	os.MkdirAll(subManual, 0755)
	os.WriteFile(filepath.Join(subManual, "sav.dat"), []byte("cp_manual"), 0644)

	cpExcluded, _ := BuildRPGExcludedPaths(cpDir, "Cyberpunk 2077", "users/steamuser/Saved Games/CD Projekt Red/Cyberpunk 2077")
	// 10 autosaves - 3 kept = 7 excluded
	if len(cpExcluded) != 7 {
		t.Errorf("Expected 7 excluded Cyberpunk autosaves, got %d", len(cpExcluded))
	}
	if cpExcluded["ManualSave-0"] {
		t.Errorf("Cyberpunk manual save was incorrectly excluded")
	}
}
