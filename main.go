// SPDX-License-Identifier: MIT

package main

import (
	"fmt"
	"os"

	tea "charm.land/bubbletea/v2"

	"rest-helper/internal/app"
	"rest-helper/internal/storage"
)

var version = "dev"

func main() {
	if err := storage.Init(); err != nil {
		fmt.Fprintf(os.Stderr, "Database init error: %v\n", err)
		os.Exit(1)
	}
	defer storage.Close()

	p := tea.NewProgram(app.New(version))

	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
