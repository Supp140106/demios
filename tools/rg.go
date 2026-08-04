package tools

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

func findRg() (string, error) {
	if p, err := exec.LookPath("rg"); err == nil {
		return p, nil
	}
	if p, err := exec.LookPath("rg.exe"); err == nil {
		return p, nil
	}
	exe, err := os.Executable()
	if err == nil {
		embedName := "rg.exe"
		if runtime.GOOS != "windows" {
			embedName = "rg"
		}
		p := filepath.Join(filepath.Dir(exe), "embed", embedName)
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
	}
	embedName := "rg.exe"
	if runtime.GOOS != "windows" {
		embedName = "rg"
	}
	for _, c := range []string{filepath.Join("embed", embedName), filepath.Join("..", "embed", embedName)} {
		if _, err := os.Stat(c); err == nil {
			abs, _ := filepath.Abs(c)
			return abs, nil
		}
	}
	return "", fmt.Errorf("ripgrep (rg) not found on PATH or in embed/ directory")
}
