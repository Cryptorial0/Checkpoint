package main

import (
	"archive/tar"
	"archive/zip"
	"bufio"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

type RestoreReport struct {
	GameID         string `json:"game_id"`
	GameName       string `json:"game_name"`
	TargetResolved string `json:"target_resolved"`
	Success        bool   `json:"success"`
	Error          string `json:"error,omitempty"`
}

// InspectVault reads an index.json from either a folder or a zip archive
func InspectVault(sourcePath string) (*VaultManifest, error) {
	stat, err := os.Stat(sourcePath)
	if err != nil {
		return nil, fmt.Errorf("source path does not exist: %w", err)
	}

	if stat.IsDir() {
		idxFile := filepath.Join(sourcePath, "index.json")
		data, err := os.ReadFile(idxFile)
		if err != nil {
			return nil, fmt.Errorf("could not find index.json in folder: %w", err)
		}
		var manifest VaultManifest
		if err := json.Unmarshal(data, &manifest); err != nil {
			return nil, err
		}
		enrichManifest(&manifest)
		return &manifest, nil
	}

	// Must be a zip file
	zr, err := zip.OpenReader(sourcePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open zip archive: %w", err)
	}
	defer zr.Close()

	for _, f := range zr.File {
		if filepath.Base(f.Name) == "index.json" {
			rc, err := f.Open()
			if err != nil {
				return nil, err
			}
			defer rc.Close()
			data, err := io.ReadAll(rc)
			if err != nil {
				return nil, err
			}
			var manifest VaultManifest
			if err := json.Unmarshal(data, &manifest); err != nil {
				return nil, err
			}
			enrichManifest(&manifest)
			return &manifest, nil
		}
	}

	return nil, fmt.Errorf("archive does not contain index.json")
}

func enrichManifest(manifest *VaultManifest) {
	var wg sync.WaitGroup
	// Concurrency-limited worker pool for fast metadata enrichment
	sem := make(chan struct{}, 8)

	for i := range manifest.Games {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			g := &manifest.Games[idx]
			if g.OriginalSourcePath == "" && g.TargetRelativePath != "" {
				g.OriginalSourcePath = g.TargetRelativePath
			}

			// Artwork and ProtonDB tier lookup
			if g.AppID != "" {
				if g.IconURL == "" {
					g.IconURL = getSteamIconURL(g.AppID)
				}
				if g.ProtonTier == "" {
					g.ProtonTier = fetchProtonDBTier(g.AppID)
				}
			}

			// Precompute paths for all launchers (Hydra, Heroic, Lutris, Steam)
			g.LauncherPaths = make(map[string]string)
			g.LauncherStatuses = make(map[string]string)

			computeAllLauncherTargets(g)

			// Default to Auto resolution (Hydra -> Heroic -> Lutris for offline games)
			autoPath, statusKey, statusText, chosenLauncher := AutoLocateLinuxTarget(*g)
			g.TargetStatus = statusKey
			g.TargetStatusText = statusText
			g.SelectedLauncher = chosenLauncher
			if autoPath != "" {
				g.SuggestedLinuxPath = autoPath
			}
		}(i)
	}
	wg.Wait()

	// Detect online games from local system
	onlineFound := ScanOnlineGames()
	seenOnline := make(map[string]bool)
	for _, o := range onlineFound {
		seenOnline[o.Name] = true
		manifest.OnlineGames = append(manifest.OnlineGames, o)
	}

	// Also check if any games inside the inspected vault match knownOnlineGames
	for _, g := range knownOnlineGames {
		if seenOnline[g.Name] {
			continue
		}
		for _, vg := range manifest.Games {
			matched := false
			if strings.EqualFold(vg.GameName, g.Name) {
				matched = true
			} else {
				for _, sig := range g.Signatures {
					if vg.AppID != "" && sig == "appmanifest_"+vg.AppID+".acf" {
						matched = true
						break
					}
					if matchesSignatureWord(vg.GameName, sig) {
						matched = true
						break
					}
				}
			}
			if matched {
				seenOnline[g.Name] = true
				manifest.OnlineGames = append(manifest.OnlineGames, OnlineGame{
					Name:          g.Name,
					Status:        g.Status,
					AntiCheats:    g.AntiCheats,
					InstallSource: fmt.Sprintf("Vault Archive Title (%s)", vg.GameName),
				})
				break
			}
		}
	}
}

// computeAllLauncherTargets populates LauncherPaths and LauncherStatuses for all platforms
func computeAllLauncherTargets(g *GameSaveMeta) {
	relPath := g.TargetRelativePath

	// 1. Hydra Launcher (Uses SteamID / AppID)
	if g.AppID != "" {
		if hydraC := LocateHydraPrefix(g.AppID); hydraC != "" {
			g.LauncherPaths["hydra"] = resolveCaseInsensitivePath(hydraC, relPath)
			g.LauncherStatuses["hydra"] = "hydra_detected"
		} else {
			g.LauncherPaths["hydra"] = ExpectedHydraPath(g.AppID, relPath)
			g.LauncherStatuses["hydra"] = "not_initialized"
		}
	}

	// 2. Heroic Games Launcher (Uses custom name / title)
	if heroicC := LocateHeroicPrefix(g.GameName, g.AppID); heroicC != "" {
		g.LauncherPaths["heroic"] = resolveCaseInsensitivePath(heroicC, relPath)
		g.LauncherStatuses["heroic"] = "heroic_detected"
	} else {
		g.LauncherPaths["heroic"] = ExpectedHeroicPath(g.GameName, relPath)
		g.LauncherStatuses["heroic"] = "not_initialized"
	}

	// 3. Lutris (Uses game name / slug)
	if lutrisC := LocateLutrisPrefix(g.GameName, g.AppID); lutrisC != "" {
		g.LauncherPaths["lutris"] = resolveCaseInsensitivePath(lutrisC, relPath)
		g.LauncherStatuses["lutris"] = "lutris_detected"
	} else {
		g.LauncherPaths["lutris"] = ExpectedLutrisPath(g.GameName, relPath)
		g.LauncherStatuses["lutris"] = "not_initialized"
	}

	// 4. Steam Proton (Optional fallback or for non-Steam added games)
	if g.AppID != "" {
		if steamC := LocateProtonPrefix(g.AppID); steamC != "" {
			g.LauncherPaths["steam"] = resolveCaseInsensitivePath(steamC, relPath)
			g.LauncherStatuses["steam"] = "proton_detected"
		} else {
			g.LauncherPaths["steam"] = ExpectedSteamPath(g.AppID, relPath)
			g.LauncherStatuses["steam"] = "not_initialized"
		}
	}
}

