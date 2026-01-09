/*
Copyright © 2025 Sun Asterisk Inc.

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in
all copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
THE SOFTWARE.
*/
package cmd

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

// ASCII banner for MoMorph CLI
const asciiBanner = `
 __  __       __  __                  _
|  \/  | ___ |  \/  | ___  _ __ _ __ | |__
| |\/| |/ _ \| |\/| |/ _ \| '__| '_ \| '_ \
| |  | | (_) | |  | | (_) | |  | |_) | | | |
|_|  |_|\___/|_|  |_|\___/|_|  | .__/|_| |_|
                               |_|
`

// ANSI color codes for terminal output
const (
	colorReset  = "\033[0m"
	colorCyan   = "\033[36m"
	colorYellow = "\033[33m"
	colorGreen  = "\033[32m"
	colorGray   = "\033[90m"
	colorBold   = "\033[1m"
)

// PrintBanner prints the ASCII banner if not in quiet mode
func PrintBanner(w io.Writer) {
	if quietMode {
		return
	}
	if isColorEnabled() {
		fmt.Fprint(w, colorCyan)
	}
	fmt.Fprint(w, asciiBanner)
	if isColorEnabled() {
		fmt.Fprint(w, colorReset)
	}
}

// isColorEnabled checks if color output should be enabled
func isColorEnabled() bool {
	// Disable colors if NO_COLOR env var is set (https://no-color.org/)
	if os.Getenv("NO_COLOR") != "" {
		return false
	}
	// Disable colors if TERM is "dumb"
	if os.Getenv("TERM") == "dumb" {
		return false
	}
	// Check if stdout is a terminal
	if fileInfo, _ := os.Stdout.Stat(); (fileInfo.Mode() & os.ModeCharDevice) == 0 {
		return false
	}
	return true
}

// SetCustomHelp configures custom help templates for the CLI
func SetCustomHelp(cmd *cobra.Command) {
	// Set custom usage template
	cmd.SetUsageTemplate(getUsageTemplate())

	// Set custom help template
	cmd.SetHelpTemplate(getHelpTemplate())

	// Override help function to show banner
	originalHelpFunc := cmd.HelpFunc()
	cmd.SetHelpFunc(func(c *cobra.Command, args []string) {
		// Only show banner for root command
		if c == rootCmd && !quietMode {
			PrintBanner(c.OutOrStdout())
		}
		originalHelpFunc(c, args)
	})
}

// getUsageTemplate returns a custom usage template
func getUsageTemplate() string {
	return `Usage:{{if .Runnable}}
  {{.UseLine}}{{end}}{{if .HasAvailableSubCommands}}
  {{.CommandPath}} [command]{{end}}{{if gt (len .Aliases) 0}}

Aliases:
  {{.NameAndAliases}}{{end}}{{if .HasAvailableSubCommands}}{{$cmds := .Commands}}{{if eq (len .Groups) 0}}

Commands:{{range $cmds}}{{if (or .IsAvailableCommand (eq .Name "help"))}}
  {{rpad .Name .NamePadding }} {{.Short}}{{end}}{{end}}{{else}}{{range $group := .Groups}}

{{.Title}}{{range $cmds}}{{if (and (eq .GroupID $group.ID) (or .IsAvailableCommand (eq .Name "help")))}}
  {{rpad .Name .NamePadding }} {{.Short}}{{end}}{{end}}{{end}}{{if not .AllChildCommandsHaveGroup}}

Subcommands:{{range $cmds}}{{if (and (eq .GroupID "") (or .IsAvailableCommand (eq .Name "help")))}}
  {{rpad .Name .NamePadding }} {{.Short}}{{end}}{{end}}{{end}}{{end}}{{end}}{{if .HasAvailableLocalFlags}}

Flags:
{{.LocalFlags.FlagUsages | trimTrailingWhitespaces}}{{end}}{{if .HasAvailableInheritedFlags}}

Global Flags:
{{.InheritedFlags.FlagUsages | trimTrailingWhitespaces}}{{end}}{{if .HasHelpSubCommands}}

Additional help topics:{{range .Commands}}{{if .IsAdditionalHelpTopicCommand}}
  {{rpad .CommandPath .CommandPathPadding}} {{.Short}}{{end}}{{end}}{{end}}{{if .HasExample}}

Examples:
{{.Example}}{{end}}
`
}

// getHelpTemplate returns a custom help template
func getHelpTemplate() string {
	return `{{with (or .Long .Short)}}{{. | trimTrailingWhitespaces}}

{{end}}{{if or .Runnable .HasSubCommands}}{{.UsageString}}{{end}}`
}

// FormatCommandGroup formats a group of commands for display
func FormatCommandGroup(title string, commands []string) string {
	var sb strings.Builder
	sb.WriteString(title)
	sb.WriteString(":\n")
	for _, cmd := range commands {
		sb.WriteString("  ")
		sb.WriteString(cmd)
		sb.WriteString("\n")
	}
	return sb.String()
}

// FormatHelpSection formats a help section with optional coloring
func FormatHelpSection(title, content string) string {
	var sb strings.Builder
	if isColorEnabled() {
		sb.WriteString(colorBold)
		sb.WriteString(title)
		sb.WriteString(colorReset)
	} else {
		sb.WriteString(title)
	}
	sb.WriteString("\n")
	sb.WriteString(content)
	sb.WriteString("\n")
	return sb.String()
}

// InitHelp sets up the custom help formatting - called from root.go init
func InitHelp() {
	SetCustomHelp(rootCmd)
}
