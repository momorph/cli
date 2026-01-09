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
	"context"
	"os"

	"github.com/momorph/cli/internal/logger"
	"github.com/spf13/cobra"
)

var (
	// Global flags
	debugMode bool
	quietMode bool
	// Global context for graceful shutdown
	globalCtx context.Context
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "momorph",
	Short: "MoMorph CLI",
	Example: `  momorph login                         # Log in to MoMorph platform
  momorph init my-project --ai=copilot  # Initialize a new MoMorph project`,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// Initialize logger before any command runs
		return logger.Init(debugMode)
	},
	// Enable command suggestions for typos
	SuggestionsMinimumDistance: 2,
}

func init() {
	// Global flags
	rootCmd.PersistentFlags().BoolVar(&debugMode, "debug", false, "Enable debug logging")
	rootCmd.PersistentFlags().BoolVarP(&quietMode, "quiet", "q", false, "Suppress non-error output")

	// Initialize custom help formatting
	InitHelp()
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

// GetDebugMode returns the current debug mode setting
func GetDebugMode() bool {
	return debugMode
}

// GetQuietMode returns the current quiet mode setting
func GetQuietMode() bool {
	return quietMode
}

// SetContext sets the global context for graceful shutdown support
func SetContext(ctx context.Context) {
	globalCtx = ctx
}

// GetContext returns the global context, or background context if not set
func GetContext() context.Context {
	if globalCtx != nil {
		return globalCtx
	}
	return context.Background()
}