// ValidateWinePrefix verifies if candidatePath is an actual initialized Wine/Proton prefix
func ValidateWinePrefix(candidatePath string) (driveC string, isValid bool, errDesc string) {
	if candidatePath == "" {
		return "", false, "Path is empty"
	}
	clean := filepath.Clean(candidatePath)
	fi, err := os.Stat(clean)
	if err != nil || !fi.IsDir() {
		return "", false, "Selected path does not exist or is not a directory"
	}

	// Find drive_c
	if strings.EqualFold(filepath.Base(clean), "drive_c") {
		driveC = clean
	} else if stat, err := os.Stat(filepath.Join(clean, "drive_c")); err == nil && stat.IsDir() {
		driveC = filepath.Join(clean, "drive_c")
	} else if stat, err := os.Stat(filepath.Join(clean, "pfx", "drive_c")); err == nil && stat.IsDir() {
		driveC = filepath.Join(clean, "pfx", "drive_c")
	}

	if driveC == "" {
		return "", false, "Folder is not a Wine/Proton prefix (missing drive_c directory)"
	}

	// Check if prefix is initialized: must have users directory or registry files
	usersDir := filepath.Join(driveC, "users")
	prefixRoot := filepath.Dir(driveC)
	if filepath.Base(prefixRoot) == "pfx" {
		prefixRoot = filepath.Dir(prefixRoot)
	}

	hasUsers := false
	if uStat, err := os.Stat(usersDir); err == nil && uStat.IsDir() {
		hasUsers = true
	}

	hasReg := false
	for _, regFile := range []string{
		filepath.Join(prefixRoot, "system.reg"),
		filepath.Join(prefixRoot, "user.reg"),
		filepath.Join(filepath.Dir(driveC), "system.reg"),
		filepath.Join(filepath.Dir(driveC), "user.reg"),
	} {
		if _, err := os.Stat(regFile); err == nil {
			hasReg = true
			break
		}
	}

	if !hasUsers && !hasReg {
		return driveC, false, "Prefix directory exists but is uninitialized (missing users/registry). Launch the game once in your launcher first."
	}

	return driveC, true, ""
}

// LocateHydraPrefix finds Hydra Launcher Wine prefix (uses SteamID / AppID)
func LocateHydraPrefix(appID string) string {
	if appID == "" {
		return ""
	}
	home := os.Getenv("HOME")
	if home == "" {
		return ""
	}

	bases := []string{
		filepath.Join(home, ".config", "hydralauncher", "wine-prefixes"),
		filepath.Join(home, ".config", "hydra", "wine-prefixes"),
		filepath.Join(home, ".local", "share", "hydralauncher", "wine-prefixes"),
		filepath.Join(home, ".local", "share", "hydra", "wine-prefixes"),
		filepath.Join(home, ".var", "app", "io.github.hydralauncher.Hydra", "config", "hydralauncher", "wine-prefixes"),
		filepath.Join(home, ".var", "app", "io.github.hydralauncher.Hydra", "data", "hydralauncher", "wine-prefixes"),
	}

	subnames := []string{
		appID,
		"umu-" + appID,
	}

	for _, base := range bases {
		for _, sub := range subnames {
			cand1 := filepath.Join(base, sub, "pfx", "drive_c")
			if stat, err := os.Stat(cand1); err == nil && stat.IsDir() {
				return cand1
			}
			cand2 := filepath.Join(base, sub, "drive_c")
			if stat, err := os.Stat(cand2); err == nil && stat.IsDir() {
				return cand2
			}
		}
	}
	return ""
}

func ExpectedHydraPath(appID, relPath string) string {
	home := os.Getenv("HOME")
	if home == "" || appID == "" {
		return ""
	}
	base := filepath.Join(home, ".config", "hydralauncher", "wine-prefixes", appID, "drive_c")
	return filepath.Join(base, filepath.FromSlash(relPath))
}

// LocateHeroicPrefix finds Heroic prefix drive_c (uses custom name / title)
func LocateHeroicPrefix(gameName, appID string) string {
	home := os.Getenv("HOME")
	if home == "" {
		return ""
	}

	names := []string{strings.TrimSpace(gameName)}
	if appID != "" {
		names = append(names, appID)
	}

	heroicBases := []string{
		filepath.Join(home, ".config", "heroic", "prefixes"),
		filepath.Join(home, ".var", "app", "com.heroicgameslauncher.hgl", "config", "heroic", "prefixes"),
		filepath.Join(home, "Games", "Heroic", "prefixes"),
		filepath.Join(home, "Games", "heroic", "prefixes"),
		filepath.Join(home, "Games", "Heroic"),
		filepath.Join(home, "Games", "heroic"),
	}

	for _, base := range heroicBases {
		for _, name := range names {
			if name == "" {
				continue
			}
			cand1 := filepath.Join(base, name, "pfx", "drive_c")
			if stat, err := os.Stat(cand1); err == nil && stat.IsDir() {
				return cand1
			}
			cand2 := filepath.Join(base, name, "drive_c")
			if stat, err := os.Stat(cand2); err == nil && stat.IsDir() {
				return cand2
			}
		}
	}
	return ""
}

func ExpectedHeroicPath(gameName, relPath string) string {
	home := os.Getenv("HOME")
	clean := strings.TrimSpace(gameName)
	if home == "" || clean == "" {
		return ""
	}
	base := filepath.Join(home, ".config", "heroic", "prefixes", clean, "pfx", "drive_c")
	return filepath.Join(base, filepath.FromSlash(relPath))
}

