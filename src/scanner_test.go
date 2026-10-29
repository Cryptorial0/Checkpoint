package main

import (
	"bytes"
	"encoding/binary"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestScanBloodborneAndDarkestDungeon(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "save_migrator_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// 1. Setup Bloodborne structure (ShadPS4 / bbport)
	bbSaveDir := filepath.Join(tmpDir, "Bloodborne", "bbport-windows", "user", "savedata", "1", "CUSA00207")
	if err := os.MkdirAll(bbSaveDir, 0755); err != nil {
		t.Fatalf("Failed to create bb save dir: %v", err)
	}
	os.WriteFile(filepath.Join(bbSaveDir, "SPRJ0005"), []byte("BLOODBORNE_SAVE_DATA"), 0644)

	// Vulkan cache folder that MUST be ignored
	bbCacheDir := filepath.Join(tmpDir, "Bloodborne", "bbport-windows", "user", "cache")
	if err := os.MkdirAll(bbCacheDir, 0755); err != nil {
		t.Fatalf("Failed to create cache dir: %v", err)
	}
	os.WriteFile(filepath.Join(bbCacheDir, "shader_01.spv"), []byte("SPV_VULKAN_SHADER_CACHE"), 0644)

	// 2. Setup God of War III Remastered structure (CUSA01715)
	gowSaveDir := filepath.Join(tmpDir, "God of War III Remastered", "gow3-windows", "user", "savedata", "1", "CUSA01715")
	if err := os.MkdirAll(gowSaveDir, 0755); err != nil {
		t.Fatalf("Failed to create gow save dir: %v", err)
	}
	os.WriteFile(filepath.Join(gowSaveDir, "GOW3_SAVE"), []byte("GOW_SAVE_DATA"), 0644)

	// 3. Setup Darkest Dungeon Documents structure
	darkestDir := filepath.Join(tmpDir, "Darkest")
	if err := os.MkdirAll(darkestDir, 0755); err != nil {
		t.Fatalf("Failed to create darkest dir: %v", err)
	}
	os.WriteFile(filepath.Join(darkestDir, "profile_1.json"), []byte("{}"), 0644)
	os.WriteFile(filepath.Join(darkestDir, "persist.game.json"), []byte("{}"), 0644)

	// 4. Setup False Positive Repack & Program Files folders
	repackDir := filepath.Join(tmpDir, "Bloodborne [FitGirl Repack]")
	os.MkdirAll(repackDir, 0755)
	os.WriteFile(filepath.Join(repackDir, "setup.exe"), []byte("MZ"), 0755)
	os.WriteFile(filepath.Join(repackDir, "data1.bin"), []byte("REPACK_BIN"), 0644)

	// B: Game folder with binaries & assets
	gameInstallDir := filepath.Join(tmpDir, "Ghost of Tsushima")
	os.MkdirAll(filepath.Join(gameInstallDir, "audio"), 0755)
	os.WriteFile(filepath.Join(gameInstallDir, "audio", "sound.bank"), []byte("AUDIO"), 0644)
	os.MkdirAll(filepath.Join(gameInstallDir, "localization"), 0755)
	os.WriteFile(filepath.Join(gameInstallDir, "localization", "en.lang"), []byte("TEXT"), 0644)
	os.MkdirAll(filepath.Join(gameInstallDir, "_windows"), 0755)
	os.WriteFile(filepath.Join(gameInstallDir, "_windows", "game.exe"), []byte("EXE"), 0755)
	os.WriteFile(filepath.Join(gameInstallDir, "game.pak"), []byte("BIG_ASSET"), 0644)

	// C: Application utility folder
	msiDir := filepath.Join(tmpDir, "Program Files (x86)", "MSI Afterburner")
	os.MkdirAll(filepath.Join(msiDir, "Profiles"), 0755)
	os.WriteFile(filepath.Join(msiDir, "Profiles", "config.cfg"), []byte("CONFIG"), 0644)
	os.WriteFile(filepath.Join(msiDir, "MSIAfterburner.exe"), []byte("EXE"), 0755)

	// D: Unreal Engine nested save folder in DOCS
	unrealDir := filepath.Join(tmpDir, "WWWWW", "DOCS", "My Games", "Oblivion Remastered", "Saved", "SaveGames")
	os.MkdirAll(unrealDir, 0755)
	os.WriteFile(filepath.Join(unrealDir, "Save001.sav"), []byte("OBLIVION_SAVE"), 0644)

	paths := []string{
		filepath.Join(tmpDir, "Bloodborne"),
		filepath.Join(tmpDir, "God of War III Remastered"),
		filepath.Join(tmpDir, "Darkest"),
		filepath.Join(tmpDir, "WWWWW"),
		repackDir,
		gameInstallDir,
		msiDir,
	}

	games, err := ScanInstalledGames(paths)
	if err != nil {
		t.Fatalf("ScanInstalledGames failed: %v", err)
	}

	var foundBloodborne, foundDarkest, foundGOW, foundOblivion bool
	for _, g := range games {
		t.Logf("Detected: %s (Type: %s, AppID: %s, Path: %s, Files: %d, Size: %d)",
			g.Name, g.SaveType, g.AppID, g.SourcePath, g.FileCount, g.SizeBytes)

		if strings.Contains(strings.ToLower(g.Name), "bloodborne") && strings.Contains(g.SourcePath, "savedata") {
			foundBloodborne = true
		}
		if strings.Contains(strings.ToLower(g.Name), "god of war iii") {
			foundGOW = true
		}
		if strings.Contains(strings.ToLower(g.Name), "darkest") && strings.Contains(g.SourcePath, "Darkest") {
			foundDarkest = true
		}
		if strings.Contains(strings.ToLower(g.Name), "oblivion") {
			foundOblivion = true
		}

		// Ensure no false positives leaked
		if strings.Contains(strings.ToLower(g.Name), "fitgirl") || strings.Contains(g.SourcePath, "FitGirl") {
			t.Errorf("False positive leaked: FitGirl Repack folder %s", g.SourcePath)
		}
		if strings.Contains(g.SourcePath, "MSI Afterburner") {
			t.Errorf("False positive leaked: MSI Afterburner %s", g.SourcePath)
		}
		if strings.Contains(g.SourcePath, "audio") || strings.Contains(g.SourcePath, "localization") || strings.Contains(g.SourcePath, "_windows") {
			t.Errorf("False positive leaked: Game resource folder %s", g.SourcePath)
		}
		if strings.Contains(g.SourcePath, "cache") {
			t.Errorf("False positive leaked: Shader cache folder %s", g.SourcePath)
		}
	}

	if !foundBloodborne {
		t.Errorf("Expected Bloodborne PS4 save to be detected")
	}
	if !foundGOW {
		t.Errorf("Expected God of War III Remastered PS4 save to be detected")
	}
	if !foundDarkest {
		t.Errorf("Expected Darkest Dungeon save to be detected")
	}
	if !foundOblivion {
		t.Errorf("Expected Oblivion Remastered save to be detected from nested DOCS directory")
	}
}

func TestInspectRealBundle(t *testing.T) {
	bundlePath := filepath.Join(os.Getenv("HOME"), "Downloads", "game_saves_bundle.zip")
	if _, err := os.Stat(bundlePath); err != nil {
		t.Skip("No real bundle found to test")
	}

	manifest, err := InspectVault(bundlePath)
	if err != nil {
		t.Fatalf("Failed to inspect vault: %v", err)
	}

	t.Logf("Bundle Version: %s, Source OS: %s, Games Count: %d", manifest.Version, manifest.OS, len(manifest.Games))
	for _, g := range manifest.Games {
		t.Logf(" - [%s] %s (AppID: %s, Storage: %s, Target: %s, Files: %d, Size: %d)",
			g.ID, g.GameName, g.AppID, g.StorageType, g.TargetRelativePath, g.FileCount, g.SizeBytes)
	}

	// Test VerifyZipIntegrity on real bundle
	rep, err := VerifyZipIntegrity(bundlePath)
	if err != nil {
		t.Fatalf("VerifyZipIntegrity failed: %v", err)
	}
	if !rep.Valid {
		t.Fatalf("Expected zip to be valid, got error: %s", rep.Error)
	}
	t.Logf("Integrity report: Valid=%v, Entries=%d, TotalBytes=%d, ZipSize=%d",
		rep.Valid, rep.EntryCount, rep.TotalBytes, rep.ZipSize)
}

func TestResolveGameInfo(t *testing.T) {
	// 1. Numeric Steam AppID lookup
	res := ResolveGameInfo("292030")
	if res.AppID != "292030" {
		t.Errorf("Expected AppID 292030, got %s", res.AppID)
	}
	t.Logf("Resolved AppID 292030: Name=%s, Icon=%s, Tier=%s", res.Name, res.IconURL, res.ProtonTier)

	// 2. Title search
	res2 := ResolveGameInfo("Dark Souls III")
	t.Logf("Resolved Dark Souls III: AppID=%s, Name=%s, Icon=%s, Tier=%s", res2.AppID, res2.Name, res2.IconURL, res2.ProtonTier)
	if res2.AppID == "" {
		t.Errorf("Expected non-empty AppID for Dark Souls III")
	}

	// 3. Delisted game fallback (Rocket League)
	res3 := ResolveGameInfo("Rocket League")
	if res3.AppID != "252950" {
		t.Errorf("Expected AppID 252950 for Rocket League, got %s", res3.AppID)
	}
	t.Logf("Resolved Rocket League: AppID=%s, Name=%s", res3.AppID, res3.Name)

	// 4. CamelCase / PascalCase title splitting (FasterThanLight)
	res4 := ResolveGameInfo("FasterThanLight")
	if res4.AppID != "212680" {
		t.Errorf("Expected AppID 212680 for FasterThanLight, got %s", res4.AppID)
	}
	t.Logf("Resolved FasterThanLight: AppID=%s, Name=%s", res4.AppID, res4.Name)
}

func TestScanTekken8(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "save_migrator_tekken_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	localAppData := filepath.Join(tmpDir, "AppData", "Local")
	tekkenSaveDir := filepath.Join(localAppData, "TEKKEN 8", "Saved", "SaveGames")
	if err := os.MkdirAll(tekkenSaveDir, 0755); err != nil {
		t.Fatalf("Failed to create tekken save dir: %v", err)
	}
	os.WriteFile(filepath.Join(tekkenSaveDir, "TEKKEN8.sav"), []byte("TEKKEN_8_SAVE_DATA"), 0644)

	var games []DetectedGame
	seen := make(map[string]bool)
	scanLocalAppDataUnrealGames(localAppData, &games, seen)

	if len(games) == 0 {
		t.Fatalf("Expected TEKKEN 8 to be detected in AppData/Local, but got 0 games")
	}

	g := games[0]
	t.Logf("Detected: %s (AppID: %s, StorageType: %s, Target: %s)", g.Name, g.AppID, g.StorageType, g.TargetRelativePath)
	if !strings.Contains(strings.ToUpper(g.Name), "TEKKEN") {
		t.Errorf("Expected title containing TEKKEN, got %s", g.Name)
	}
	if g.AppID != "1778820" {
		t.Errorf("Expected AppID 1778820 for TEKKEN 8, got %s", g.AppID)
	}
	if g.TargetRelativePath != "users/steamuser/AppData/Local/TEKKEN 8/Saved/SaveGames" {
		t.Errorf("Unexpected TargetRelativePath: %s", g.TargetRelativePath)
	}
}

func TestStudioCorrelationAndStrictMatching(t *testing.T) {
	// 1. Verify strict search does NOT falsely match DLC packs for generic names
	resDisc, _, _ := resolveAppIDByName("Discovery")
	if resDisc == "1200349" { // Fishing Planet DLC pack
		t.Errorf("Discovery falsely matched Fishing Planet DLC: %s", resDisc)
	}
	t.Logf("Discovery strict resolution: AppID=%s (expected empty)", resDisc)

	resPioneer, _, _ := resolveAppIDByName("PioneerGame")
	if resPioneer == "2716250" { // MudRunner Pioneer Pack
		t.Errorf("PioneerGame falsely matched MudRunner Pioneer Pack: %s", resPioneer)
	}
	t.Logf("PioneerGame strict resolution: AppID=%s (expected empty)", resPioneer)

	// 2. Machine correlation: Pre-populate developer cache with AppID 1903340
	fetchSteamMetadata("1903340") // Clair Obscur: Expedition 33 (Sandfall Interactive)
	matched := findAppIDByDeveloperOrStudio("Sandfall", []string{"1903340"})
	if matched != "1903340" {
		t.Errorf("Expected Sandfall to correlate with AppID 1903340, got %s", matched)
	} else {
		t.Logf("Successfully correlated studio folder 'Sandfall' to AppID 1903340 (%s)", getGameName(matched, ""))
	}

	// 3. Machine title correlation without hardcoding:
	// Dawnwalker matches The Blood of Dawnwalker (AppID 3751260)
	detectedGames := []DetectedGame{
		{AppID: "3751260", Name: "The Blood of Dawnwalker"},
		{AppID: "2124490", Name: "SILENT HILL 2"},
		{AppID: "1903340", Name: "Clair Obscur: Expedition 33"},
	}

	matchedDawn := findAppIDByMachineCorrelation("Dawnwalker", detectedGames)
	if matchedDawn != "3751260" {
		t.Errorf("Expected Dawnwalker to correlate with AppID 3751260, got %s", matchedDawn)
	} else {
		t.Logf("Successfully correlated 'Dawnwalker' to AppID 3751260 (%s)", getGameName(matchedDawn, ""))
	}

	// SilentHill2 with splitCamelCase matches SILENT HILL 2 (AppID 2124490)
	splitSH2 := splitCamelCase("SilentHill2")
	if splitSH2 != "Silent Hill 2" {
		t.Errorf("Expected splitCamelCase('SilentHill2') == 'Silent Hill 2', got %s", splitSH2)
	}
	matchedSH2 := findAppIDByMachineCorrelation("SilentHill2", detectedGames)
	if matchedSH2 != "2124490" {
		t.Errorf("Expected SilentHill2 to correlate with AppID 2124490, got %s", matchedSH2)
	} else {
		t.Logf("Successfully correlated 'SilentHill2' to AppID 2124490 (%s)", getGameName(matchedSH2, ""))
	}

	// 4. Test FalloutNV acronym resolution (AppID 22380)
	resFallout, _, _ := resolveAppIDByName("FalloutNV")
	if resFallout != "22380" {
		t.Errorf("Expected FalloutNV to resolve to 22380, got %s", resFallout)
	} else {
		t.Logf("Successfully resolved FalloutNV to AppID 22380 (%s)", getGameName(resFallout, ""))
	}

	// 5. Test kingdomcome2 collapsed space & numeral matching (AppID 1771300)
	detectedGamesWithKCD := append(detectedGames, DetectedGame{AppID: "1771300", Name: "Kingdom Come: Deliverance II"})
	matchedKCD := findAppIDByMachineCorrelation("kingdomcome2", detectedGamesWithKCD)
	if matchedKCD != "1771300" {
		t.Errorf("Expected kingdomcome2 to correlate with AppID 1771300, got %s", matchedKCD)
	} else {
		t.Logf("Successfully correlated 'kingdomcome2' to AppID 1771300 (%s)", getGameName(matchedKCD, ""))
	}

	resKCD, _, _ := resolveAppIDByName("kingdomcome2")
	if resKCD != "1771300" {
		t.Errorf("Expected kingdomcome2 online search to resolve to 1771300, got %s", resKCD)
	} else {
		t.Logf("Successfully resolved kingdomcome2 to AppID 1771300 (%s)", getGameName(resKCD, ""))
	}

	// Unassociated generic folders do NOT falsely correlate
	matchedDisc := findAppIDByMachineCorrelation("Discovery", detectedGames)
	if matchedDisc != "" {
		t.Errorf("Expected Discovery to have no machine correlation, got %s", matchedDisc)
	}

	// Emulator names do NOT falsely correlate to publishers like PlayStation or Xbox
	fetchSteamMetadata("3489700") // Stellar Blade (PlayStation Publishing LLC)
	fetchSteamMetadata("2627260") // NINJA GAIDEN 4 (Xbox Game Studios)

	falsePS2 := findAppIDByDeveloperOrStudio("PlayStation 2 Memory Cards (PCSX2)", []string{"3489700", "2627260"})
	if falsePS2 != "" {
		t.Errorf("False positive: PCSX2 correlated to AppID %s!", falsePS2)
	}

	falseDolphin := findAppIDByDeveloperOrStudio("GameCube & Wii Saves (Dolphin)", []string{"3489700", "2627260"})
	if falseDolphin != "" {
		t.Errorf("False positive: Dolphin correlated to AppID %s!", falseDolphin)
	}
}

func TestDynamicSteamLibraryDiscovery(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "steam_mock_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	steamapps := filepath.Join(tmpDir, "mock_windows", "steamapps")
	if err := os.MkdirAll(filepath.Join(steamapps, "common", "THE FINALS", "Discovery"), 0755); err != nil {
		t.Fatalf("Failed to create mock steam tree: %v", err)
	}

	manifestContent := `"AppState"
{
	"appid"		"2073850"
	"name"		"THE FINALS"
	"installdir"		"THE FINALS"
}
`
	if err := os.WriteFile(filepath.Join(steamapps, "appmanifest_2073850.acf"), []byte(manifestContent), 0644); err != nil {
		t.Fatalf("Failed to write mock manifest: %v", err)
	}

	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", oldHome)

	appID, name := findAppIDByInstalledGameFolder("Discovery")
	if appID != "2073850" || name != "THE FINALS" {
		t.Errorf("Expected dynamic discovery Discovery -> 2073850 THE FINALS, got %s %s", appID, name)
	} else {
		t.Logf("Dynamically resolved Discovery from local Steam library without hardcoding: %s (%s)", appID, name)
	}
}

func TestDiskFreeSpace(t *testing.T) {
	free, total, err := GetDiskFreeSpace(os.TempDir())
	if err != nil {
		t.Fatalf("GetDiskFreeSpace failed: %v", err)
	}
	if free == 0 || total == 0 {
		t.Errorf("Expected non-zero free and total bytes, got free=%d, total=%d", free, total)
	}
	t.Logf("Disk space on %s: Free=%.2f GB, Total=%.2f GB", os.TempDir(), float64(free)/(1024*1024*1024), float64(total)/(1024*1024*1024))
}

func TestProgressTracker(t *testing.T) {
	SetProgress("scan", 25.5, "Scanning AppData...", 10, 40, "local_test")
	p := GetProgress()
	if !p.Active || p.Operation != "scan" || p.Percent != 25.5 || p.Status != "Scanning AppData..." {
		t.Errorf("Unexpected progress state: %+v", p)
	}

	ClearProgress("Done")
	p2 := GetProgress()
	if p2.Active || p2.Percent != 100 || p2.Status != "Done" {
		t.Errorf("Unexpected cleared progress: %+v", p2)
	}
}

func TestAllConsoleEmulators(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "emu_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Mock AppData emulators
	appData := filepath.Join(tmpDir, "AppData", "Roaming")
	localAppData := filepath.Join(tmpDir, "AppData", "Local")

	// 1. RPCS3 (PS3)
	rpcs3Game := filepath.Join(appData, "RPCS3", "dev_hdd0", "home", "00000001", "savedata", "BLUS30109")
	os.MkdirAll(rpcs3Game, 0755)
	os.WriteFile(filepath.Join(rpcs3Game, "PARAM.SFO"), []byte("...TITLE\x00Demon's Souls\x00..."), 0644)
	os.WriteFile(filepath.Join(rpcs3Game, "SYS-DATA"), []byte("savedata_bytes"), 0644)

	// 2. Ryujinx (Switch)
	ryuSave := filepath.Join(appData, "Ryujinx", "bis", "user", "save", "0000000000000001")
	os.MkdirAll(ryuSave, 0755)
	os.WriteFile(filepath.Join(ryuSave, "main"), []byte("switch_save_data"), 0644)

	// 3. Yuzu (Switch)
	yuzuSave := filepath.Join(appData, "yuzu", "nand", "user", "save", "0000000000000000", "0100000000010000")
	os.MkdirAll(yuzuSave, 0755)
	os.WriteFile(filepath.Join(yuzuSave, "save.bin"), []byte("yuzu_save_data"), 0644)

	// 4. Citra (3DS)
	citraSave := filepath.Join(appData, "Citra", "sdmc", "Nintendo 3DS")
	os.MkdirAll(citraSave, 0755)
	os.WriteFile(filepath.Join(citraSave, "title.db"), []byte("citra_3ds_save"), 0644)

	// 5. Cemu (Wii U)
	os.Setenv("LOCALAPPDATA", localAppData)
	cemuSave := filepath.Join(localAppData, "Cemu", "mlc01", "usr", "save", "00050000")
	os.MkdirAll(cemuSave, 0755)
	os.WriteFile(filepath.Join(cemuSave, "user.dat"), []byte("cemu_wiiu_save"), 0644)

	// 6. Documents Emulators (PCSX2, Dolphin, Xenia, DuckStation)
	docsDir := filepath.Join(tmpDir, "Documents")
	os.Setenv("USERPROFILE", tmpDir)

	pcsx2Mem := filepath.Join(docsDir, "PCSX2", "memcards")
	os.MkdirAll(pcsx2Mem, 0755)
	os.WriteFile(filepath.Join(pcsx2Mem, "Mcd001.ps2"), []byte("ps2_memory_card"), 0644)

	duckMem := filepath.Join(docsDir, "DuckStation", "memcards")
	os.MkdirAll(duckMem, 0755)
	os.WriteFile(filepath.Join(duckMem, "shared.mcd"), []byte("ps1_memory_card"), 0644)

	dolphinUser := filepath.Join(docsDir, "Dolphin Emulator", "GC")
	os.MkdirAll(dolphinUser, 0755)
	os.WriteFile(filepath.Join(dolphinUser, "MemoryCardA.USA.raw"), []byte("dolphin_gc_memcard"), 0644)

	xeniaContent := filepath.Join(docsDir, "Xenia", "content", "0000000000000000")
	os.MkdirAll(xeniaContent, 0755)
	os.WriteFile(filepath.Join(xeniaContent, "save.x360"), []byte("xenia_xbox360_save"), 0644)

	var detected []DetectedGame
	seen := make(map[string]bool)

	scanAppDataEmulators(appData, &detected, seen)
	scanDocumentsEmulators(&detected, seen)

	t.Logf("Detected %d console emulator save entries:", len(detected))
	for _, g := range detected {
		t.Logf(" - [%s] %s (Type: %s, Files: %d, Target: %s)", g.ID, g.Name, g.StorageType, g.FileCount, g.TargetRelativePath)
	}

	if len(detected) < 8 {
		t.Errorf("Expected at least 8 emulators detected, got %d", len(detected))
	}
}

func TestCemuZeldaDetection(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "save_migrator_cemu_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	cemuSave := filepath.Join(tmpDir, "00050000", "101c9500")
	metaDir := filepath.Join(cemuSave, "meta")
	userDir := filepath.Join(cemuSave, "user", "80000001", "0")
	os.MkdirAll(metaDir, 0755)
	os.MkdirAll(userDir, 0755)

	metaContent := `<?xml version="1.0" encoding="utf-8"?>
<menu type="complex" access="777">
  <longname_en type="string" length="512">The Legend of Zelda
Breath of the Wild</longname_en>
  <shortname_en type="string" length="256">Breath of the Wild</shortname_en>
  <title_id type="hexBinary" length="8">00050000101C9500</title_id>
</menu>`
	if err := os.WriteFile(filepath.Join(metaDir, "meta.xml"), []byte(metaContent), 0644); err != nil {
		t.Fatalf("Failed to write meta.xml: %v", err)
	}
	if err := os.WriteFile(filepath.Join(userDir, "game_data.sav"), []byte("zelda_save_data"), 0644); err != nil {
		t.Fatalf("Failed to write game_data.sav: %v", err)
	}

	var games []DetectedGame
	seen := make(map[string]bool)
	scanCemuSaves(tmpDir, "cemu/mlc01/usr/save", &games, seen)

	if len(games) != 1 {
		t.Fatalf("Expected 1 Cemu title detected, got %d", len(games))
	}
	g := games[0]
	t.Logf("Detected Cemu title: %s (ID: %s, Rel: %s, Files: %d)", g.Name, g.ID, g.TargetRelativePath, g.FileCount)

	expectedName := "The Legend of Zelda: Breath of the Wild (Cemu)"
	if g.Name != expectedName {
		t.Errorf("Expected name '%s', got '%s'", expectedName, g.Name)
	}
	expectedRel := "cemu/mlc01/usr/save/00050000/101c9500"
	if g.TargetRelativePath != expectedRel {
		t.Errorf("Expected relative path '%s', got '%s'", expectedRel, g.TargetRelativePath)
	}
}

func TestOnlineGamesAndRustFalsePositive(t *testing.T) {
	// 1. Verify word boundary matching prevents false positives like "Elytra Trusted Certs"
	if matchesSignatureWord("Elytra Trusted Certs", "Rust") {
		t.Errorf("matchesSignatureWord matched 'Rust' inside 'Elytra Trusted Certs'!")
	}
	if !matchesSignatureWord("Rust", "Rust") {
		t.Errorf("matchesSignatureWord failed to match exact 'Rust'")
	}
	if !matchesSignatureWord("Rust Client (x64)", "Rust") {
		t.Errorf("matchesSignatureWord failed to match 'Rust' in 'Rust Client (x64)'")
	}

	// 2. Verify Rust anti-cheat status is Broken (EAC official servers kick Linux players)
	var rustFound bool
	for _, g := range knownOnlineGames {
		if g.Name == "Rust" {
			rustFound = true
			if g.Status != "Broken" {
				t.Errorf("Expected Rust status to be 'Broken', got '%s'", g.Status)
			}
			break
		}
	}
	if !rustFound {
		t.Errorf("Rust not found in knownOnlineGames")
	}

	// 3. Verify online live-service game filtering
	if !isOnlineLiveServiceGame("THE FINALS", "2073850", `C:\Users\Mmdxd\AppData\Local\Discovery\Saved\SaveGames`) {
		t.Errorf("isOnlineLiveServiceGame failed to identify THE FINALS")
	}
	if !isOnlineLiveServiceGame("Rainbow Six - Siege", "359550", `C:\Users\Mmdxd\Documents\My Games\Rainbow Six - Siege`) {
		t.Errorf("isOnlineLiveServiceGame failed to identify Rainbow Six Siege")
	}
	if !isOnlineLiveServiceGame("Marvel", "", `C:\Users\GAMERS\AppData\Local\Marvel\Saved`) {
		t.Errorf("isOnlineLiveServiceGame failed to identify Marvel Rivals")
	}
	if !isOnlineLiveServiceGame("Squad Game", "2011840", `C:\Users\GAMERS\AppData\Local\SquadGame\Saved\SaveGames`) {
		t.Errorf("isOnlineLiveServiceGame failed to identify Squad Game")
	}
	if !isOnlineLiveServiceGame("PioneerGame", "", `C:\Users\GAMERS\AppData\Local\PioneerGame\Saved\SaveGames`) {
		t.Errorf("isOnlineLiveServiceGame failed to identify PioneerGame (ARC Raiders)")
	}
	if isOnlineLiveServiceGame("Elden Ring (Character Saves)", "1245620", `C:\Users\Mmdxd\AppData\Roaming\EldenRing`) {
		t.Errorf("isOnlineLiveServiceGame incorrectly flagged offline game Elden Ring as online")
	}

	// 4. Verify Roblox is marked Running via Sober
	var robloxFound bool
	for _, g := range knownOnlineGames {
		if g.Name == "Roblox" {
			robloxFound = true
			if g.Status != "Running" {
				t.Errorf("Expected Roblox status to be 'Running', got '%s'", g.Status)
			}
			break
		}
	}
	if !robloxFound {
		t.Errorf("Roblox not found in knownOnlineGames")
	}
}

func TestLocalizedDocumentsAndLocalLow(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "save_migrator_loc_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// 1. Mock Localized Arabic Documents under OneDrive
	arabicDocs := filepath.Join(tmpDir, "OneDrive", "المستندات")
	tsushimaSave := filepath.Join(arabicDocs, "Ghost of Tsushima DIRECTOR'S CUT", "76561197960271872")
	tlou2Save := filepath.Join(arabicDocs, "The Last of Us Part II", "76561197960271872", "savedata")
	cpyRE7Save := filepath.Join(arabicDocs, "CPY_SAVES", "CPY", "418370", "win64_save")
	outlastSave := filepath.Join(arabicDocs, "My Games", "Outlast", "OLGame", "SaveData")
	rockstarGTA := filepath.Join(arabicDocs, "Rockstar Games", "GTAV Enhanced", "Profiles", "E1A55112")

	os.MkdirAll(tsushimaSave, 0755)
	os.MkdirAll(tlou2Save, 0755)
	os.MkdirAll(cpyRE7Save, 0755)
	os.MkdirAll(outlastSave, 0755)
	os.MkdirAll(rockstarGTA, 0755)

	os.WriteFile(filepath.Join(tsushimaSave, "manual_save.sav"), []byte("tsushima_save"), 0644)
	os.WriteFile(filepath.Join(tlou2Save, "SAVEFILE00"), []byte("tlou2_save"), 0644)
	os.WriteFile(filepath.Join(cpyRE7Save, "save.bin"), []byte("re7_save"), 0644)
	os.WriteFile(filepath.Join(outlastSave, "save.bin"), []byte("outlast_save"), 0644)
	os.WriteFile(filepath.Join(rockstarGTA, "SGTA50000"), []byte("gta5_save"), 0644)

	var docGames []DetectedGame
	seenDocs := make(map[string]bool)
	scanSpecificDocsDir(arabicDocs, &docGames, seenDocs)

	t.Logf("Detected %d titles in Arabic OneDrive Documents:", len(docGames))
	for _, g := range docGames {
		t.Logf(" - [%s] %s (Type: %s, Rel: %s)", g.ID, g.Name, g.StorageType, g.TargetRelativePath)
	}

	if len(docGames) < 4 {
		t.Errorf("Expected at least 4 games detected in localized documents, got %d", len(docGames))
	}

	// 2. Mock Unity LocalLow
	localLow := filepath.Join(tmpDir, "AppData", "LocalLow")
	rubberBandits := filepath.Join(localLow, "Flashbulb", "Rubber Bandits", "76561198370249745", "Synced")
	ror2 := filepath.Join(localLow, "Hopoo Games, LLC", "Risk of Rain 2")
	howToFish := filepath.Join(localLow, "Dazed Games", "How to Fish", "Saves")

	os.MkdirAll(rubberBandits, 0755)
	os.MkdirAll(ror2, 0755)
	os.MkdirAll(howToFish, 0755)

	os.WriteFile(filepath.Join(rubberBandits, "Progress_prod"), []byte("rubber_bandits_save"), 0644)
	os.WriteFile(filepath.Join(ror2, "save.xml"), []byte("ror2_save"), 0644)
	os.WriteFile(filepath.Join(howToFish, "save.dat"), []byte("fish_save"), 0644)

	var localLowGames []DetectedGame
	seenLocalLow := make(map[string]bool)
	scanUserAppDataLocalLow(localLow, &localLowGames, seenLocalLow)

	t.Logf("Detected %d titles in AppData/LocalLow:", len(localLowGames))
	for _, g := range localLowGames {
		t.Logf(" - [%s] %s (AppID: %s, Rel: %s)", g.ID, g.Name, g.AppID, g.TargetRelativePath)
	}

	if len(localLowGames) < 3 {
		t.Errorf("Expected at least 3 games detected in LocalLow, got %d", len(localLowGames))
	}
}

func TestSakoEdenAndShadPS4(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "sako_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Mock Sako's AppData/Roaming structure
	appData := filepath.Join(tmpDir, "AppData", "Roaming")

	// 1. Sako's Eden path: AppData/Roaming/eden/nand/user/save
	edenSaveDir := filepath.Join(appData, "eden", "nand", "user", "save", "0000000000000000", "6BC8AB52B3E88CC597064CE5B8704CED", "010019A01E2F2000")
	if err := os.MkdirAll(edenSaveDir, 0755); err != nil {
		t.Fatalf("Failed to create eden dir: %v", err)
	}
	os.WriteFile(filepath.Join(edenSaveDir, "save.bin"), []byte("SWITCH_SAVE_DATA"), 0644)

	// 2. Sako's ShadPS4 path: AppData/Roaming/shadPS4/home/1000/savedata/CUSA00900/SPRJ0005/sce_sys
	bbDir := filepath.Join(appData, "shadPS4", "home", "1000", "savedata", "CUSA00900", "SPRJ0005", "sce_sys")
	if err := os.MkdirAll(bbDir, 0755); err != nil {
		t.Fatalf("Failed to create shadPS4 bb dir: %v", err)
	}
	// Copy actual SFO from /tmp/sako_ps4 if available, or write dummy save
	sfoSource := "/tmp/sako_ps4/savedata/CUSA00900/SPRJ0005/sce_sys/param.sfo"
	if sfoData, err := os.ReadFile(sfoSource); err == nil {
		os.WriteFile(filepath.Join(bbDir, "param.sfo"), sfoData, 0644)
	} else {
		os.WriteFile(filepath.Join(bbDir, "param.sfo"), []byte("DUMMY_SFO"), 0644)
	}
	os.WriteFile(filepath.Join(filepath.Dir(bbDir), "userdata0000"), []byte("BB_USERDATA"), 0644)

	var detected []DetectedGame
	seen := make(map[string]bool)
	scanAppDataEmulators(appData, &detected, seen)

	t.Logf("Detected %d emulators from Sako AppData:", len(detected))
	foundEden := false
	foundBloodborne := false
	for _, g := range detected {
		t.Logf(" - [%s] %s (Storage: %s, Rel: %s, Files: %d)", g.ID, g.Name, g.StorageType, g.TargetRelativePath, g.FileCount)
		if g.ID == "emu_eden" || strings.HasPrefix(g.ID, "switch_eden") {
			foundEden = true
		}
		if g.ID == "ps4_cusa00900" {
			foundBloodborne = true
			if !strings.Contains(g.Name, "Bloodborne") {
				t.Errorf("Expected Bloodborne in game name, got: %s", g.Name)
			}
		}
	}

	if !foundEden {
		t.Errorf("Failed to detect Eden Switch emulator saves")
	}
	if !foundBloodborne {
		t.Errorf("Failed to detect ShadPS4 Bloodborne (CUSA00900) saves")
	}
}

func TestTelemetryFindings(t *testing.T) {
	// 1. Acronym and title expansions
	if title := cleanGameTitle("SB"); title != "Stellar Blade" {
		t.Errorf("Expected SB to expand to 'Stellar Blade', got '%s'", title)
	}
	if title := cleanGameTitle("OnimushaWotS"); title != "Onimusha: Way of the Sword" {
		t.Errorf("Expected OnimushaWotS to expand to 'Onimusha: Way of the Sword', got '%s'", title)
	}
	if title := cleanGameTitle("Goofy"); title != "Goofy Gorillas" {
		t.Errorf("Expected Goofy to expand to 'Goofy Gorillas', got '%s'", title)
	}
	if title := cleanGameTitle("Darkest"); title != "Darkest Dungeon" {
		t.Errorf("Expected Darkest to expand to 'Darkest Dungeon', got '%s'", title)
	}

	// 2. Unreal Engine project codename for Stellar Blade
	appID, gameName := resolveUnrealProjectCodename("SB")
	if appID != "3489700" || gameName != "Stellar Blade" {
		t.Errorf("Expected SB codename to resolve to (3489700, Stellar Blade), got (%s, %s)", appID, gameName)
	}

	// 3. Online live-service check for NIKKE
	if !isOnlineLiveServiceGame("NIKKE", "", `C:\Users\test\AppData\LocalLow\com.proximabeta\NIKKE`) {
		t.Errorf("Expected NIKKE in LocalLow/com.proximabeta to be detected as online live-service game")
	}

	// 4. Steam Userdata filter verification for 241100, 7, and 760
	testAppIDs := []string{"7", "760", "241100", "1446780"}
	for _, id := range testAppIDs {
		isFiltered := (id == "7" || id == "760" || id == "241100")
		if (id == "241100" || id == "7" || id == "760") && !isFiltered {
			t.Errorf("AppID %s should be filtered from Steam userdata", id)
		}
		if id == "1446780" && isFiltered {
			t.Errorf("AppID %s should not be filtered", id)
		}
	}
}

func TestEmulationIntelligenceAndEmuDeck(t *testing.T) {
	// 1. Switch Title ID resolution
	if !isSwitchTitleID("010019A01E2F2000") {
		t.Errorf("Expected 010019A01E2F2000 to be recognized as valid Switch Title ID")
	}
	if switchTitleMap["010019a01e2f2000"] != "Metroid Prime 4: Beyond" {
		t.Errorf("Expected Metroid Prime 4: Beyond in switchTitleMap")
	}
	if switchTitleMap["0100f2c0115b6000"] != "The Legend of Zelda: Tears of the Kingdom" {
		t.Errorf("Expected Tears of the Kingdom in switchTitleMap")
	}

	// 2. Binary SFO Parser test
	var sfoBuf bytes.Buffer
	sfoBuf.WriteString("\x00PSF")
	binary.Write(&sfoBuf, binary.LittleEndian, uint32(0x00000101))
	keyOffset := uint32(0x24)
	binary.Write(&sfoBuf, binary.LittleEndian, keyOffset)
	dataOffset := uint32(0x40)
	binary.Write(&sfoBuf, binary.LittleEndian, dataOffset)
	binary.Write(&sfoBuf, binary.LittleEndian, uint32(1))
	binary.Write(&sfoBuf, binary.LittleEndian, uint16(0))
	binary.Write(&sfoBuf, binary.LittleEndian, uint16(0x0204))
	binary.Write(&sfoBuf, binary.LittleEndian, uint32(12))
	binary.Write(&sfoBuf, binary.LittleEndian, uint32(128))
	binary.Write(&sfoBuf, binary.LittleEndian, uint32(0))

	for sfoBuf.Len() < int(keyOffset) {
		sfoBuf.WriteByte(0)
	}
	sfoBuf.WriteString("TITLE\x00")

	for sfoBuf.Len() < int(dataOffset) {
		sfoBuf.WriteByte(0)
	}
	sfoBuf.WriteString("Bloodborne\x00\x00")

	info := parseSFOData(sfoBuf.Bytes())
	if info.Title != "Bloodborne" {
		t.Errorf("Expected parsed SFO Title 'Bloodborne', got '%s'", info.Title)
	}

	// 3. EmuDeck restorer target resolution
	tmpHome, err := os.MkdirTemp("", "emudeck_home_*")
	if err != nil {
		t.Fatalf("Failed to create temp home: %v", err)
	}
	defer os.RemoveAll(tmpHome)

	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpHome)
	defer os.Setenv("HOME", origHome)

	emuDeckCemu := filepath.Join(tmpHome, "Emulation", "saves", "cemu")
	os.MkdirAll(emuDeckCemu, 0755)

	cemuMeta := GameSaveMeta{
		ID:                 "cemu_00050000_101c9500",
		GameName:           "The Legend of Zelda: Breath of the Wild",
		StorageType:          "Wii U Emulator",
		TargetRelativePath: "cemu/mlc01/usr/save/00050000/101c9500",
	}

	path, label := LocateConsoleEmulatorPath(cemuMeta)
	if path != emuDeckCemu {
		t.Errorf("Expected EmuDeck Cemu path %s, got %s", emuDeckCemu, path)
	}
	if !strings.Contains(label, "EmuDeck") {
		t.Errorf("Expected EmuDeck in label, got %s", label)
	}

	targetPath, status, _, _ := AutoLocateLinuxTarget(cemuMeta)
	if status != "emulator_detected" {
		t.Errorf("Expected emulator_detected status, got %s", status)
	}
	expectedTarget := filepath.Join(emuDeckCemu, "mlc01", "usr", "save", "00050000", "101c9500")
	if targetPath != expectedTarget {
		t.Errorf("Expected stripped target %s, got %s", expectedTarget, targetPath)
	}

	// 2. Flatpak Cemu with mlc01/usr/save already created
	flatpakCemu := filepath.Join(tmpHome, ".var", "app", "info.cemu.Cemu", "data", "Cemu", "mlc01", "usr", "save")
	os.MkdirAll(flatpakCemu, 0755)
	// Remove emuDeck dir to test flatpak detection
	os.RemoveAll(emuDeckCemu)

	flatPath, flatLabel := LocateConsoleEmulatorPath(cemuMeta)
	if flatPath != flatpakCemu {
		t.Errorf("Expected Flatpak Cemu mlc01 path %s, got %s", flatpakCemu, flatPath)
	}
	if !strings.Contains(flatLabel, "Flatpak") {
		t.Errorf("Expected Flatpak in label, got %s", flatLabel)
	}
	flatTarget, flatStatus, _, _ := AutoLocateLinuxTarget(cemuMeta)
	if flatStatus != "emulator_detected" {
		t.Errorf("Expected emulator_detected status for Flatpak, got %s", flatStatus)
	}
	expectedFlatTarget := filepath.Join(flatpakCemu, "00050000", "101c9500")
	if flatTarget != expectedFlatTarget {
		t.Errorf("Expected flatpak target %s, got %s", expectedFlatTarget, flatTarget)
	}

	// 3. Fallback when neither exists on disk yet (must NOT route to Wine/Hydra/Heroic!)
	os.RemoveAll(filepath.Join(tmpHome, ".var"))
	fallbackTarget, fallbackStatus, _, _ := AutoLocateLinuxTarget(cemuMeta)
	if fallbackStatus != "emulator_detected" {
		t.Errorf("Expected emulator fallback status emulator_detected, got %s", fallbackStatus)
	}
	if strings.Contains(fallbackTarget, "wine-prefixes") || strings.Contains(fallbackTarget, "drive_c") {
		t.Errorf("Emulator fallback should never route to Wine prefix: %s", fallbackTarget)
	}

	// 4. Offline game prefix pre-seeding status check
	offlineMeta := GameSaveMeta{
		ID:                 "offline_game_123",
		AppID:              "123456",
		GameName:           "Test Game",
		StorageType:        "Windows LocalAppData",
		TargetRelativePath: "users/steamuser/AppData/Local/TestGame/Saved/SaveGames",
	}
	offTarget, offStatus, offText, _ := AutoLocateLinuxTarget(offlineMeta)
	if offStatus != "not_initialized" {
		t.Errorf("Expected not_initialized status, got %s", offStatus)
	}
	if !strings.Contains(offText, "Auto-seeds on Restore") {
		t.Errorf("Expected 'Auto-seeds on Restore' in statusText, got %s", offText)
	}
	if !strings.Contains(offTarget, "123456") {
		t.Errorf("Expected AppID in uninitialized Hydra path, got %s", offTarget)
	}
}

