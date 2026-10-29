//go:build windows

package main

import (
	"fmt"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"unsafe"
)

var (
	moduser32   = syscall.NewLazyDLL("user32.dll")
	modkernel32 = syscall.NewLazyDLL("kernel32.dll")
	modgdi32    = syscall.NewLazyDLL("gdi32.dll")
	modcomctl32 = syscall.NewLazyDLL("comctl32.dll")

	procRegisterClassExW     = moduser32.NewProc("RegisterClassExW")
	procCreateWindowExW      = moduser32.NewProc("CreateWindowExW")
	procDefWindowProcW       = moduser32.NewProc("DefWindowProcW")
	procPostQuitMessage      = moduser32.NewProc("PostQuitMessage")
	procShowWindow           = moduser32.NewProc("ShowWindow")
	procUpdateWindow         = moduser32.NewProc("UpdateWindow")
	procGetMessageW          = moduser32.NewProc("GetMessageW")
	procTranslateMessage     = moduser32.NewProc("TranslateMessage")
	procDispatchMessageW     = moduser32.NewProc("DispatchMessageW")
	procSendMessageW         = moduser32.NewProc("SendMessageW")
	procSetWindowTextW       = moduser32.NewProc("SetWindowTextW")
	procGetWindowTextW       = moduser32.NewProc("GetWindowTextW")
	procGetModuleHandleW     = modkernel32.NewProc("GetModuleHandleW")
	procGetStockObject       = modgdi32.NewProc("GetStockObject")
	procGetSysColorBrush     = moduser32.NewProc("GetSysColorBrush")
	procEnableWindow         = moduser32.NewProc("EnableWindow")
	procLoadIconW            = moduser32.NewProc("LoadIconW")
	procInitCommonControlsEx = modcomctl32.NewProc("InitCommonControlsEx")
)

const (
	WS_OVERLAPPEDWINDOW = 0x00CF0000
	WS_VISIBLE          = 0x10000000
	WS_CHILD            = 0x40000000
	WS_BORDER           = 0x00800000
	WS_TABSTOP          = 0x00010000
	WS_VSCROLL          = 0x00200000
	WS_EX_CLIENTEDGE    = 0x00000200

	BS_PUSHBUTTON   = 0x00000000
	BS_AUTOCHECKBOX = 0x00000003
	ES_AUTOHSCROLL  = 0x00000080
	LBS_NOTIFY      = 0x0001

	WM_DESTROY        = 0x0002
	WM_COMMAND        = 0x0111
	WM_SETFONT        = 0x0030
	WM_CTLCOLORSTATIC = 0x0138

	DEFAULT_GUI_FONT = 17

	COLOR_BTNFACE = 15

	// Control IDs
	ID_BTN_SCAN     = 1001
	ID_BTN_BACKUP   = 1002
	ID_BTN_BROWSE_S = 1003
	ID_BTN_BROWSE_D = 1004
	ID_BTN_FASTBOOT = 1005
	ID_BTN_UNDOBOOT = 1006
	ID_LST_GAMES    = 1007
	ID_EDT_SCAN     = 1008
	ID_EDT_DEST     = 1009
	ID_BTN_OPEN_WEB = 1010
)

type INITCOMMONCONTROLSEX struct {
	DwSize uint32
	DwICC  uint32
}

type WNDCLASSEXW struct {
	CbSize        uint32
	Style         uint32
	LpfnWndProc   uintptr
	CbClsExtra    int32
	CbWndExtra    int32
	HInstance     uintptr
	HIcon         uintptr
	HCursor       uintptr
	HbrBackground uintptr
	LpszMenuName  *uint16
	LpszClassName *uint16
	HIconSm       uintptr
}

type POINT struct {
	X, Y int32
}

type MSG struct {
	Hwnd     uintptr
	Message  uint32
	WParam   uintptr
	LParam   uintptr
	Time     uint32
	Pt       POINT
	LPrivate uint32
}

var (
	hMainWnd     uintptr
	hBtnScan     uintptr
	hBtnBackup   uintptr
	hBtnBrowseS  uintptr
	hBtnBrowseD  uintptr
	hBtnFastboot uintptr
	hBtnUndoBoot uintptr
	hBtnOpenWeb  uintptr
	hLstGames    uintptr
	hEdtScan     uintptr
	hEdtDest     uintptr
	hLblStatus   uintptr
	hLblFastboot uintptr
	hFont        uintptr

	guiDetectedGames []DetectedGame
)

