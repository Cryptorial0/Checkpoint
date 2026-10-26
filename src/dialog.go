package main

type DriveInfo struct {
	Path        string `json:"path"`
	Label       string `json:"label"`
	IsRemovable bool   `json:"is_removable"`
}

// OpenFolderPicker opens the native OS directory selection dialog
func OpenFolderPicker(title string) (string, error) {
	return openFolderPickerOS(title)
}

// OpenFilePicker opens the native OS file selection dialog (e.g. for .zip)
func OpenFilePicker(title, filter string) (string, error) {
	return openFilePickerOS(title, filter)
}

// GetAvailableDrives lists mounted drives (USB drives, partitions)
func GetAvailableDrives() []DriveInfo {
	return getAvailableDrivesOS()
}
