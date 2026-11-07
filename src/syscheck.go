package main

import (
	"bufio"
	"context"
	"fmt"
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

	// 1. Direct execution attempt (succeeds if running as Administrator)
	cmd := exec.Command("powercfg", "-h", "off")
	cmd.SysProcAttr = hideWindowAttr()
	_, err := cmd.CombinedOutput()
	if err == nil {
		regCmd := exec.Command("reg", "add", `HKLM\SYSTEM\CurrentControlSet\Control\Session Manager\Power`, "/v", "HiberbootEnabled", "/t", "REG_DWORD", "/d", "0", "/f")
		regCmd.SysProcAttr = hideWindowAttr()
		_ = regCmd.Run()
		if !checkWindowsFastStartup() {
			return "Fast startup and hibernation successfully disabled.", nil
		}
	}

	// 2. Request UAC elevation via PowerShell Start-Process -Verb RunAs
	psScript := `Start-Process powershell.exe -ArgumentList "-NoProfile -Command powercfg -h off; Set-ItemProperty -Path 'HKLM:\SYSTEM\CurrentControlSet\Control\Session Manager\Power' -Name HiberbootEnabled -Value 0 -Type DWord" -Verb RunAs -WindowStyle Hidden -Wait`
	elevCmd := exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-Command", psScript)
	elevCmd.SysProcAttr = hideWindowAttr()
	elevOut, elevErr := elevCmd.CombinedOutput()
	if elevErr != nil {
		outStr := strings.TrimSpace(string(elevOut))
		low := strings.ToLower(outStr)
		if strings.Contains(low, "canceled by the user") || strings.Contains(low, "cancelled") {
			return "Administrator permission was denied by the user. Fast startup requires administrator privileges to modify.", elevErr
		}
		if outStr != "" {
			return fmt.Sprintf("Failed to disable fast startup: %s", outStr), elevErr
		}
		return fmt.Sprintf("Failed to disable fast startup (%v). Please run Checkpoint as Administrator.", elevErr), elevErr
	}

	if checkWindowsFastStartup() {
		return "Fast startup check still reported active. Please run Checkpoint as Administrator.", fmt.Errorf("fast startup still active")
	}

	return "Fast startup and hibernation successfully disabled.", nil
}

func EnableFastStartup() (string, error) {
	if runtime.GOOS != "windows" {
		return "Fast startup is only applicable on Windows.", nil
	}

	// 1. Direct execution attempt
	cmd := exec.Command("powercfg", "-h", "on")
	cmd.SysProcAttr = hideWindowAttr()
	_, err := cmd.CombinedOutput()
	if err == nil {
		regCmd := exec.Command("reg", "add", `HKLM\SYSTEM\CurrentControlSet\Control\Session Manager\Power`, "/v", "HiberbootEnabled", "/t", "REG_DWORD", "/d", "1", "/f")
		regCmd.SysProcAttr = hideWindowAttr()
		_ = regCmd.Run()
		if checkWindowsFastStartup() {
			return "Fast startup and hibernation successfully re-enabled.", nil
		}
	}

	// 2. Request UAC elevation via PowerShell Start-Process -Verb RunAs
	psScript := `Start-Process powershell.exe -ArgumentList "-NoProfile -Command powercfg -h on; Set-ItemProperty -Path 'HKLM:\SYSTEM\CurrentControlSet\Control\Session Manager\Power' -Name HiberbootEnabled -Value 1 -Type DWord" -Verb RunAs -WindowStyle Hidden -Wait`
	elevCmd := exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-Command", psScript)
	elevCmd.SysProcAttr = hideWindowAttr()
	elevOut, elevErr := elevCmd.CombinedOutput()
	if elevErr != nil {
		outStr := strings.TrimSpace(string(elevOut))
		low := strings.ToLower(outStr)
		if strings.Contains(low, "canceled by the user") || strings.Contains(low, "cancelled") {
			return "Administrator permission was denied by the user.", elevErr
		}
		if outStr != "" {
			return fmt.Sprintf("Failed to enable fast startup: %s", outStr), elevErr
		}
		return fmt.Sprintf("Failed to enable fast startup (%v). Please run Checkpoint as Administrator.", elevErr), elevErr
	}

	return "Fast startup and hibernation successfully re-enabled.", nil
}

func checkWindowsBitLocker() (bool, string) {
	// Use manage-bde with a strict 1-second timeout and hidden window.
	// Avoid PowerShell to prevent console window flashing.
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
