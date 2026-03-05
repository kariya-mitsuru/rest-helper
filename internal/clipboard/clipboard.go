// SPDX-License-Identifier: MIT

package clipboard

import (
	"os/exec"
	"strings"
)

// Write copies text to the system clipboard.
// Tries platform-specific commands in order: clip.exe (WSL), pbcopy (macOS),
// wl-copy (Wayland), xclip (X11), xsel (X11).
func Write(text string) error {
	commands := []struct {
		name string
		args []string
	}{
		{"clip.exe", nil},
		{"/mnt/c/Windows/System32/clip.exe", nil}, // WSL2 fallback
		{"pbcopy", nil},
		{"wl-copy", nil},
		{"xclip", []string{"-selection", "clipboard"}},
		{"xsel", []string{"--clipboard", "--input"}},
	}

	for _, c := range commands {
		// Try LookPath first, fall back to direct path for absolute paths
		path, err := exec.LookPath(c.name)
		if err != nil {
			if c.name[0] == '/' {
				path = c.name
			} else {
				continue
			}
		}
		cmd := exec.Command(path, c.args...)
		cmd.Stdin = strings.NewReader(text)
		if err := cmd.Run(); err != nil {
			continue
		}
		return nil
	}

	return errNoClipboard
}

type clipboardError string

func (e clipboardError) Error() string { return string(e) }

const errNoClipboard = clipboardError("no clipboard command available")
