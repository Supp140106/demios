package tools

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
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
		p := filepath.Join(filepath.Dir(exe), "embed", "rg.exe")
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
	}
	for _, c := range []string{"embed/rg.exe", "../embed/rg.exe"} {
		if _, err := os.Stat(c); err == nil {
			abs, _ := filepath.Abs(c)
			return abs, nil
		}
	}
	return "", fmt.Errorf("ripgrep (rg.exe) not found on PATH or in embed/ directory")
}