// LocateLutrisPrefix finds Lutris prefix (uses game title / slug)
func LocateLutrisPrefix(gameName, appID string) string {
	home := os.Getenv("HOME")
	if home == "" {
		return ""
	}

	cleanName := strings.TrimSpace(gameName)
	slug := strings.ToLower(strings.ReplaceAll(cleanName, " ", "-"))
	names := []string{cleanName, slug}
	if appID != "" {
		names = append(names, appID)
	}

	lutrisBases := []string{
		filepath.Join(home, "Games"),
		filepath.Join(home, "Games", "lutris"),
		filepath.Join(home, ".local", "share", "lutris", "prefixes"),
	}

	for _, base := range lutrisBases {
		for _, name := range names {
			cand1 := filepath.Join(base, name, "drive_c")
			if stat, err := os.Stat(cand1); err == nil && stat.IsDir() {
				return cand1
			}
			cand2 := filepath.Join(base, name, "pfx", "drive_c")
			if stat, err := os.Stat(cand2); err == nil && stat.IsDir() {
				return cand2
			}
		}
	}
	return ""
}

func ExpectedLutrisPath(gameName, relPath string) string {
	home := os.Getenv("HOME")
	clean := strings.TrimSpace(gameName)
	if home == "" || clean == "" {
		return ""
	}
	base := filepath.Join(home, "Games", clean, "drive_c")
	return filepath.Join(base, filepath.FromSlash(relPath))
}

// LocatePortProtonPrefix finds PortProton prefixes
func LocatePortProtonPrefix(gameName, appID string) string {
	home := os.Getenv("HOME")
	if home == "" {
		return ""
	}
	names := []string{strings.TrimSpace(gameName), appID}
	bases := []string{
		filepath.Join(home, "PortWINE", "PortProton", "data", "prefixes"),
		filepath.Join(home, ".local", "share", "PortProton", "prefixes"),
	}
	for _, base := range bases {
		for _, name := range names {
			if name == "" {
				continue
			}
			cand := filepath.Join(base, name, "drive_c")
			if stat, err := os.Stat(cand); err == nil && stat.IsDir() {
				return cand
			}
		}
	}
	return ""
}

// LocateBottlesPrefix finds a matching Bottles prefix
func LocateBottlesPrefix(gameName string) string {
	home := os.Getenv("HOME")
	if home == "" {
		return ""
	}

	bottleRoots := []string{
		filepath.Join(home, ".var", "app", "com.usebottles.bottles", "data", "bottles", "bottles"),
		filepath.Join(home, ".local", "share", "bottles", "bottles"),
	}

	cleanName := strings.ToLower(strings.TrimSpace(gameName))
	for _, root := range bottleRoots {
		entries, err := os.ReadDir(root)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if e.IsDir() {
				if strings.Contains(strings.ToLower(e.Name()), cleanName) || strings.Contains(cleanName, strings.ToLower(e.Name())) {
					cand := filepath.Join(root, e.Name(), "drive_c")
					if stat, err := os.Stat(cand); err == nil && stat.IsDir() {
						return cand
					}
				}
			}
		}
	}
	return ""
}

// findSteamLibraryFolders returns all unique Steam library root folders across drives
func findSteamLibraryFolders() []string {
	home := os.Getenv("HOME")
	if home == "" {
		return nil
	}

	potentialRoots := []string{
		filepath.Join(home, ".local", "share", "Steam"),
		filepath.Join(home, ".steam", "steam"),
		filepath.Join(home, ".var", "app", "com.valvesoftware.Steam", ".local", "share", "Steam"),
		filepath.Join(home, ".var", "app", "com.valvesoftware.Steam", ".steam", "steam"),
	}

	seen := make(map[string]bool)
	var libraries []string

	addLib := func(dir string) {
		clean := filepath.Clean(dir)
		if clean == "" || clean == "." {
			return
		}
		if !seen[clean] {
			if stat, err := os.Stat(clean); err == nil && stat.IsDir() {
				seen[clean] = true
				libraries = append(libraries, clean)
			}
		}
	}

	rePath := regexp.MustCompile(`"path"\s+"([^"]+)"`)

	for _, root := range potentialRoots {
		addLib(root)

		vdfPath := filepath.Join(root, "steamapps", "libraryfolders.vdf")
		file, err := os.Open(vdfPath)
		if err != nil {
			continue
		}
		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			matches := rePath.FindStringSubmatch(scanner.Text())
			if len(matches) == 2 {
				addLib(matches[1])
			}
		}
		file.Close()
	}

	return libraries
}

// LocateProtonPrefix finds Steam Proton prefix drive_c
func LocateProtonPrefix(appID string) string {
	if appID == "" {
		return ""
	}

	libraries := findSteamLibraryFolders()
	for _, lib := range libraries {
		cand := filepath.Join(lib, "steamapps", "compatdata", appID, "pfx", "drive_c")
		if stat, err := os.Stat(cand); err == nil && stat.IsDir() {
			return cand
		}
	}

	home := os.Getenv("HOME")
	if home != "" {
		cands := []string{
			filepath.Join(home, ".steam", "steam", "steamapps", "compatdata", appID, "pfx", "drive_c"),
			filepath.Join(home, ".local", "share", "Steam", "steamapps", "compatdata", appID, "pfx", "drive_c"),
			filepath.Join(home, ".var", "app", "com.valvesoftware.Steam", ".local", "share", "Steam", "steamapps", "compatdata", appID, "pfx", "drive_c"),
		}
		for _, c := range cands {
			if stat, err := os.Stat(c); err == nil && stat.IsDir() {
				return c
			}
		}
	}

	return ""
}

func ExpectedSteamPath(appID, relPath string) string {
	home := os.Getenv("HOME")
	if home == "" || appID == "" {
		return ""
	}
	base := filepath.Join(home, ".steam", "steam", "steamapps", "compatdata", appID, "pfx", "drive_c")
	return filepath.Join(base, filepath.FromSlash(relPath))
}

