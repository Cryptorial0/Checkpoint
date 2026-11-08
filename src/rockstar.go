package main

import (
	"io"
	"os"
	"path/filepath"
	"strings"
)

// HandleRockstarGamesAlignment synchronizes save files across all profile IDs,
// emulator storage paths (Goldberg SocialClub Emu, .1911/EMPRESS), and Wine user directories.
func HandleRockstarGamesAlignment(targetDir, appID string) {
	isRDR2 := appID == "1174180" || strings.Contains(strings.ToLower(targetDir), "red dead redemption 2")
	isGTAV := appID == "271590" || appID == "3240220" || strings.Contains(strings.ToLower(targetDir), "gtav") || strings.Contains(strings.ToLower(targetDir), "gta v")

	if !isRDR2 && !isGTAV {
		return
	}

	gameName := "Red Dead Redemption 2"
	if isGTAV {
		gameName = "Grand Theft Auto V"
	}
	Log("Executing Rockstar Games alignment for %s in: %s", gameName, targetDir)

	// Collect all save files from targetDir
	saveFiles := make(map[string]string) // filename -> full source path
	sourceProfiles := make(map[string]bool)

	_ = filepath.Walk(targetDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		name := info.Name()
		nameLow := strings.ToLower(name)

		isSave := false
		if isRDR2 {
			if strings.HasPrefix(name, "SRDR") || nameLow == "player" || nameLow == "profilesettings" || nameLow == "cloudsaves.dat" {
				isSave = true
			}
		} else if isGTAV {
			if strings.HasPrefix(name, "SGTA5") || strings.HasPrefix(nameLow, "control") || nameLow == "cfg.dat" || nameLow == "pc_settings.bin" {
				isSave = true
			}
		}

		if isSave {
			saveFiles[name] = path
			parent := filepath.Base(filepath.Dir(path))
			if parent != "." && !strings.EqualFold(parent, "profiles") && !strings.EqualFold(parent, "red dead redemption 2") && !strings.EqualFold(parent, "gtav") && !strings.EqualFold(parent, "gtav enhanced") {
				sourceProfiles[parent] = true
			}
		}
		return nil
	})

	if len(saveFiles) == 0 {
		Log("Rockstar Games alignment: no save files found in %s", targetDir)
		return
	}

	Log("Rockstar Games alignment: found %d save files across %d source profiles", len(saveFiles), len(sourceProfiles))

	// Find drive_c root if this is inside a Wine/Proton prefix
	driveC := findDriveCRoot(targetDir)

	// Determine common profile IDs (Goldberg default, EMPRESS default, generic, plus source profiles)
	profileIDs := []string{"0F74F4C4", "12345678", "00000000", "11111111"}
	for sp := range sourceProfiles {
		profileIDs = append(profileIDs, sp)
	}

	// 1. Sync across all existing profile directories in targetDir/Profiles
	profilesDir := filepath.Join(targetDir, "Profiles")
	if _, err := os.Stat(profilesDir); err == nil {
		syncToProfilesFolder(profilesDir, saveFiles, profileIDs)
	}

	// If not inside a Wine prefix, we're done with local profiles sync
	if driveC == "" {
		return
	}

	// 2. Multi-destination seeding across all user directories in drive_c/users
	userDirs := getPrefixUserDirs(driveC)
	for _, uDir := range userDirs {
		if isRDR2 {
			// A. Documents / Rockstar Games / Red Dead Redemption 2 / Profiles
			docProfiles := filepath.Join(uDir, "Documents", "Rockstar Games", "Red Dead Redemption 2", "Profiles")
			syncToProfilesFolder(docProfiles, saveFiles, profileIDs)

			// B. Goldberg SocialClub Emu Saves / RDR2
			goldbergRDR2 := filepath.Join(uDir, "AppData", "Roaming", "Goldberg SocialClub Emu Saves", "RDR2")
			syncToProfilesFolder(goldbergRDR2, saveFiles, profileIDs)

			// C. EMPRESS / .1911 Crack directory
			empress1911 := filepath.Join(uDir, "AppData", "Roaming", ".1911", "Red Dead Redemption 2", "profile")
			syncSaveFiles(empress1911, saveFiles)

			// D. Goldberg SteamEmu Saves / 1174180
			goldbergSteam := filepath.Join(uDir, "AppData", "Roaming", "Goldberg SteamEmu Saves", "1174180")
			if stat, err := os.Stat(filepath.Dir(goldbergSteam)); err == nil && stat.IsDir() {
				syncSaveFiles(goldbergSteam, saveFiles)
			}
		} else if isGTAV {
			// A. Documents / Rockstar Games / GTAV / Profiles
			docGTAV := filepath.Join(uDir, "Documents", "Rockstar Games", "GTAV", "Profiles")
			syncToProfilesFolder(docGTAV, saveFiles, profileIDs)

			docGTAVEnhanced := filepath.Join(uDir, "Documents", "Rockstar Games", "GTAV Enhanced", "Profiles")
			syncToProfilesFolder(docGTAVEnhanced, saveFiles, profileIDs)

			// B. Goldberg SocialClub Emu Saves / GTA V
			goldbergGTAV := filepath.Join(uDir, "AppData", "Roaming", "Goldberg SocialClub Emu Saves", "GTA V")
			syncToProfilesFolder(goldbergGTAV, saveFiles, profileIDs)

			// C. EMPRESS / .1911 Crack directory
			empress1911 := filepath.Join(uDir, "AppData", "Roaming", ".1911", "GTAV", "profile")
			syncSaveFiles(empress1911, saveFiles)
		}
	}
}

