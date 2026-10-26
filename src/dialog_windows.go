//go:build windows

package main

import (
	"fmt"
	"os"
	"syscall"
	"unsafe"
)

var (
	modshell32              = syscall.NewLazyDLL("shell32.dll")
	procSHBrowseForFolderW   = modshell32.NewProc("SHBrowseForFolderW")
	procSHGetPathFromIDListW = modshell32.NewProc("SHGetPathFromIDListW")
	modole32                = syscall.NewLazyDLL("ole32.dll")
	procCoTaskMemFree       = modole32.NewProc("CoTaskMemFree")

	modcomdlg32             = syscall.NewLazyDLL("comdlg32.dll")
	procGetOpenFileNameW    = modcomdlg32.NewProc("GetOpenFileNameW")

	procGetDriveTypeW         = modkernel32.NewProc("GetDriveTypeW")
	procGetVolumeInformationW = modkernel32.NewProc("GetVolumeInformationW")
)

type BROWSEINFOW struct {
	HwndOwner      uintptr
	PidlRoot       uintptr
	PszDisplayName *uint16
	LpszTitle      *uint16
	UlFlags        uint32
	Lpfn           uintptr
	LParam         uintptr
	IImage         int32
}

type OPENFILENAMEW struct {
	LStructSize       uint32
	HwndOwner         uintptr
	HInstance         uintptr
	LpstrFilter       *uint16
	LpstrCustomFilter *uint16
	NMaxCustFilter    uint32
	NFilterIndex      uint32
	LpstrFile         *uint16
	NMaxFile          uint32
	LpstrFileTitle    *uint16
	NMaxFileTitle     uint32
	LpstrInitialDir   *uint16
	LpstrTitle        *uint16
	Flags             uint32
	NFileOffset       uint16
	NFileExtension    uint16
	LpstrDefExt       *uint16
	LCustData         uintptr
	LpfnHook          uintptr
	LpTemplateName    *uint16
	PvReserved        uintptr
	DwReserved        uint32
	FlagsEx           uint32
}

func openFolderPickerOS(title string) (string, error) {
	var bi BROWSEINFOW
	bi.LpszTitle = syscall.StringToUTF16Ptr(title)
	bi.UlFlags = 0x00000040 | 0x00000010 // BIF_NEWDIALOGSTYLE | BIF_EDITBOX

	pidl, _, _ := procSHBrowseForFolderW.Call(uintptr(unsafe.Pointer(&bi)))
	if pidl == 0 {
		return "", nil // Cancelled
	}
	defer procCoTaskMemFree.Call(pidl)

	buf := make([]uint16, 1024)
	ret, _, _ := procSHGetPathFromIDListW.Call(pidl, uintptr(unsafe.Pointer(&buf[0])))
	if ret == 0 {
		return "", fmt.Errorf("could not resolve path from folder dialog")
	}
	return syscall.UTF16ToString(buf), nil
}

func openFilePickerOS(title, filter string) (string, error) {
	var ofn OPENFILENAMEW
	ofn.LStructSize = uint32(unsafe.Sizeof(ofn))
	buf := make([]uint16, 1024)
	ofn.LpstrFile = &buf[0]
	ofn.NMaxFile = 1024
	ofn.LpstrTitle = syscall.StringToUTF16Ptr(title)
	filterText := "ZIP Archives (*.zip)\x00*.zip\x00All Files (*.*)\x00*.*\x00\x00"
	filterUTF16, _ := syscall.UTF16FromString(filterText)
	ofn.LpstrFilter = &filterUTF16[0]
	ofn.Flags = 0x00000800 | 0x00001000 // OFN_PATHMUSTEXIST | OFN_FILEMUSTEXIST

	ret, _, _ := procGetOpenFileNameW.Call(uintptr(unsafe.Pointer(&ofn)))
	if ret == 0 {
		return "", nil // Cancelled
	}
	return syscall.UTF16ToString(buf), nil
}

func getAvailableDrivesOS() []DriveInfo {
	var drives []DriveInfo
	// Instant filesystem scan of drive letters - zero subprocesses, zero powershell
	for r := 'C'; r <= 'Z'; r++ {
		p := string(r) + ":\\"
		if fi, err := os.Stat(p); err == nil && fi.IsDir() {
			utf16Path, _ := syscall.UTF16PtrFromString(p)
			driveType, _, _ := procGetDriveTypeW.Call(uintptr(unsafe.Pointer(utf16Path)))

			var volBuf [256]uint16
			procGetVolumeInformationW.Call(
				uintptr(unsafe.Pointer(utf16Path)),
				uintptr(unsafe.Pointer(&volBuf[0])),
				256,
				0, 0, 0, 0, 0,
			)
			volName := syscall.UTF16ToString(volBuf[:])
			label := p
			if volName != "" {
				label = fmt.Sprintf("%s (%s)", volName, string(r)+":")
			}
			isRemovable := (driveType == 2) // DRIVE_REMOVABLE (USB flash drives, SD cards)
			if isRemovable {
				label += " [USB Drive]"
			}

			drives = append(drives, DriveInfo{
				Path:        p,
				Label:       label,
				IsRemovable: isRemovable,
			})
		}
	}
	return drives
}
