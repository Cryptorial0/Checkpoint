//go:build !windows

package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func openFolderPickerOS(title string) (string, error) {
	if path, err := exec.LookPath("zenity"); err == nil {
		cmd := exec.Command(path, "--file-selection", "--directory", "--title="+title)
		out, err := cmd.Output()
		if err == nil {
			return strings.TrimSpace(string(out)), nil
		}
	}
	if path, err := exec.LookPath("kdialog"); err == nil {
		cmd := exec.Command(path, "--getexistingdirectory")
		out, err := cmd.Output()
		if err == nil {
			return strings.TrimSpace(string(out)), nil
		}
	}
	return "", nil
}

func openFilePickerOS(title, filter string) (string, error) {
	if path, err := exec.LookPath("zenity"); err == nil {
		cmd := exec.Command(path, "--file-selection", "--title="+title, "--file-filter=ZIP Files (*.zip) | *.zip")
		out, err := cmd.Output()
		if err == nil {
			return strings.TrimSpace(string(out)), nil
		}
	}
	if path, err := exec.LookPath("kdialog"); err == nil {
		cmd := exec.Command(path, "--getopenfilename", ":", "*.zip")
		out, err := cmd.Output()
		if err == nil {
			return strings.TrimSpace(string(out)), nil
		}
	}
	return "", nil
}

func getAvailableDrivesOS() []DriveInfo {
	var drives []DriveInfo
	user := os.Getenv("USER")
	mediaRoots := []string{
		filepath.Join("/run/media", user),
		filepath.Join("/media", user),
		"/mnt",
	}
	for _, m := range mediaRoots {
		entries, err := os.ReadDir(m)
		if err == nil {
			for _, e := range entries {
				if e.IsDir() {
					full := filepath.Join(m, e.Name())
					isRemovable := strings.HasPrefix(m, "/run/media") || strings.HasPrefix(m, "/media")
					label := e.Name() + " (" + full + ")"
					if isRemovable {
						label += " [USB Drive]"
					}
					drives = append(drives, DriveInfo{
						Path:        full,
						Label:       label,
						IsRemovable: isRemovable,
					})
				}
			}
		}
	}
	return drives
}