// LocateConsoleEmulatorPath checks console emulator directories including EmuDeck, RetroDECK, Flatpaks, and native installs
func LocateConsoleEmulatorPath(meta GameSaveMeta) (string, string) {
	home := os.Getenv("HOME")
	if home == "" {
		return "", ""
	}

	cLow := strings.ToLower(meta.StorageType)
	idLow := strings.ToLower(meta.ID)

	// EmuDeck & RetroDECK base paths
	emuDeckRoots := []string{filepath.Join(home, "Emulation", "saves")}
	if matches, err := filepath.Glob("/run/media/*/Emulation/saves"); err == nil {
		emuDeckRoots = append(emuDeckRoots, matches...)
	}
	if matches, err := filepath.Glob("/run/media/*/*/Emulation/saves"); err == nil {
		emuDeckRoots = append(emuDeckRoots, matches...)
	}
	if matches, err := filepath.Glob("/media/*/Emulation/saves"); err == nil {
		emuDeckRoots = append(emuDeckRoots, matches...)
	}
	emuDeckSaves := emuDeckRoots[0]
	retroDeckSaves := filepath.Join(home, "retrodeck", "saves")

	type candidate struct {
		path  string
		label string
	}

	// 1. PlayStation 4 (ShadPS4 / bbport)
	if strings.Contains(cLow, "ps4") || strings.HasPrefix(idLow, "ps4_") {
		var cands []candidate
		for _, ed := range emuDeckRoots {
			cands = append(cands, candidate{filepath.Join(ed, "shadps4"), "EmuDeck (ShadPS4)"})
		}
		cands = append(cands, []candidate{
			{filepath.Join(home, ".local", "share", "shadPS4"), "Auto-Detected ShadPS4"},
			{filepath.Join(home, ".local", "share", "shadps4"), "Auto-Detected ShadPS4"},
			{filepath.Join(home, ".var", "app", "net.shadps4.shadPS4", "data", "shadPS4"), "Flatpak (ShadPS4)"},
			{filepath.Join(home, "shadps4"), "Local ShadPS4 Directory"},
			{filepath.Join(home, "Games", "bbport-linux"), "bbport Installation"},
			{filepath.Join(home, "bbport"), "Local bbport Directory"},
		}...)
		for _, c := range cands {
			if stat, err := os.Stat(c.path); err == nil && stat.IsDir() {
				return c.path, c.label
			}
		}
	}

	// 2. Wii U (Cemu)
	if strings.Contains(cLow, "cemu") || strings.HasPrefix(idLow, "cemu_") || strings.Contains(cLow, "wii u") || strings.Contains(cLow, "wiiu") {
		var cands []candidate
		for _, ed := range emuDeckRoots {
			cands = append(cands, candidate{filepath.Join(ed, "cemu", "mlc01", "usr", "save"), "EmuDeck (Cemu mlc01)"})
			cands = append(cands, candidate{filepath.Join(ed, "cemu"), "EmuDeck (Cemu)"})
		}
		cands = append(cands, []candidate{
			{filepath.Join(retroDeckSaves, "cemu"), "RetroDECK (Cemu)"},
			{filepath.Join(home, ".var", "app", "net.retrodeck.retrodeck", "data", "saves", "cemu"), "RetroDECK Flatpak (Cemu)"},
			{filepath.Join(home, ".var", "app", "info.cemu.Cemu", "data", "Cemu", "mlc01", "usr", "save"), "Flatpak (Cemu mlc01)"},
			{filepath.Join(home, ".var", "app", "info.cemu.Cemu", "data", "cemu", "mlc01", "usr", "save"), "Flatpak (Cemu mlc01)"},
			{filepath.Join(home, ".var", "app", "info.cemu.Cemu", "data", "Cemu"), "Flatpak (Cemu)"},
			{filepath.Join(home, ".var", "app", "info.cemu.Cemu", "data", "cemu"), "Flatpak (Cemu)"},
			{filepath.Join(home, ".local", "share", "Cemu"), "Auto-Detected Cemu"},
			{filepath.Join(home, ".local", "share", "cemu"), "Auto-Detected Cemu"},
			{filepath.Join(home, ".config", "Cemu"), "Auto-Detected Cemu Config"},
			{filepath.Join(home, ".config", "cemu"), "Auto-Detected Cemu Config"},
			{filepath.Join(home, "Games", "Cemu"), "Local Cemu Installation"},
			{filepath.Join(home, "Games", "cemu"), "Local Cemu Installation"},
		}...)
		for _, c := range cands {
			if stat, err := os.Stat(c.path); err == nil && stat.IsDir() {
				return c.path, c.label
			}
		}
	}

	// 3. PlayStation 3 (RPCS3)
	if strings.Contains(cLow, "rpcs3") || strings.HasPrefix(idLow, "rpcs3_") {
		var cands []candidate
		for _, ed := range emuDeckRoots {
			cands = append(cands, candidate{filepath.Join(ed, "rpcs3"), "EmuDeck (RPCS3)"})
		}
		cands = append(cands, []candidate{
			{filepath.Join(retroDeckSaves, "rpcs3"), "RetroDECK (RPCS3)"},
			{filepath.Join(home, ".config", "rpcs3"), "Auto-Detected RPCS3"},
			{filepath.Join(home, ".var", "app", "net.rpcs3.RPCS3", "config", "rpcs3"), "Flatpak (RPCS3)"},
		}...)
		for _, c := range cands {
			if stat, err := os.Stat(c.path); err == nil && stat.IsDir() {
				return c.path, c.label
			}
		}
	}

	// 4. PlayStation 2 (PCSX2)
	if strings.Contains(cLow, "pcsx2") || strings.HasPrefix(idLow, "pcsx2_") {
		var cands []candidate
		for _, ed := range emuDeckRoots {
			cands = append(cands, candidate{filepath.Join(ed, "pcsx2"), "EmuDeck (PCSX2)"})
		}
		cands = append(cands, []candidate{
			{filepath.Join(retroDeckSaves, "pcsx2"), "RetroDECK (PCSX2)"},
			{filepath.Join(home, ".config", "PCSX2"), "Auto-Detected PCSX2"},
			{filepath.Join(home, ".var", "app", "net.pcsx2.PCSX2", "config", "PCSX2"), "Flatpak (PCSX2)"},
		}...)
		for _, c := range cands {
			if stat, err := os.Stat(c.path); err == nil && stat.IsDir() {
				return c.path, c.label
			}
		}
	}

	// 5. Nintendo Switch (Ryujinx, Yuzu, Suyu, Eden, Sudachi, Torzu, Citron)
	if strings.Contains(cLow, "switch") || strings.Contains(cLow, "ryujinx") || strings.Contains(cLow, "yuzu") || strings.Contains(cLow, "suyu") || strings.Contains(cLow, "eden") || strings.Contains(cLow, "sudachi") || strings.HasPrefix(idLow, "emu_") || strings.HasPrefix(idLow, "switch_") {
		cands := []candidate{
			{filepath.Join(emuDeckSaves, "ryujinx"), "EmuDeck (Ryujinx)"},
			{filepath.Join(emuDeckSaves, "yuzu"), "EmuDeck (Yuzu)"},
			{filepath.Join(retroDeckSaves, "ryujinx"), "RetroDECK (Ryujinx)"},
			{filepath.Join(retroDeckSaves, "yuzu"), "RetroDECK (Yuzu)"},
			{filepath.Join(home, ".config", "Ryujinx"), "Auto-Detected Ryujinx"},
			{filepath.Join(home, ".var", "app", "org.ryujinx.Ryujinx", "config", "Ryujinx"), "Flatpak (Ryujinx)"},
			{filepath.Join(home, ".local", "share", "eden"), "Auto-Detected Eden"},
			{filepath.Join(home, ".local", "share", "yuzu"), "Auto-Detected Yuzu"},
			{filepath.Join(home, ".local", "share", "suyu"), "Auto-Detected Suyu"},
			{filepath.Join(home, ".local", "share", "sudachi"), "Auto-Detected Sudachi"},
			{filepath.Join(home, ".var", "app", "org.yuzu_emu.yuzu", "data", "yuzu"), "Flatpak (Yuzu)"},
			{filepath.Join(home, ".var", "app", "org.suyu_emu.suyu", "data", "suyu"), "Flatpak (Suyu)"},
		}
		for _, c := range cands {
			if stat, err := os.Stat(c.path); err == nil && stat.IsDir() {
				return c.path, c.label
			}
		}
	}

	// 6. PlayStation 1 (DuckStation)
	if strings.Contains(cLow, "duckstation") || strings.HasPrefix(idLow, "duckstation_") {
		cands := []candidate{
			{filepath.Join(emuDeckSaves, "duckstation"), "EmuDeck (DuckStation)"},
			{filepath.Join(retroDeckSaves, "duckstation"), "RetroDECK (DuckStation)"},
			{filepath.Join(home, ".local", "share", "duckstation"), "Auto-Detected DuckStation"},
			{filepath.Join(home, ".var", "app", "org.duckstation.DuckStation", "data", "duckstation"), "Flatpak (DuckStation)"},
		}
		for _, c := range cands {
			if stat, err := os.Stat(c.path); err == nil && stat.IsDir() {
				return c.path, c.label
			}
		}
	}

	// 7. GameCube & Wii (Dolphin)
	if strings.Contains(cLow, "dolphin") || strings.Contains(cLow, "wii/gc") || strings.HasPrefix(idLow, "emu_dolphin") {
		cands := []candidate{
			{filepath.Join(emuDeckSaves, "dolphin"), "EmuDeck (Dolphin)"},
			{filepath.Join(retroDeckSaves, "dolphin"), "RetroDECK (Dolphin)"},
			{filepath.Join(home, ".local", "share", "dolphin-emu"), "Auto-Detected Dolphin"},
			{filepath.Join(home, ".var", "app", "org.DolphinEmu.dolphin-emu", "data", "dolphin-emu"), "Flatpak (Dolphin)"},
		}
		for _, c := range cands {
			if stat, err := os.Stat(c.path); err == nil && stat.IsDir() {
				return c.path, c.label
			}
		}
	}

	// 8. Nintendo 3DS (Citra)
	if strings.Contains(cLow, "citra") || strings.Contains(cLow, "3ds") || strings.HasPrefix(idLow, "emu_citra") {
		cands := []candidate{
			{filepath.Join(emuDeckSaves, "citra"), "EmuDeck (Citra)"},
			{filepath.Join(home, ".local", "share", "citra-emu"), "Auto-Detected Citra"},
			{filepath.Join(home, ".var", "app", "org.citra_emu.citra", "data", "citra-emu"), "Flatpak (Citra)"},
		}
		for _, c := range cands {
			if stat, err := os.Stat(c.path); err == nil && stat.IsDir() {
				return c.path, c.label
			}
		}
	}

	// 9. Xbox 360 (Xenia)
	if strings.Contains(cLow, "xenia") || strings.Contains(cLow, "xbox 360") || strings.HasPrefix(idLow, "emu_xenia") {
		cands := []candidate{
			{filepath.Join(emuDeckSaves, "xenia"), "EmuDeck (Xenia)"},
			{filepath.Join(home, ".var", "app", "jp.xenia.xenia", "data", "xenia"), "Flatpak (Xenia)"},
		}
		for _, c := range cands {
			if stat, err := os.Stat(c.path); err == nil && stat.IsDir() {
				return c.path, c.label
			}
		}
	}

	return "", ""
}

