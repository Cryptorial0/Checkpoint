package main

import (
	"bytes"
	"crypto/md5"
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
)

// PatchEldenRingSave updates the 8-byte SteamID64 and recalculates all 10 slot MD5 checksums
func PatchEldenRingSave(filePath string, oldSteamID, newSteamID uint64) error {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read save file: %w", err)
	}

	var oldBytes [8]byte
	var newBytes [8]byte
	binary.LittleEndian.PutUint64(oldBytes[:], oldSteamID)
	binary.LittleEndian.PutUint64(newBytes[:], newSteamID)

	// Replace all instances of old SteamID in the save file
	replacedCount := 0
	for {
		idx := bytes.Index(data, oldBytes[:])
		if idx == -1 {
			break
		}
		copy(data[idx:idx+8], newBytes[:])
		replacedCount++
	}

	Log("Replaced %d instances of SteamID %d with %d in %s", replacedCount, oldSteamID, newSteamID, filepath.Base(filePath))

	// Elden Ring .sl2 structure: 10 character slots with MD5 checksums
	const (
		slotCount = 10
		slotLen   = 0x280000 // 2,621,440 bytes per slot
		csStart   = 0x300    // First checksum at 0x300 (16 bytes)
		sStart    = 0x310    // First slot data at 0x310
		stride    = 0x280010 // Distance between slot checksums
	)

	// Recalculate MD5 checksum for each of the 10 character slots
	for i := 0; i < slotCount; i++ {
		curCs := csStart + i*stride
		curSlot := sStart + i*stride

		if curSlot+slotLen > len(data) || curCs+16 > len(data) {
			break
		}

		slotBytes := data[curSlot : curSlot+slotLen]
		hash := md5.Sum(slotBytes)
		copy(data[curCs:curCs+16], hash[:])
	}

	// Write patched save back to disk
	if err := os.WriteFile(filePath, data, 0666); err != nil {
		return fmt.Errorf("failed to write patched save: %w", err)
	}

	Log("Successfully recalculated all slot MD5 checksums for %s", filepath.Base(filePath))
	return nil
}

// HandleFromSoftwareAlignment checks if the Linux Proton prefix has a different SteamID and auto-patches
func HandleFromSoftwareAlignment(targetDir, appID string) {
	if appID == "1245620" { // Elden Ring
		eldenRingDir := targetDir
		if filepath.Base(targetDir) != "EldenRing" && filepath.Base(filepath.Dir(targetDir)) == "EldenRing" {
			eldenRingDir = filepath.Dir(targetDir)
		}
		if filepath.Base(eldenRingDir) != "EldenRing" {
			return
		}

		entries, err := os.ReadDir(eldenRingDir)
		if err != nil {
			return
		}

		var sourceID uint64
		var sourceSavePath string
		var targetDirs []string

		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			id, err := strconv.ParseUint(e.Name(), 10, 64)
			if err != nil || len(e.Name()) < 16 {
				continue
			}
			savePath := filepath.Join(eldenRingDir, e.Name(), "ER0000.sl2")
			if fi, err := os.Stat(savePath); err == nil && fi.Size() > 0 {
				if sourceSavePath == "" {
					sourceID = id
					sourceSavePath = savePath
				}
			} else {
				targetDirs = append(targetDirs, filepath.Join(eldenRingDir, e.Name()))
			}
		}

		if sourceSavePath == "" || len(targetDirs) == 0 {
			return
		}

		data, err := os.ReadFile(sourceSavePath)
		if err != nil {
			return
		}

		for _, tDir := range targetDirs {
			targetIDStr := filepath.Base(tDir)
			targetID, err := strconv.ParseUint(targetIDStr, 10, 64)
			if err != nil || targetID == 0 || targetID == sourceID {
				continue
			}
			newSavePath := filepath.Join(tDir, "ER0000.sl2")
			Log("Aligning Elden Ring save from SteamID %d to Proton SteamID %s", sourceID, targetIDStr)
			if err := os.WriteFile(newSavePath, data, 0666); err == nil {
				if err := PatchEldenRingSave(newSavePath, sourceID, targetID); err == nil {
					Log("Successfully patched and aligned ER0000.sl2 for SteamID %s", targetIDStr)
				}
			}
		}
	}
}

// FindTargetProtonSteamID checks if the Linux Proton prefix already has a SteamID directory
func FindTargetProtonSteamID(prefixPath, gameFolder string) (string, error) {
	appDataDir := filepath.Join(prefixPath, "drive_c", "users", "steamuser", "AppData", "Roaming", gameFolder)
	entries, err := os.ReadDir(appDataDir)
	if err != nil {
		return "", err
	}

	for _, e := range entries {
		if e.IsDir() {
			name := e.Name()
			if len(name) >= 16 {
				if _, err := strconv.ParseUint(name, 10, 64); err == nil {
					return name, nil
				}
			}
		}
	}
	return "", nil
}

// SyncGoldbergSteamID drops or updates user_steam_id.txt so Goldberg Steam emulator impersonates the save ID
func SyncGoldbergSteamID(gameDir, steamID string) error {
	candidates := []string{
		filepath.Join(gameDir, "steam_settings", "user_steam_id.txt"),
		filepath.Join(gameDir, "settings", "user_steam_id.txt"),
	}

	for _, c := range candidates {
		dir := filepath.Dir(c)
		if fi, err := os.Stat(dir); err == nil && fi.IsDir() {
			if err := os.WriteFile(c, []byte(steamID), 0666); err == nil {
				Log("Wrote SteamID %s to %s", steamID, c)
				return nil
			}
		}
	}
	return nil
}
