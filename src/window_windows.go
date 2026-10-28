//go:build windows

package main

import (
	"os"
	"path/filepath"

	"github.com/jchv/go-webview2"
)

func runEmbeddedAppWindow(url string) bool {
	defer func() {
		if r := recover(); r != nil {
			Log("WebView2 host initialization failed: %v", r)
		}
	}()

	tempDir := os.Getenv("TEMP")
	if tempDir == "" {
		tempDir = `C:\Temp`
	}
	dataPath := filepath.Join(tempDir, "save_migrator_webview2")

	opts := webview2.WebViewOptions{
		Debug:    false,
		DataPath: dataPath,
		WindowOptions: webview2.WindowOptions{
			Title:  "Checkpoint (v1.0.0)",
			Width:  980,
			Height: 820,
			Center: true,
		},
	}

	w := webview2.NewWithOptions(opts)
	if w == nil {
		Log("WebView2 runtime not available on this system.")
		return false
	}
	defer w.Destroy()

	w.SetTitle("Checkpoint (v1.0.0)")
	w.SetSize(980, 820, webview2.HintNone)
	w.Navigate(url)

	Log("Embedded native desktop window running (WebView2)...")
	w.Run()
	Log("Embedded native desktop window closed.")
	return true
}