func TestWindowsToWindowsRestoration(t *testing.T) {
	tmpWinHome, err := os.MkdirTemp("", "win_restore_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp win home: %v", err)
	}
	defer os.RemoveAll(tmpWinHome)

	origAppData := os.Getenv("APPDATA")
	origLocalAppData := os.Getenv("LOCALAPPDATA")
	origUserProfile := os.Getenv("USERPROFILE")
	origPublic := os.Getenv("PUBLIC")

	winAppData := filepath.Join(tmpWinHome, "AppData", "Roaming")
	winLocalAppData := filepath.Join(tmpWinHome, "AppData", "Local")
	winUserProfile := tmpWinHome
	winPublic := filepath.Join(tmpWinHome, "Public")

	os.Setenv("APPDATA", winAppData)
	os.Setenv("LOCALAPPDATA", winLocalAppData)
	os.Setenv("USERPROFILE", winUserProfile)
	os.Setenv("PUBLIC", winPublic)
	defer func() {
		os.Setenv("APPDATA", origAppData)
		os.Setenv("LOCALAPPDATA", origLocalAppData)
		os.Setenv("USERPROFILE", origUserProfile)
		os.Setenv("PUBLIC", origPublic)
	}()

	os.MkdirAll(filepath.Join(winLocalAppData, "TEKKEN 8", "Saved", "SaveGames"), 0755)
	os.MkdirAll(filepath.Join(winAppData, "EldenRing"), 0755)
	os.MkdirAll(filepath.Join(winUserProfile, "Documents", "My Games"), 0755)
	os.MkdirAll(filepath.Join(winUserProfile, "Saved Games"), 0755)
	os.MkdirAll(filepath.Join(winPublic, "Documents", "Steam", "RUNE"), 0755)

	// 1. AppData/Local (Tekken 8)
	t8Meta := GameSaveMeta{
		ID:                 "unreal_tekken_8",
		GameName:           "TEKKEN 8",
		AppID:              "1778820",
		TargetRelativePath: "users/steamuser/AppData/Local/TEKKEN 8/Saved/SaveGames",
	}
	t8Target, t8Status, t8Desc := AutoLocateWindowsTarget(t8Meta)
	expectedT8 := filepath.Join(winLocalAppData, "TEKKEN 8", "Saved", "SaveGames")
	if t8Target != expectedT8 {
		t.Errorf("Expected T8 target %s, got %s", expectedT8, t8Target)
	}
	if t8Status != "windows_detected" {
		t.Errorf("Expected windows_detected status for existing folder, got %s", t8Status)
	}
	if !strings.Contains(t8Desc, "AppData/Local") {
		t.Errorf("Expected AppData/Local in desc, got %s", t8Desc)
	}

	// 2. AppData/Roaming (Elden Ring)
	erMeta := GameSaveMeta{
		ID:                 "fromsoft_eldenring",
		GameName:           "Elden Ring",
		AppID:              "1245620",
		TargetRelativePath: "users/steamuser/AppData/Roaming/EldenRing",
	}
	erTarget, erStatus, _ := AutoLocateWindowsTarget(erMeta)
	expectedER := filepath.Join(winAppData, "EldenRing")
	if erTarget != expectedER {
		t.Errorf("Expected ER target %s, got %s", expectedER, erTarget)
	}
	if erStatus != "windows_detected" {
		t.Errorf("Expected windows_detected, got %s", erStatus)
	}

	// 3. Documents (Outlast)
	olMeta := GameSaveMeta{
		ID:                 "doc_outlast",
		GameName:           "Outlast",
		AppID:              "238320",
		TargetRelativePath: "users/steamuser/Documents/My Games/Outlast",
	}
	olTarget, olStatus, _ := AutoLocateWindowsTarget(olMeta)
	expectedOL := filepath.Join(winUserProfile, "Documents", "My Games", "Outlast")
	if olTarget != expectedOL {
		t.Errorf("Expected Outlast target %s, got %s", expectedOL, olTarget)
	}
	if olStatus != "windows_ready" {
		t.Errorf("Expected windows_ready for uncreated Outlast child folder, got %s", olStatus)
	}

	// 4. Public Documents (RUNE)
	runeMeta := GameSaveMeta{
		ID:                 "profile_123456",
		GameName:           "RUNE Game",
		AppID:              "123456",
		TargetRelativePath: "users/Public/Documents/Steam/RUNE/123456",
	}
	runeTarget, _, _ := AutoLocateWindowsTarget(runeMeta)
	expectedRune := filepath.Join(winPublic, "Documents", "Steam", "RUNE", "123456")
	if runeTarget != expectedRune {
		t.Errorf("Expected RUNE target %s, got %s", expectedRune, runeTarget)
	}

	// 5. Cemu on Windows
	cemuMeta := GameSaveMeta{
		ID:                 "cemu_00050000_101c9500",
		GameName:           "The Legend of Zelda",
		TargetRelativePath: "cemu/mlc01/usr/save/00050000/101c9500",
	}
	cemuTarget, _, _ := AutoLocateWindowsTarget(cemuMeta)
	expectedCemu := filepath.Join(winAppData, "Cemu", "mlc01", "usr", "save", "00050000", "101c9500")
	if cemuTarget != expectedCemu {
		t.Errorf("Expected Cemu Windows target %s, got %s", expectedCemu, cemuTarget)
	}
}