func runNativeWindowsGUI() {
	// Lock this goroutine to its OS thread so Windows message queues never detach
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	defer func() {
		if r := recover(); r != nil {
			Log("FATAL ERROR in Win32 GUI: %v", r)
		}
	}()

	Log("Initializing Win32 native GUI...")

	// Initialize common controls
	var icex INITCOMMONCONTROLSEX
	icex.DwSize = uint32(unsafe.Sizeof(icex))
	icex.DwICC = 0x000040FF // Standard + Win95 classes
	procInitCommonControlsEx.Call(uintptr(unsafe.Pointer(&icex)))

	hInstance, _, _ := procGetModuleHandleW.Call(0)
	hFont, _, _ = procGetStockObject.Call(DEFAULT_GUI_FONT)

	hbrFace, _, _ := procGetSysColorBrush.Call(COLOR_BTNFACE)
	hAppIcon, _, _ := procLoadIconW.Call(hInstance, uintptr(1))

	className := syscall.StringToUTF16Ptr("CheckpointWin32Class")

	var wc WNDCLASSEXW
	wc.CbSize = uint32(unsafe.Sizeof(wc))
	wc.Style = 0x0002 | 0x0001 // CS_HREDRAW | CS_VREDRAW
	wc.LpfnWndProc = syscall.NewCallback(wndProc)
	wc.HInstance = hInstance
	wc.HIcon = hAppIcon
	wc.HIconSm = hAppIcon
	wc.HbrBackground = hbrFace
	wc.LpszClassName = className

	procRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc)))

	title := syscall.StringToUTF16Ptr("Checkpoint (v" + AppVersion + ")")
	hMainWnd, _, _ = procCreateWindowExW.Call(
		0,
		uintptr(unsafe.Pointer(className)),
		uintptr(unsafe.Pointer(title)),
		uintptr(WS_OVERLAPPEDWINDOW|WS_VISIBLE),
		150, 80, 840, 680,
		0, 0, hInstance, 0,
	)

	Log(fmt.Sprintf("Main window created (handle: 0x%x)", hMainWnd))
	createChildControls(hMainWnd, hInstance)

	procShowWindow.Call(hMainWnd, 1)
	procUpdateWindow.Call(hMainWnd)
	Log("Main window displayed and refreshed.")

	// Run system checks in background to never block GUI responsiveness
	go refreshSystemStatus()

	Log("Entering Windows message pump loop...")
	var msg MSG
	for {
		r, _, _ := procGetMessageW.Call(uintptr(unsafe.Pointer(&msg)), 0, 0, 0)
		if int32(r) <= 0 {
			break
		}
		procTranslateMessage.Call(uintptr(unsafe.Pointer(&msg)))
		procDispatchMessageW.Call(uintptr(unsafe.Pointer(&msg)))
	}
	Log("Exited Windows message pump loop.")
}

