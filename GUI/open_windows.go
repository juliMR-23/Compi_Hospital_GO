//go:build windows

package main

import "os/exec"

// openDefaultBrowser hands the URL to the Windows shell, which opens it in the
// user's default browser. Used only when no Chromium browser can host the
// chromeless application window.
func openDefaultBrowser(url string) {
	_ = exec.Command("cmd", "/c", "start", "", url).Start()
}