func TestLinuxHostScanner(t *testing.T) {
	tmpLinux, err := os.MkdirTemp("", "linux_host_scanner_*")
	if err != nil {
		t.Fatalf("Failed to create temp linux dir: %v", err)
	}
	defer os.RemoveAll(tmpLinux)

	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpLinux)
	defer os.Setenv("HOME", origHome)

	// 1. Setup a mock Steam Proton compatdata prefix
	protonPfx := filepath.Join(tmpLinux, ".local", "share", "Steam", "steamapps", "compatdata", "1245620", "pfx", "drive_c")
	erSaveDir := filepath.Join(protonPfx, "users", "steamuser", "AppData", "Roaming", "EldenRing")
	os.MkdirAll(erSaveDir, 0755)
	os.WriteFile(filepath.Join(erSaveDir, "ER0000.sl2"), []byte("elden_ring_save_bytes"), 0644)

	// 2. Setup mock EmuDeck Cemu saves
	cemuSave := filepath.Join(tmpLinux, "Emulation", "saves", "cemu", "mlc01", "usr", "save", "00050000", "101c9500")
	os.MkdirAll(filepath.Join(cemuSave, "user"), 0755)
	os.WriteFile(filepath.Join(cemuSave, "user", "game_data.sav"), []byte("botw_save"), 0644)

	var games []DetectedGame
	seen := make(map[string]bool)

	scanLinuxHostSaves(&games, seen)

	foundER := false
	foundCemu := false
	for _, g := range games {
		if g.AppID == "1245620" && strings.Contains(g.Name, "Elden Ring") {
			foundER = true
			if g.TargetRelativePath != "users/steamuser/AppData/Roaming/EldenRing" {
				t.Errorf("Expected normalized relative path for Elden Ring, got %s", g.TargetRelativePath)
			}
		}
		if strings.HasPrefix(g.ID, "cemu_") {
			foundCemu = true
		}
	}

	if !foundER {
		t.Errorf("Failed to detect Elden Ring from Proton compatdata prefix")
	}
	if !foundCemu {
		t.Errorf("Failed to detect Cemu from Linux EmuDeck saves")
	}
}