func createChildControls(parent, hInstance uintptr) {
	btnClass := syscall.StringToUTF16Ptr("BUTTON")
	editClass := syscall.StringToUTF16Ptr("EDIT")
	staticClass := syscall.StringToUTF16Ptr("STATIC")
	listClass := syscall.StringToUTF16Ptr("LISTBOX")

	makeControl := func(class *uint16, text string, style uint32, x, y, w, h int32, id int) uintptr {
		t := syscall.StringToUTF16Ptr(text)
		hCtrl, _, _ := procCreateWindowExW.Call(
			0,
			uintptr(unsafe.Pointer(class)),
			uintptr(unsafe.Pointer(t)),
			uintptr(style|WS_CHILD|WS_VISIBLE),
			uintptr(x), uintptr(y), uintptr(w), uintptr(h),
			parent, uintptr(id), hInstance, 0,
		)
		if hFont != 0 {
			procSendMessageW.Call(hCtrl, WM_SETFONT, hFont, 1)
		}
		return hCtrl
	}

	// 1. Banner & Web UI Link
	makeControl(staticClass, "Checkpoint: Game Save & Emulator Migration (v"+AppVersion+")", 0, 20, 15, 450, 20, 0)
	hBtnOpenWeb = makeControl(btnClass, "Open Web Dashboard", BS_PUSHBUTTON|WS_TABSTOP, 480, 12, 330, 26, ID_BTN_OPEN_WEB)

	// 2. System Safety Group
	makeControl(staticClass, "System & Drive Safety:", 0, 20, 48, 200, 20, 0)
	hLblFastboot = makeControl(staticClass, "Checking Fast Startup...", 0, 20, 71, 420, 20, 0)
	hBtnFastboot = makeControl(btnClass, "Disable Fast Startup", BS_PUSHBUTTON|WS_TABSTOP, 460, 68, 160, 26, ID_BTN_FASTBOOT)
	hBtnUndoBoot = makeControl(btnClass, "Undo / Re-enable", BS_PUSHBUTTON|WS_TABSTOP, 630, 68, 140, 26, ID_BTN_UNDOBOOT)

	// 3. Scan Settings
	makeControl(staticClass, "Custom Scan Path (Optional, e.g. D:\\Games):", 0, 20, 105, 300, 20, 0)
	hEdtScan = makeControl(editClass, "", WS_BORDER|ES_AUTOHSCROLL|WS_TABSTOP, 20, 128, 540, 26, ID_EDT_SCAN)
	hBtnBrowseS = makeControl(btnClass, "Browse...", BS_PUSHBUTTON|WS_TABSTOP, 570, 128, 90, 26, ID_BTN_BROWSE_S)
	hBtnScan = makeControl(btnClass, "Scan Games", BS_PUSHBUTTON|WS_TABSTOP, 670, 128, 120, 26, ID_BTN_SCAN)

	// 4. Detected Games List
	makeControl(staticClass, "Detected Saves (Emulators, In-Tree & Non-Cloud Titles):", 0, 20, 168, 500, 20, 0)
	hLstGames = makeControl(listClass, "", WS_BORDER|WS_VSCROLL|LBS_NOTIFY|WS_TABSTOP, 20, 192, 770, 240, ID_LST_GAMES)

	// 5. Destination & Action
	makeControl(staticClass, "Backup Destination Folder (e.g. USB Flash Drive E:\\):", 0, 20, 450, 400, 20, 0)
	hEdtDest = makeControl(editClass, "", WS_BORDER|ES_AUTOHSCROLL|WS_TABSTOP, 20, 472, 650, 26, ID_EDT_DEST)
	hBtnBrowseD = makeControl(btnClass, "Browse...", BS_PUSHBUTTON|WS_TABSTOP, 680, 472, 110, 26, ID_BTN_BROWSE_D)

	hBtnBackup = makeControl(btnClass, "Start Backup (Single ZIP)", BS_PUSHBUTTON|WS_TABSTOP, 20, 520, 220, 36, ID_BTN_BACKUP)
	hLblStatus = makeControl(staticClass, "Ready. Click 'Scan Games' to find save files.", 0, 260, 530, 530, 24, 0)

	Log("Child controls created successfully.")
}

func wndProc(hwnd, msg, wParam, lParam uintptr) uintptr {
	switch msg {
	case WM_COMMAND:
		id := int(loword(wParam))
		switch id {
		case ID_BTN_SCAN:
			go onScanClicked()
		case ID_BTN_BACKUP:
			go onBackupClicked()
		case ID_BTN_BROWSE_S:
			go func() {
				p, err := OpenFolderPicker("Select Additional Game Folder")
				if err == nil && p != "" {
					setWindowText(hEdtScan, p)
				}
			}()
		case ID_BTN_BROWSE_D:
			go func() {
				p, err := OpenFolderPicker("Select Backup Destination (e.g. USB Drive)")
				if err == nil && p != "" {
					setWindowText(hEdtDest, p)
				}
			}()
		case ID_BTN_FASTBOOT:
			go func() {
				setWindowText(hLblStatus, "Disabling Fast Startup...")
				msg, _ := DisableFastStartup()
				setWindowText(hLblStatus, msg)
				refreshSystemStatus()
			}()
		case ID_BTN_UNDOBOOT:
			go func() {
				setWindowText(hLblStatus, "Re-enabling Fast Startup...")
				msg, _ := EnableFastStartup()
				setWindowText(hLblStatus, msg)
				refreshSystemStatus()
			}()
		case ID_BTN_OPEN_WEB:
			go launchWebFallback()
		}
	case WM_CTLCOLORSTATIC:
		// Return valid GDI brush for background
		hbr, _, _ := procGetSysColorBrush.Call(COLOR_BTNFACE)
		return hbr
	case WM_DESTROY:
		Log("WM_DESTROY received. Exiting GUI.")
		procPostQuitMessage.Call(0)
		return 0
	}
	r, _, _ := procDefWindowProcW.Call(hwnd, msg, wParam, lParam)
	return r
}

