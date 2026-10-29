//go:build (!windows && !cgo) || (nowebview && !windows) || (!windows && !linux)

package main

func runEmbeddedAppWindow(url string) bool {
	return false
}