// AutoLocateLinuxTarget resolves target paths with strict priority order for offline games:
// 1. Console Emulators (if emulator save)
// 2. Hydra Launcher (uses AppID / SteamID)
// 3. Heroic Games Launcher (uses custom game name / title)
// 4. Lutris (uses game slug / name)
// 5. Native In-Tree game directories
// 6. Default fallback to Hydra if uninitialized
func AutoLocateLinuxTarget(meta GameSaveMeta) (string, string, string, string) {
	cfg := LoadConfig()
	if loc, found := cfg.GetLocation(meta.ID, meta.AppID, meta.GameName); found && loc.LinuxResolvedPath != "" {
		if stat, err := os.Stat(loc.LinuxResolvedPath); err == nil && stat.IsDir() {
			return loc.LinuxResolvedPath, "remembered", "Remembered Location", "custom"
		}
	}

	home := os.Getenv("HOME")
	if home == "" {
		return "", "manual", "No HOME environment", "manual"
	}

	// 1. Console Emulators
	if emuRoot, emuLabel := LocateConsoleEmulatorPath(meta); emuRoot != "" {
		relPath := meta.TargetRelativePath
		cleanRel := filepath.ToSlash(relPath)
		cleanRoot := filepath.ToSlash(emuRoot)

		if strings.HasSuffix(strings.ToLower(cleanRoot), "/mlc01/usr/save") {
			cleanRel = strings.TrimPrefix(cleanRel, "cemu/")
			cleanRel = strings.TrimPrefix(cleanRel, "mlc01/usr/save/")
			relPath = cleanRel
		} else {
			parts := strings.Split(cleanRel, "/")
			base := filepath.Base(emuRoot)
			if len(parts) > 1 && strings.EqualFold(parts[0], base) {
				relPath = strings.Join(parts[1:], "/")
			}
		}
		target := resolveCaseInsensitivePath(emuRoot, relPath)
		return target, "emulator_detected", emuLabel, "emulator"
	}

	// 2. In-Tree native Linux game installation folders
	cleanName := strings.TrimSpace(meta.GameName)
	if meta.SaveType == SaveTypeInTree && cleanName != "" {
		gameCands := []string{
			filepath.Join(home, "Games", cleanName),
			filepath.Join(home, ".local", "share", "Steam", "steamapps", "common", cleanName),
		}
		for _, lib := range findSteamLibraryFolders() {
			gameCands = append(gameCands, filepath.Join(lib, "steamapps", "common", cleanName))
		}
		for _, cand := range gameCands {
			if stat, err := os.Stat(cand); err == nil && stat.IsDir() {
				target := resolveCaseInsensitivePath(cand, meta.TargetRelativePath)
				return target, "intree_detected", "Native Game Directory", "native"
			}
		}
	}

	// Check if emulator save needs fallback routing (never fall through to Wine/Proton prefixes)
	cLow := strings.ToLower(meta.StorageType)
	idLow := strings.ToLower(meta.ID)

	isEmulatorSave := strings.Contains(cLow, "emulator") || strings.Contains(cLow, "switch") ||
		strings.Contains(cLow, "wii") || strings.Contains(cLow, "ps1") || strings.Contains(cLow, "ps2") ||
		strings.Contains(cLow, "ps3") || strings.Contains(cLow, "ps4") || strings.Contains(cLow, "xbox 360") ||
		strings.HasPrefix(idLow, "emu_") || strings.HasPrefix(idLow, "cemu_") ||
		strings.HasPrefix(idLow, "rpcs3_") || strings.HasPrefix(idLow, "pcsx2_") ||
		strings.HasPrefix(idLow, "ps4_") || strings.HasPrefix(idLow, "switch_") ||
		strings.HasPrefix(idLow, "duckstation_")

	if isEmulatorSave {
		// Determine best default emulator directory
		if strings.Contains(cLow, "cemu") || strings.HasPrefix(idLow, "cemu_") || strings.Contains(cLow, "wii u") || strings.Contains(cLow, "wiiu") {
			var defaultRoot string
			var defaultLabel string
			if stat, err := os.Stat(filepath.Join(home, ".var", "app", "info.cemu.Cemu")); err == nil && stat.IsDir() {
				defaultRoot = filepath.Join(home, ".var", "app", "info.cemu.Cemu", "data", "Cemu")
				defaultLabel = "Flatpak (Cemu - Default)"
			} else if stat, err := os.Stat(filepath.Join(home, "Emulation")); err == nil && stat.IsDir() {
				defaultRoot = filepath.Join(home, "Emulation", "saves", "cemu")
				defaultLabel = "EmuDeck (Cemu - Default)"
			} else {
				defaultRoot = filepath.Join(home, ".var", "app", "info.cemu.Cemu", "data", "Cemu")
				defaultLabel = "Flatpak (Cemu - Default)"
			}
			relPath := meta.TargetRelativePath
			parts := strings.Split(filepath.ToSlash(relPath), "/")
			if len(parts) > 1 && strings.EqualFold(parts[0], "cemu") {
				relPath = strings.Join(parts[1:], "/")
			}
			target := filepath.Join(defaultRoot, filepath.FromSlash(relPath))
			return target, "emulator_detected", defaultLabel, "emulator"
		}

		if strings.Contains(cLow, "rpcs3") || strings.HasPrefix(idLow, "rpcs3_") || strings.Contains(cLow, "ps3") {
			defaultRoot := filepath.Join(home, ".var", "app", "net.rpcs3.RPCS3", "config", "rpcs3")
			if stat, err := os.Stat(filepath.Join(home, "Emulation")); err == nil && stat.IsDir() {
				defaultRoot = filepath.Join(home, "Emulation", "saves", "rpcs3")
			}
			relPath := meta.TargetRelativePath
			parts := strings.Split(filepath.ToSlash(relPath), "/")
			if len(parts) > 1 && strings.EqualFold(parts[0], "rpcs3") {
				relPath = strings.Join(parts[1:], "/")
			}
			target := filepath.Join(defaultRoot, filepath.FromSlash(relPath))
			return target, "emulator_detected", "Auto-Target (RPCS3)", "emulator"
		}

		if strings.Contains(cLow, "pcsx2") || strings.HasPrefix(idLow, "pcsx2_") || strings.Contains(cLow, "ps2") {
			defaultRoot := filepath.Join(home, ".var", "app", "net.pcsx2.PCSX2", "config", "PCSX2")
			if stat, err := os.Stat(filepath.Join(home, "Emulation")); err == nil && stat.IsDir() {
				defaultRoot = filepath.Join(home, "Emulation", "saves", "pcsx2")
			}
			relPath := meta.TargetRelativePath
			parts := strings.Split(filepath.ToSlash(relPath), "/")
			if len(parts) > 1 && strings.EqualFold(parts[0], "pcsx2") {
				relPath = strings.Join(parts[1:], "/")
			}
			target := filepath.Join(defaultRoot, filepath.FromSlash(relPath))
			return target, "emulator_detected", "Auto-Target (PCSX2)", "emulator"
		}

		if strings.Contains(cLow, "switch") || strings.Contains(cLow, "ryujinx") || strings.HasPrefix(idLow, "emu_ryujinx") {
			defaultRoot := filepath.Join(home, ".var", "app", "org.ryujinx.Ryujinx", "config", "Ryujinx")
			if stat, err := os.Stat(filepath.Join(home, "Emulation")); err == nil && stat.IsDir() {
				defaultRoot = filepath.Join(home, "Emulation", "saves", "ryujinx")
			}
			target := filepath.Join(defaultRoot, filepath.FromSlash(meta.TargetRelativePath))
			return target, "emulator_detected", "Auto-Target (Ryujinx)", "emulator"
		}
	}

	// ==========================================
	// OFFLINE GAME PRIORITY DETECTION
	// 1st: Hydra Launcher (AppID)
	// 2nd: Heroic (Custom Name)
	// 3rd: Lutris
	// ==========================================

	// Check if Hydra prefix is already on disk
	if meta.AppID != "" {
		if hydraC := LocateHydraPrefix(meta.AppID); hydraC != "" {
			target := resolveCaseInsensitivePath(hydraC, meta.TargetRelativePath)
			return target, "hydra_detected", "Auto-Detected Hydra Launcher Prefix", "hydra"
		}
	}

	// Check if Heroic prefix is on disk
	if cleanName != "" {
		if heroicC := LocateHeroicPrefix(cleanName, meta.AppID); heroicC != "" {
			target := resolveCaseInsensitivePath(heroicC, meta.TargetRelativePath)
			return target, "heroic_detected", "Auto-Detected Heroic Prefix", "heroic"
		}
	}

	// Check if Lutris prefix is on disk
	if cleanName != "" {
		if lutrisC := LocateLutrisPrefix(cleanName, meta.AppID); lutrisC != "" {
			target := resolveCaseInsensitivePath(lutrisC, meta.TargetRelativePath)
			return target, "lutris_detected", "Auto-Detected Lutris Prefix", "lutris"
		}
	}

	// PortProton check
	if cleanName != "" {
		if portC := LocatePortProtonPrefix(cleanName, meta.AppID); portC != "" {
			target := resolveCaseInsensitivePath(portC, meta.TargetRelativePath)
			return target, "portproton_detected", "Auto-Detected PortProton Prefix", "portproton"
		}
	}

	// Bottles check
	if cleanName != "" {
		if bottlesC := LocateBottlesPrefix(cleanName); bottlesC != "" {
			target := resolveCaseInsensitivePath(bottlesC, meta.TargetRelativePath)
			return target, "bottles_detected", "Auto-Detected Bottles Prefix", "bottles"
		}
	}

	// If prefix not created yet on disk:
	// For offline games, default target is Hydra Launcher (or Heroic if no AppID)
	if meta.AppID != "" {
		expected := ExpectedHydraPath(meta.AppID, meta.TargetRelativePath)
		return expected, "not_initialized", "Prefix Uninitialized (Auto-seeds on Restore)", "hydra"
	}

	if cleanName != "" {
		expected := ExpectedHeroicPath(cleanName, meta.TargetRelativePath)
		return expected, "not_initialized", "Prefix Uninitialized (Auto-seeds on Restore)", "heroic"
	}

	return "", "manual", "Manual Path Selection Required", "manual"
}

