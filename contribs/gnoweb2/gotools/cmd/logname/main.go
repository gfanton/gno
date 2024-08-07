package main

import (
	"bufio"
	"fmt"
	"os"

	"github.com/charmbracelet/lipgloss"
)

func main() {
	if len(os.Args) < 1 {
		fmt.Fprintln(os.Stderr, "invalid name")
		os.Exit(1)
	}

	name := os.Args[1]

	const width = 12
	if len(name) >= width {
		name = name[:width-3] + "..."
	}

	fg := colorFromString(name, 0.5, 0.6, 90)
	leftBorder := lipgloss.NewStyle().Foreground(fg).
		Border(lipgloss.ThickBorder(), false, true, false, false).
		BorderForeground(fg).
		Bold(true).
		Width(width)

	w, r := os.Stdout, os.Stdin

	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := scanner.Text()
		fmt.Fprint(w, leftBorder.Render(name)+" ")
		fmt.Fprintln(w, line)
	}
}