func refreshSystemStatus() {
	Log("Checking system status in background...")
	status := CheckSystem()
	if status.FastStartupActive {
		setWindowText(hLblFastboot, "Fast Startup: Active (Locks storage partitions on shutdown)")
		procEnableWindow.Call(hBtnFastboot, 1)
		procEnableWindow.Call(hBtnUndoBoot, 0)
	} else {
		setWindowText(hLblFastboot, "Fast Startup: Disabled (Storage partitions unmount cleanly)")
		procEnableWindow.Call(hBtnFastboot, 0)
		procEnableWindow.Call(hBtnUndoBoot, 1)
	}

	if status.BitLockerWarning {
		setWindowText(hLblStatus, status.BitLockerDetails)
	}
	Log("System status check completed.")
}

func onScanClicked() {
	Log("User clicked 'Scan Games'...")
	setWindowText(hLblStatus, "Scanning for save files and emulators...")
	procSendMessageW.Call(hLstGames, 0x0184, 0, 0) // LB_RESETCONTENT

	customPath := getWindowText(hEdtScan)
	var roots []string
	if customPath != "" {
		roots = append(roots, customPath)
	}

	games, err := ScanInstalledGames(roots)
	if err != nil {
		Log("Scan failed with error: %v", err)
		setWindowText(hLblStatus, "Scan error: "+err.Error())
		return
	}
	guiDetectedGames = games
	Log(fmt.Sprintf("Found %d games with saves", len(games)))

	for _, g := range games {
		cloudTag := ""
		if g.IsSteamCloud {
			cloudTag = " [Steam Cloud]"
		}
		itemText := fmt.Sprintf("%s (%s, %s)%s", g.Name, g.StorageType, formatBytes(g.SizeBytes), cloudTag)
		ptr := syscall.StringToUTF16Ptr(itemText)
		procSendMessageW.Call(hLstGames, 0x0180, 0, uintptr(unsafe.Pointer(ptr))) // LB_ADDSTRING
	}

	online := ScanOnlineGames()
	for _, o := range online {
		itemText := fmt.Sprintf("[Online/Cloud] %s: %s (%s)", o.Name, o.Status, strings.Join(o.AntiCheats, ", "))
		ptr := syscall.StringToUTF16Ptr(itemText)
		procSendMessageW.Call(hLstGames, 0x0180, 0, uintptr(unsafe.Pointer(ptr)))
	}

	setWindowText(hLblStatus, fmt.Sprintf("Found %d save-based game(s) and %d online title(s).", len(games), len(online)))
}

func onBackupClicked() {
	dest := getWindowText(hEdtDest)
	if dest == "" {
		setWindowText(hLblStatus, "Please select a backup destination folder first.")
		return
	}
	if len(guiDetectedGames) == 0 {
		setWindowText(hLblStatus, "No games scanned yet. Click 'Scan Games' first.")
		return
	}

	setWindowText(hLblStatus, fmt.Sprintf("Archiving %d games into single ZIP...", len(guiDetectedGames)))
	procEnableWindow.Call(hBtnBackup, 0)
	defer procEnableWindow.Call(hBtnBackup, 1)

	Log("Starting backup archive...")
	_, err := CreateBackup(guiDetectedGames, dest, true)
	if err != nil {
		Log("Backup error: %v", err)
		setWindowText(hLblStatus, "Backup error: "+err.Error())
	} else {
		Log("Backup successfully created at " + dest)
		setWindowText(hLblStatus, "Backup completed successfully! Created: "+filepath.Join(dest, "game_saves_bundle.zip"))
	}
}

func launchWebFallback() {
	Log("Launching web interface on demand...")
	url := "http://127.0.0.1:9119"
	launchAppWindow(url)
}

func setWindowText(hwnd uintptr, text string) {
	ptr := syscall.StringToUTF16Ptr(text)
	procSetWindowTextW.Call(hwnd, uintptr(unsafe.Pointer(ptr)))
}

func getWindowText(hwnd uintptr) string {
	buf := make([]uint16, 512)
	procGetWindowTextW.Call(hwnd, uintptr(unsafe.Pointer(&buf[0])), 512)
	return syscall.UTF16ToString(buf)
}

func loword(l uintptr) uint16 {
	return uint16(l & 0xFFFF)
}