// RestoreGame restores a single game save from an extracted folder or zip archive
func RestoreGame(sourcePath string, meta GameSaveMeta, customDestDir string) RestoreReport {
	report := RestoreReport{
		GameID:   meta.ID,
		GameName: meta.GameName,
	}

	var targetDir string

	if customDestDir != "" {
		// If user selected a prefix root (e.g. Bottles, PortProton, Heroic folder), validate and resolve drive_c
		if driveC, valid, _ := ValidateWinePrefix(customDestDir); valid {
			targetDir = resolveCaseInsensitivePath(driveC, meta.TargetRelativePath)
		} else {
			targetDir = customDestDir
			if meta.SaveType == SaveTypeInTree && !strings.Contains(filepath.ToSlash(targetDir), filepath.ToSlash(meta.TargetRelativePath)) {
				targetDir = resolveCaseInsensitivePath(customDestDir, meta.TargetRelativePath)
			}
		}

		// Remember location in persistent config
		cfg := LoadConfig()
		cfg.RememberLocation(SavedLocation{
			GameID:             meta.ID,
			GameName:           meta.GameName,
			AppID:              meta.AppID,
			OriginalPath:       meta.OriginalSourcePath,
			TargetRelativePath: meta.TargetRelativePath,
			LinuxResolvedPath:  targetDir,
			SaveType:           string(meta.SaveType),
			StorageType:          meta.StorageType,
		})
		_ = SaveConfig(cfg)
	} else {
		autoTarget, _, _, _ := AutoLocateLinuxTarget(meta)
		if autoTarget != "" {
			targetDir = autoTarget
		} else {
			report.Success = false
			report.Error = "Target directory not detected on Linux. Please select or initialize the prefix first."
			return report
		}
	}

	report.TargetResolved = targetDir

	// Extract data.tar.gz into targetDir
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		report.Success = false
		report.Error = fmt.Sprintf("failed to create directory %s: %v", targetDir, err)
		return report
	}

	stat, err := os.Stat(sourcePath)
	if err != nil {
		report.Success = false
		report.Error = fmt.Sprintf("source path inaccessible: %v", err)
		return report
	}

	var extractErr error
	if stat.IsDir() {
		tarPath := filepath.Join(sourcePath, meta.ID, "data.tar.gz")
		extractErr = extractTarGz(tarPath, targetDir)
	} else {
		extractErr = extractTarFromZip(sourcePath, meta.ID+"/data.tar.gz", targetDir)
	}

	if extractErr != nil {
		report.Success = false
		report.Error = extractErr.Error()
		return report
	}

	// Align FromSoftware SteamID & MD5 checksums if running inside Proton/Wine
	HandleFromSoftwareAlignment(targetDir, meta.AppID)

	report.Success = true
	return report
}