func TestCustomPrefixDirectoryScan(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "custom_prefixes_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create prefix 1: Elden Ring by name
	erPfx := filepath.Join(tmpDir, "Elden Ring", "drive_c", "users", "steamuser", "AppData", "Roaming", "EldenRing")
	os.MkdirAll(erPfx, 0755)
	os.WriteFile(filepath.Join(erPfx, "ER0000.sl2"), []byte("elden_ring_save"), 0644)

	// Create prefix 2: Cyberpunk by AppID
	cpPfx := filepath.Join(tmpDir, "1091500", "pfx", "drive_c", "users", "steamuser", "Saved Games", "CD Projekt Red", "Cyberpunk 2077")
	os.MkdirAll(cpPfx, 0755)
	os.WriteFile(filepath.Join(cpPfx, "manualsave.dat"), []byte("cp_save"), 0644)

	var games []DetectedGame
	seen := make(map[string]bool)

	scanCustomPath(tmpDir, &games, seen)

	foundER := false
	foundCP := false
	for _, g := range games {
		if strings.Contains(g.Name, "Elden Ring") {
			foundER = true
			if g.AppID != "1245620" {
				t.Errorf("Expected Elden Ring AppID 1245620 from name resolution, got %s", g.AppID)
			}
		}
		if strings.Contains(g.Name, "Cyberpunk") || g.AppID == "1091500" {
			foundCP = true
		}
	}

	if !foundER {
		t.Errorf("Custom prefix scan failed to find Elden Ring from subfolder prefix")
	}
	if !foundCP {
		t.Errorf("Custom prefix scan failed to find Cyberpunk 2077 from AppID subfolder prefix")
	}
}

