package main

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// RPCSaveProfile defines rules for pruning oversized RPG saves
type RPCSaveProfile struct {
	GameSignature    string
	MaxQuicksaves    int
	MaxAutosaves     int
	PreserveManual   bool
}

var knownRPGProfiles = []RPCSaveProfile{
	{
		GameSignature:  "baldurs gate 3",
		MaxQuicksaves:  5,
		MaxAutosaves:   3,
		PreserveManual: true,
	},
	{
		GameSignature:  "starfield",
		MaxQuicksaves:  5,
		MaxAutosaves:   3,
		PreserveManual: true,
	},
	{
		GameSignature:  "cyberpunk 2077",
		MaxQuicksaves:  5,
		MaxAutosaves:   3,
		PreserveManual: true,
	},
	{
		GameSignature:  "the witcher 3",
		MaxQuicksaves:  5,
		MaxAutosaves:   3,
		PreserveManual: true,
	},
	{
		GameSignature:  "skyrim",
		MaxQuicksaves:  5,
		MaxAutosaves:   3,
		PreserveManual: true,
	},
	{
		GameSignature:  "fallout 4",
		MaxQuicksaves:  5,
		MaxAutosaves:   3,
		PreserveManual: true,
	},
}

type SaveItem struct {
	Name     string
	Path     string
	RelPath  string
	ModTime  time.Time
	SizeBytes int64
	IsManual bool
	IsQuick  bool
	IsAuto   bool
}

// IsRPGOptimizable checks if the detected game matches an RPG with sprawling autosaves
func IsRPGOptimizable(gameName, targetRelPath string) bool {
	combined := strings.ToLower(gameName + " " + targetRelPath)
	for _, p := range knownRPGProfiles {
		if strings.Contains(combined, p.GameSignature) {
			return true
		}
	}
	// Extra checks for Larian, CDPR, Bethesda paths
	if strings.Contains(combined, "larian studios") ||
		strings.Contains(combined, "cd projekt red") ||
		strings.Contains(combined, "my games/starfield") ||
		strings.Contains(combined, "my games/skyrim") ||
		strings.Contains(combined, "my games/fallout4") {
		return true
	}
	return false
}