func safeBackupExistingFile(target string) {
	stat, err := os.Stat(target)
	if err != nil || stat.IsDir() {
		return
	}
	backupPath := target + ".bak"
	if _, err := os.Stat(backupPath); err == nil {
		backupPath = fmt.Sprintf("%s.bak_%s", target, time.Now().Format("20060102_150405"))
	}
	if err := copyFile(target, backupPath); err == nil {
		Log("Preserved existing save: created backup at %s", backupPath)
	}
}

func copyFile(src, dst string) error {
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

// extractTarGz extracts a .tar.gz archive with path traversal checks and automatic backup
func extractTarGz(tarPath, targetDir string) error {
	file, err := os.Open(tarPath)
	if err != nil {
		return err
	}
	defer file.Close()

	gzr, err := gzip.NewReader(file)
	if err != nil {
		return err
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)
	now := time.Now()
	cleanDest := filepath.Clean(targetDir)

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		target := filepath.Join(cleanDest, filepath.FromSlash(header.Name))
		cleanTarget := filepath.Clean(target)
		if !strings.HasPrefix(cleanTarget, cleanDest+string(filepath.Separator)) && cleanTarget != cleanDest {
			return fmt.Errorf("illegal path traversal in archive: %s", header.Name)
		}

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(cleanTarget, 0755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(cleanTarget), 0755); err != nil {
				return err
			}
			safeBackupExistingFile(cleanTarget)

			outFile, err := os.OpenFile(cleanTarget, os.O_CREATE|os.O_RDWR|os.O_TRUNC, 0644)
			if err != nil {
				return err
			}
			if _, err := io.Copy(outFile, tr); err != nil {
				outFile.Close()
				return err
			}
			outFile.Close()

			// Timestamp Protection
			modTime := now
			if header.ModTime.After(now) {
				modTime = header.ModTime
			}
			_ = os.Chtimes(cleanTarget, modTime, modTime)
		}
	}
	return nil
}