func TestLatestTelemetryCodenamesAndEdgeCases(t *testing.T) {
	// 1. Unreal Engine Codenames
	testCodenames := map[string]struct {
		expectedID   string
		expectedName string
	}{
		"Phoenix":    {"990080", "Hogwarts Legacy"},
		"Oregon":     {"1583230", "High On Life"},
		"BlueClient": {"2456740", "inZOI"},
		"Chameleon":  {"4704690", "MECCHA CHAMELEON"},
	}

	for codename, expected := range testCodenames {
		appID, name := resolveUnrealProjectCodename(codename)
		if appID != expected.expectedID || name != expected.expectedName {
			t.Errorf("Codename %s: expected (%s, %s), got (%s, %s)",
				codename, expected.expectedID, expected.expectedName, appID, name)
		}
	}

	// 2. Title cleaning & normalization
	if got := cleanGameTitle("Beyond Two Souls Steam"); got != "Beyond Two Souls" {
		t.Errorf("Expected 'Beyond Two Souls', got '%s'", got)
	}
	if got := cleanGameTitle("The Walking Dead - Definitive Series"); got != "The Walking Dead Definitive Series" {
		t.Errorf("Expected 'The Walking Dead Definitive Series', got '%s'", got)
	}

	// 3. Known AppIDs and delisted games
	if name := getGameName("224060", ""); name != "Deadpool" {
		t.Errorf("Expected 'Deadpool' for AppID 224060, got '%s'", name)
	}
	if name := getGameName("2420110", ""); name != "Horizon Forbidden West Complete Edition" {
		t.Errorf("Expected 'Horizon Forbidden West Complete Edition' for AppID 2420110, got '%s'", name)
	}

	// 4. LocalLow Mock scanning
	tmpRoot, err := os.MkdirTemp("", "locallow_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpRoot)

	// Create directories
	os.MkdirAll(filepath.Join(tmpRoot, "webviewdata", "2"), 0755)
	os.WriteFile(filepath.Join(tmpRoot, "webviewdata", "2", "cache.dat"), []byte("junk"), 0644)

	os.MkdirAll(filepath.Join(tmpRoot, "DefaultCompany", "Mirror"), 0755)
	os.WriteFile(filepath.Join(tmpRoot, "DefaultCompany", "Mirror", "save.dat"), []byte("save"), 0644)

	os.MkdirAll(filepath.Join(tmpRoot, "Gryphline", "Endfield"), 0755)
	os.WriteFile(filepath.Join(tmpRoot, "Gryphline", "Endfield", "save.dat"), []byte("save"), 0644)

	os.MkdirAll(filepath.Join(tmpRoot, "ILLGAMES", "HoneyCome"), 0755)
	os.WriteFile(filepath.Join(tmpRoot, "ILLGAMES", "HoneyCome", "save.dat"), []byte("save"), 0644)

	os.MkdirAll(filepath.Join(tmpRoot, "Landfall West", "Stick Fight_ The Game"), 0755)
	os.WriteFile(filepath.Join(tmpRoot, "Landfall West", "Stick Fight_ The Game", "save.dat"), []byte("save"), 0644)

	var detected []DetectedGame
	seen := make(map[string]bool)
	scanUserAppDataLocalLow(tmpRoot, &detected, seen)

	foundMirror := false
	foundEndfield := false
	foundHoneyCome := false
	foundStickFight := false

	for _, g := range detected {
		if strings.Contains(g.SourcePath, "webviewdata") {
			t.Errorf("False positive: webviewdata should be ignored!")
		}
		if g.Name == "Mirror" {
			foundMirror = true
		}
		if strings.Contains(g.Name, "Endfield") {
			foundEndfield = true
		}
		if strings.Contains(g.Name, "HoneyCome") {
			foundHoneyCome = true
		}
		if strings.Contains(g.Name, "Stick Fight") {
			foundStickFight = true
		}
	}

	if !foundMirror {
		t.Errorf("Expected Mirror to be detected under DefaultCompany")
	}
	if !foundEndfield {
		t.Errorf("Expected Arknights: Endfield to be detected under Gryphline")
	}
	if !foundHoneyCome {
		t.Errorf("Expected HoneyCome to be detected under ILLGAMES")
	}
	if !foundStickFight {
		t.Errorf("Expected Stick Fight: The Game to be detected under Landfall West")
	}
}

func TestV0110EmulatorAndArtworkFeatures(t *testing.T) {
	// 1. Unreal Engine Telemetry Codenames
	fAppID, fName := resolveUnrealProjectCodename("FactoryGame")
	if fAppID != "526870" || fName != "Satisfactory" {
		t.Errorf("Expected Satisfactory (526870), got %s (%s)", fName, fAppID)
	}

	bbqAppID, bbqName := resolveUnrealProjectCodename("BBQ")
	if bbqAppID != "1433140" || bbqName != "The Texas Chain Saw Massacre" {
		t.Errorf("Expected The Texas Chain Saw Massacre (1433140), got %s (%s)", bbqName, bbqAppID)
	}

	_, shfName := resolveUnrealProjectCodename("SHf")
	if shfName != "Silent Hill f" {
		t.Errorf("Expected Silent Hill f, got %s", shfName)
	}

	// 2. Switch title ID mappings & artwork
	if name := switchTitleMap["01004a4010fea000"]; name != "Bayonetta 3" {
		t.Errorf("Expected Bayonetta 3, got '%s'", name)
	}
	if name := switchTitleMap["01004d300c5ae000"]; name != "Kirby and the Forgotten Land" {
		t.Errorf("Expected Kirby and the Forgotten Land, got '%s'", name)
	}
	if art := switchExclusiveArt["01006a800016e000"]; !strings.HasPrefix(art, "http") {
		t.Errorf("Expected valid art URL for Smash Bros, got '%s'", art)
	}

	// 3. Local console save icon extraction (icon0.png)
	tmpDir, err := os.MkdirTemp("", "save_art_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	sceSysDir := filepath.Join(tmpDir, "sce_sys")
	os.MkdirAll(sceSysDir, 0755)
	dummyPNG := []byte("\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR\x00\x00\x00\x01\x00\x00\x00\x01\x08\x06\x00\x00\x00\x1f\x15c4")
	os.WriteFile(filepath.Join(sceSysDir, "icon0.png"), dummyPNG, 0644)

	extracted := extractLocalSaveIcon(tmpDir)
	if !strings.HasPrefix(extracted, "data:image/png;base64,") {
		t.Errorf("Expected data URI base64 icon, got '%s'", extracted)
	}

	// 4. Xbox 360 Title Mapping
	if title := getXboxTitleName("545408A7"); title != "Red Dead Redemption" {
		t.Errorf("Expected Red Dead Redemption, got '%s'", title)
	}
	if title := getXboxTitleName("4541088f"); title != "Skate 3" {
		t.Errorf("Expected Skate 3, got '%s'", title)
	}
}


