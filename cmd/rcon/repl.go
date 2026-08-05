package main

import (
	"bufio"
	"context"
	"fmt"
	"strings"
)

// RunInteractive opens a Session and reads commands from Stdin until EOF or a
// quit meta-command. The prompt is written before each line when non-empty.
func (a *App) RunInteractive(ctx context.Context, target Resolved, prompt string) int {
	session, err := a.Client.Dial(ctx, target.Address, target.Password)
	if err != nil {
		_, _ = fmt.Fprintln(a.Stderr, "error:", err)
		return exitCode(err)
	}
	defer func() { _ = session.Close() }()

	scanner := bufio.NewScanner(a.Stdin)
	for {
		if prompt != "" {
			_, _ = fmt.Fprint(a.Stdout, prompt)
		}
		if !scanner.Scan() {
			break
		}
		line := strings.TrimSpace(scanner.Text())
		switch line {
		case "":
			continue
		case ":exit", ":quit":
			return 0
		}
		out, err := session.Execute(ctx, line)
		if err != nil {
			_, _ = fmt.Fprintln(a.Stderr, "error:", err)
			continue
		}
		_, _ = fmt.Fprintln(a.Stdout, out)
	}
	if err := scanner.Err(); err != nil {
		_, _ = fmt.Fprintln(a.Stderr, "error:", err)
		return 2
	}
	return 0
}