// extractTarFromZip extracts an inner tar.gz from zip with path traversal protection and backup
func extractTarFromZip(zipPath, innerTarPath, targetDir string) error {
	zr, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	defer zr.Close()

	now := time.Now()
	cleanDest := filepath.Clean(targetDir)

	for _, f := range zr.File {
		if strings.HasSuffix(filepath.ToSlash(f.Name), innerTarPath) {
			rc, err := f.Open()
			if err != nil {
				return err
			}
			defer rc.Close()

			gzr, err := gzip.NewReader(rc)
			if err != nil {
				return err
			}
			defer gzr.Close()

			tr := tar.NewReader(gzr)
			for {
				header, err := tr.Next()
				if err == io.EOF {
					break
				}
				if err != nil {
					return err
				}

				target := filepath.Join(cleanDest, filepath.FromSlash(header.Name))
				cleanTarget := filepath.Clean(target)
				if !strings.HasPrefix(cleanTarget, cleanDest+string(filepath.Separator)) && cleanTarget != cleanDest {
					return fmt.Errorf("illegal path traversal in archive: %s", header.Name)
				}

				switch header.Typeflag {
				case tar.TypeDir:
					if err := os.MkdirAll(cleanTarget, 0755); err != nil {
						return err
					}
				case tar.TypeReg:
					if err := os.MkdirAll(filepath.Dir(cleanTarget), 0755); err != nil {
						return err
					}
					safeBackupExistingFile(cleanTarget)

					outFile, err := os.OpenFile(cleanTarget, os.O_CREATE|os.O_RDWR|os.O_TRUNC, 0644)
					if err != nil {
						return err
					}
					if _, err := io.Copy(outFile, tr); err != nil {
						outFile.Close()
						return err
					}
					outFile.Close()

					// Timestamp Protection
					modTime := now
					if header.ModTime.After(now) {
						modTime = header.ModTime
					}
					_ = os.Chtimes(cleanTarget, modTime, modTime)
				}
			}
			return nil
		}
	}
	return fmt.Errorf("could not find %s in zip archive", innerTarPath)
}

// FindCandidateVaults searches common locations for save vault zip archives
func FindCandidateVaults() []string {
	home := os.Getenv("HOME")
	if home == "" {
		return nil
	}

	searchDirs := []string{
		filepath.Join(home, "Downloads"),
		filepath.Join(home, "Downloads", "temp"),
		filepath.Join(home, "Downloads", "Telegram Desktop"),
		filepath.Join(home, "Downloads", "Reook"),
		filepath.Join(home, "Downloads", "save-migrator"),
		filepath.Join(home, "Desktop"),
	}

	user := os.Getenv("USER")
	if user != "" {
		mediaRoots := []string{
			filepath.Join("/run/media", user),
			filepath.Join("/media", user),
			"/mnt",
		}
		for _, m := range mediaRoots {
			if entries, err := os.ReadDir(m); err == nil {
				for _, e := range entries {
					if e.IsDir() {
						searchDirs = append(searchDirs, filepath.Join(m, e.Name()))
					}
				}
			}
		}
	}

	var found []string
	seen := make(map[string]bool)

	for _, dir := range searchDirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if !e.IsDir() && strings.HasSuffix(strings.ToLower(e.Name()), ".zip") {
				nameLow := strings.ToLower(e.Name())
				if strings.Contains(nameLow, "bundle") || strings.Contains(nameLow, "save") {
					full := filepath.Join(dir, e.Name())
					if !seen[full] {
						seen[full] = true
						found = append(found, full)
					}
				}
			}
		}
	}
	return found
}

// resolveCaseInsensitivePath navigates directories segment-by-segment matching existing case-insensitivity on Linux
func resolveCaseInsensitivePath(root, relPath string) string {
	parts := strings.Split(filepath.ToSlash(relPath), "/")
	current := root

	for _, part := range parts {
		if part == "" || part == "." {
			continue
		}
		entries, err := os.ReadDir(current)
		if err != nil {
			current = filepath.Join(current, part)
			continue
		}

		matched := false
		for _, e := range entries {
			if strings.EqualFold(e.Name(), part) {
				current = filepath.Join(current, e.Name())
				matched = true
				break
			}
		}
		if !matched {
			current = filepath.Join(current, part)
		}
	}
	return current
}
