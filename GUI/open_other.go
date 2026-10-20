//go:build !windows

package main

import (
	"os/exec"
	"runtime"
)

// openDefaultBrowser opens the URL with the platform's usual handler on the
// systems that are not Windows.
func openDefaultBrowser(url string) {
	if runtime.GOOS == "darwin" {
		_ = exec.Command("open", url).Start()
		return
	}
	_ = exec.Command("xdg-open", url).Start()
}
