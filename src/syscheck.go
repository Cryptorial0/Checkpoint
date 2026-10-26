package main

import (
	"bufio"
	"context"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

type SystemStatus struct {
	Platform          string `json:"platform"`
	FastStartupActive bool   `json:"fast_startup_active"`
	BitLockerWarning  bool   `json:"bitlocker_warning"`
	BitLockerDetails  string `json:"bitlocker_details"`
	NTFSIssuesFound   bool   `json:"ntfs_issues_found"`
	NTFSSuggestion    string `json:"ntfs_suggestion"`
}


func CheckSystem() SystemStatus {
	status := SystemStatus{
		Platform: runtime.GOOS,
	}

	if runtime.GOOS == "windows" {
		status.FastStartupActive = checkWindowsFastStartup()
		status.BitLockerWarning, status.BitLockerDetails = checkWindowsBitLocker()
	} else if runtime.GOOS == "linux" {
		status.NTFSIssuesFound, status.NTFSSuggestion = checkLinuxNTFSMounts()
	}

	return status
}

func checkWindowsFastStartup() bool {
	// Query registry directly with hidden window - no powershell
	cmd := exec.Command("reg", "query", `HKLM\SYSTEM\CurrentControlSet\Control\Session Manager\Power`, "/v", "HiberbootEnabled")
	cmd.SysProcAttr = hideWindowAttr()
	out, err := cmd.Output()
	if err != nil {
		return false
	}
	return strings.Contains(string(out), "0x1")
}

func DisableFastStartup() (string, error) {
	if runtime.GOOS != "windows" {
		return "Fast startup is only applicable on Windows.", nil
	}
	cmd := exec.Command("powercfg", "-h", "off")
	cmd.SysProcAttr = hideWindowAttr()
	out, err := cmd.CombinedOutput()
	if err != nil {
		return string(out), err
	}
	return "Fast startup and hibernation successfully disabled.", nil
}

func EnableFastStartup() (string, error) {
	if runtime.GOOS != "windows" {
		return "Fast startup is only applicable on Windows.", nil
	}
	cmd := exec.Command("powercfg", "-h", "on")
	cmd.SysProcAttr = hideWindowAttr()
	out, err := cmd.CombinedOutput()
	if err != nil {
		return string(out), err
	}
	return "Fast startup and hibernation successfully re-enabled.", nil
}

func checkWindowsBitLocker() (bool, string) {
	// Use manage-bde with a strict 1-second timeout and hidden window.
	// Zero PowerShell, zero console window flashing.
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "manage-bde.exe", "-status")
	cmd.SysProcAttr = hideWindowAttr()
	out, err := cmd.Output()
	if err != nil {
		return false, ""
	}

	outStr := string(out)
	if strings.Contains(outStr, "Protection On") || strings.Contains(outStr, "Percentage Encrypted: 100%") {
		return true, "BitLocker protection detected on one or more drives. Export recovery keys before formatting."
	}
	return false, ""
}

func checkLinuxNTFSMounts() (bool, string) {
	file, err := os.Open("/proc/mounts")
	if err != nil {
		return false, ""
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	var problematicMounts []string
	for scanner.Scan() {
		line := scanner.Text()
		fields := strings.Fields(line)
		if len(fields) >= 4 {
			fsType := fields[2]
			mountPoint := fields[1]
			options := fields[3]

			if fsType == "ntfs" || fsType == "ntfs3" || fsType == "fuseblk" {
				if !strings.Contains(options, "exec") || strings.Contains(options, "noexec") {
					problematicMounts = append(problematicMounts, mountPoint)
				}
			}
		}
	}

	if len(problematicMounts) > 0 {
		return true, "NTFS drive mounted at " + strings.Join(problematicMounts, ", ") + " without 'exec' permission. Steam Proton will fail to execute games here."
	}
	return false, ""
}