// findDriveCRoot walks upward until it finds a directory named drive_c
func findDriveCRoot(path string) string {
	curr := filepath.Clean(path)
	for {
		if strings.EqualFold(filepath.Base(curr), "drive_c") {
			return curr
		}
		parent := filepath.Dir(curr)
		if parent == curr || parent == "." || parent == "/" {
			break
		}
		curr = parent
	}
	return ""
}

// getPrefixUserDirs returns all user profile directories inside drive_c/users
func getPrefixUserDirs(driveC string) []string {
	usersDir := filepath.Join(driveC, "users")
	entries, err := os.ReadDir(usersDir)
	if err != nil {
		return []string{filepath.Join(usersDir, "steamuser")}
	}

	var dirs []string
	hasSteamUser := false
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		if strings.EqualFold(name, "public") || strings.EqualFold(name, "default") || strings.EqualFold(name, "all users") {
			continue
		}
		if strings.EqualFold(name, "steamuser") {
			hasSteamUser = true
		}
		dirs = append(dirs, filepath.Join(usersDir, name))
	}

	if !hasSteamUser {
		dirs = append(dirs, filepath.Join(usersDir, "steamuser"))
	}
	return dirs
}

// syncToProfilesFolder copies save files into all existing subdirectories of profilesDir,
// and ensures default profile directories are also seeded.
func syncToProfilesFolder(profilesDir string, saveFiles map[string]string, defaultProfiles []string) {
	_ = os.MkdirAll(profilesDir, 0755)

	targetDirs := make(map[string]bool)

	// Check any existing profile directories created by games or emulators
	if entries, err := os.ReadDir(profilesDir); err == nil {
		for _, e := range entries {
			if e.IsDir() {
				targetDirs[filepath.Join(profilesDir, e.Name())] = true
			}
		}
	}

	// Always ensure common profile IDs exist
	for _, pid := range defaultProfiles {
		targetDirs[filepath.Join(profilesDir, pid)] = true
	}

	for tDir := range targetDirs {
		syncSaveFiles(tDir, saveFiles)
	}
}

// syncSaveFiles copies all save files into destDir
func syncSaveFiles(destDir string, saveFiles map[string]string) {
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return
	}
	for filename, srcPath := range saveFiles {
		destPath := filepath.Join(destDir, filename)
		if srcPath == destPath {
			continue
		}
		_ = copyFileRaw(srcPath, destPath)
	}
}

// copyFileRaw copies bytes from src to dst
func copyFileRaw(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Sync()
}
