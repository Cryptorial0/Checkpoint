package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRockstarGamesRDR2Alignment(t *testing.T) {
	tempPrefix := t.TempDir()
	driveC := filepath.Join(tempPrefix, "pfx", "drive_c")

	steamuserDocs := filepath.Join(driveC, "users", "steamuser", "Documents", "Rockstar Games", "Red Dead Redemption 2")
	sourceProfile := filepath.Join(steamuserDocs, "Profiles", "WIN_4A6B7C8D")
	if err := os.MkdirAll(sourceProfile, 0755); err != nil {
		t.Fatalf("Failed to create source profile dir: %v", err)
	}

	// Create dummy save files in source profile
	saveContent := []byte("RDR2_SAVE_TEST_CONTENT_12345")
	if err := os.WriteFile(filepath.Join(sourceProfile, "SRDR30000"), saveContent, 0644); err != nil {
		t.Fatalf("Failed to write save file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(sourceProfile, "SRDR30015"), saveContent, 0644); err != nil {
		t.Fatalf("Failed to write autosave file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(sourceProfile, "Player"), []byte("PLAYER_DATA"), 0644); err != nil {
		t.Fatalf("Failed to write player data: %v", err)
	}

	// Pre-create an active emulator directory as would be generated when user launches RDR2 repack on Linux
	goldbergEmuDir := filepath.Join(driveC, "users", "steamuser", "AppData", "Roaming", "Goldberg SocialClub Emu Saves", "RDR2", "0F74F4C4")
	if err := os.MkdirAll(goldbergEmuDir, 0755); err != nil {
		t.Fatalf("Failed to create mock goldberg dir: %v", err)
	}

	// Pre-create another user directory in the prefix (non-steamuser)
	gamerDocs := filepath.Join(driveC, "users", "gamer", "Documents", "Rockstar Games", "Red Dead Redemption 2", "Profiles", "ACTIVE_9999")
	if err := os.MkdirAll(gamerDocs, 0755); err != nil {
		t.Fatalf("Failed to create mock gamer dir: %v", err)
	}

	// Run alignment
	HandleRockstarGamesAlignment(steamuserDocs, "1174180")

	// 1. Verify Goldberg SocialClub Emu received the saves
	goldbergSave := filepath.Join(goldbergEmuDir, "SRDR30000")
	if data, err := os.ReadFile(goldbergSave); err != nil || string(data) != string(saveContent) {
		t.Errorf("Expected Goldberg emu to receive SRDR30000, err: %v", err)
	}

	// 2. Verify EMPRESS / .1911 received the saves
	empressSave := filepath.Join(driveC, "users", "steamuser", "AppData", "Roaming", ".1911", "Red Dead Redemption 2", "profile", "SRDR30000")
	if data, err := os.ReadFile(empressSave); err != nil || string(data) != string(saveContent) {
		t.Errorf("Expected EMPRESS /.1911 to receive SRDR30000, err: %v", err)
	}

	// 3. Verify non-steamuser profile folder received the saves
	gamerSave := filepath.Join(gamerDocs, "SRDR30000")
	if data, err := os.ReadFile(gamerSave); err != nil || string(data) != string(saveContent) {
		t.Errorf("Expected gamer user dir to receive SRDR30000, err: %v", err)
	}

	// 4. Verify common emulator profile IDs exist under steamuser Documents
	defaultEmuProfileSave := filepath.Join(steamuserDocs, "Profiles", "0F74F4C4", "SRDR30000")
	if data, err := os.ReadFile(defaultEmuProfileSave); err != nil || string(data) != string(saveContent) {
		t.Errorf("Expected default emu profile 0F74F4C4 to receive SRDR30000, err: %v", err)
	}
}

func TestRockstarGamesGTAVAlignment(t *testing.T) {
	tempPrefix := t.TempDir()
	driveC := filepath.Join(tempPrefix, "drive_c")

	steamuserDocs := filepath.Join(driveC, "users", "steamuser", "Documents", "Rockstar Games", "GTAV Enhanced")
	sourceProfile := filepath.Join(steamuserDocs, "Profiles", "GTA_PROFILE_1")
	if err := os.MkdirAll(sourceProfile, 0755); err != nil {
		t.Fatalf("Failed to create GTA source profile: %v", err)
	}

	gtaSave := []byte("GTAV_SAVE_DATA_BYTES")
	if err := os.WriteFile(filepath.Join(sourceProfile, "SGTA50000"), gtaSave, 0644); err != nil {
		t.Fatalf("Failed to write GTA save: %v", err)
	}

	HandleRockstarGamesAlignment(steamuserDocs, "3240220")

	// Verify Goldberg GTA V directory received the save
	goldbergGTAVSave := filepath.Join(driveC, "users", "steamuser", "AppData", "Roaming", "Goldberg SocialClub Emu Saves", "GTA V", "0F74F4C4", "SGTA50000")
	if data, err := os.ReadFile(goldbergGTAVSave); err != nil || string(data) != string(gtaSave) {
		t.Errorf("Expected Goldberg GTA V to receive SGTA50000, err: %v", err)
	}
}

func TestScanGoldbergSocialClub(t *testing.T) {
	tempDir := t.TempDir()
	rdr2Dir := filepath.Join(tempDir, "RDR2", "0F74F4C4")
	gtavDir := filepath.Join(tempDir, "GTA V", "0F74F4C4")
	os.MkdirAll(rdr2Dir, 0755)
	os.MkdirAll(gtavDir, 0755)

	os.WriteFile(filepath.Join(rdr2Dir, "SRDR30000"), []byte("sample_rdr2_save_data_long_enough"), 0644)
	os.WriteFile(filepath.Join(gtavDir, "SGTA50000"), []byte("sample_gtav_save_data_long_enough"), 0644)

	var games []DetectedGame
	seen := make(map[string]bool)

	scanGoldbergSocialClubSubdirs(tempDir, &games, seen)

	if len(games) != 2 {
		t.Fatalf("Expected 2 games detected from Goldberg SocialClub, got %d", len(games))
	}

	foundRDR2, foundGTAV := false, false
	for _, g := range games {
		if g.AppID == "1174180" {
			foundRDR2 = true
		}
		if g.AppID == "271590" {
			foundGTAV = true
		}
	}

	if !foundRDR2 || !foundGTAV {
		t.Errorf("Expected both RDR2 and GTAV detected, got: %+v", games)
	}
}
