//go:build !windows && (nowebview || !linux)

package main

func runEmbeddedAppWindow(url string) bool {
	return false
}