// BuildRPGExcludedPaths scans sourceDir for an RPG and returns a set of relative file/folder paths to exclude
func BuildRPGExcludedPaths(sourceDir string, gameName, targetRelPath string) (map[string]bool, int64) {
	excluded := make(map[string]bool)
	if !IsRPGOptimizable(gameName, targetRelPath) {
		return excluded, 0
	}

	combined := strings.ToLower(gameName + " " + targetRelPath)
	var maxQuick, maxAuto int = 5, 3
	for _, p := range knownRPGProfiles {
		if strings.Contains(combined, p.GameSignature) {
			maxQuick = p.MaxQuicksaves
			maxAuto = p.MaxAutosaves
			break
		}
	}

	// 1. Baldur's Gate 3 folder-based saves (Story/<SaveName>/<SaveName>.lsv)
	if strings.Contains(combined, "baldurs gate 3") || strings.Contains(combined, "larian studios") {
		storyDir := sourceDir
		if !strings.HasSuffix(strings.ToLower(sourceDir), "story") {
			// Search for Story subfolder
			cand := filepath.Join(sourceDir, "PlayerProfiles", "Public", "Savegames", "Story")
			if fi, err := os.Stat(cand); err == nil && fi.IsDir() {
				storyDir = cand
			}
		}

		entries, err := os.ReadDir(storyDir)
		if err == nil {
			var quickSaves []SaveItem
			var autoSaves []SaveItem

			for _, e := range entries {
				if !e.IsDir() {
					continue
				}
				info, err := e.Info()
				if err != nil {
					continue
				}
				savePath := filepath.Join(storyDir, e.Name())
				rel, err := filepath.Rel(sourceDir, savePath)
				if err != nil {
					continue
				}
				item := SaveItem{
					Name:    e.Name(),
					Path:    savePath,
					RelPath: filepath.ToSlash(rel),
					ModTime: info.ModTime(),
				}

				low := strings.ToLower(e.Name())
				if strings.HasPrefix(low, "quicksave_") {
					quickSaves = append(quickSaves, item)
				} else if strings.HasPrefix(low, "autosave_") {
					autoSaves = append(autoSaves, item)
				}
			}

			var savedBytes int64
			// Sort descending by ModTime (newest first)
			sort.Slice(quickSaves, func(i, j int) bool {
				return quickSaves[i].ModTime.After(quickSaves[j].ModTime)
			})
			sort.Slice(autoSaves, func(i, j int) bool {
				return autoSaves[i].ModTime.After(autoSaves[j].ModTime)
			})

			// Exclude older quicksaves beyond maxQuick
			if len(quickSaves) > maxQuick {
				for _, q := range quickSaves[maxQuick:] {
					excluded[q.RelPath] = true
					_, sz := getDirStats(q.Path)
					savedBytes += sz
				}
			}
			// Exclude older autosaves beyond maxAuto
			if len(autoSaves) > maxAuto {
				for _, a := range autoSaves[maxAuto:] {
					excluded[a.RelPath] = true
					_, sz := getDirStats(a.Path)
					savedBytes += sz
				}
			}
			return excluded, savedBytes
		}
	}

	// 2. Cyberpunk 2077 folder-based saves (AutoSave-*, QuickSave-*, ManualSave-*)
	if strings.Contains(combined, "cyberpunk 2077") || strings.Contains(combined, "cd projekt red") {
		entries, err := os.ReadDir(sourceDir)
		if err == nil {
			var quickSaves []SaveItem
			var autoSaves []SaveItem

			for _, e := range entries {
				if !e.IsDir() {
					continue
				}
				info, err := e.Info()
				if err != nil {
					continue
				}
				savePath := filepath.Join(sourceDir, e.Name())
				rel, err := filepath.Rel(sourceDir, savePath)
				if err != nil {
					continue
				}
				item := SaveItem{
					Name:    e.Name(),
					Path:    savePath,
					RelPath: filepath.ToSlash(rel),
					ModTime: info.ModTime(),
				}

				low := strings.ToLower(e.Name())
				if strings.HasPrefix(low, "quicksave-") {
					quickSaves = append(quickSaves, item)
				} else if strings.HasPrefix(low, "autosave-") {
					autoSaves = append(autoSaves, item)
				}
			}

			var savedBytes int64
			sort.Slice(quickSaves, func(i, j int) bool {
				return quickSaves[i].ModTime.After(quickSaves[j].ModTime)
			})
			sort.Slice(autoSaves, func(i, j int) bool {
				return autoSaves[i].ModTime.After(autoSaves[j].ModTime)
			})

			if len(quickSaves) > maxQuick {
				for _, q := range quickSaves[maxQuick:] {
					excluded[q.RelPath] = true
					_, sz := getDirStats(q.Path)
					savedBytes += sz
				}
			}
			if len(autoSaves) > maxAuto {
				for _, a := range autoSaves[maxAuto:] {
					excluded[a.RelPath] = true
					_, sz := getDirStats(a.Path)
					savedBytes += sz
				}
			}
			return excluded, savedBytes
		}
	}

	// 3. Flat file-based RPG saves (Starfield *.sfs, Skyrim *.ess, Fallout 4 *.fos)
	entries, err := os.ReadDir(sourceDir)
	if err == nil {
		var quickSaves []SaveItem
		var autoSaves []SaveItem

		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			info, err := e.Info()
			if err != nil {
				continue
			}
			filePath := filepath.Join(sourceDir, e.Name())
			rel, err := filepath.Rel(sourceDir, filePath)
			if err != nil {
				continue
			}
			item := SaveItem{
				Name:      e.Name(),
				Path:      filePath,
				RelPath:   filepath.ToSlash(rel),
				ModTime:   info.ModTime(),
				SizeBytes: info.Size(),
			}

			low := strings.ToLower(e.Name())
			if strings.HasPrefix(low, "quicksave") || strings.Contains(low, "_quick") {
				quickSaves = append(quickSaves, item)
			} else if strings.HasPrefix(low, "autosave") || strings.Contains(low, "_auto") {
				autoSaves = append(autoSaves, item)
			}
		}

		var savedBytes int64
		sort.Slice(quickSaves, func(i, j int) bool {
			return quickSaves[i].ModTime.After(quickSaves[j].ModTime)
		})
		sort.Slice(autoSaves, func(i, j int) bool {
			return autoSaves[i].ModTime.After(autoSaves[j].ModTime)
		})

		if len(quickSaves) > maxQuick {
			for _, q := range quickSaves[maxQuick:] {
				excluded[q.RelPath] = true
				savedBytes += q.SizeBytes
			}
		}
		if len(autoSaves) > maxAuto {
			for _, a := range autoSaves[maxAuto:] {
				excluded[a.RelPath] = true
				savedBytes += a.SizeBytes
			}
		}
		return excluded, savedBytes
	}

	return excluded, 0
}
