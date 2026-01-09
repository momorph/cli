package cmd

import (
	"fmt"
	"runtime"

	"github.com/momorph/cli/internal/version"
	"github.com/spf13/cobra"
)

var versionCmd = &cobra.Command{
	Use:     "version",
	Short:   "Display version information",
	Example: "  momorph version           # Show version info",
	Run:     runVersion,
}

func init() {
	rootCmd.AddCommand(versionCmd)
}

func runVersion(cmd *cobra.Command, args []string) {
	fmt.Printf("MoMorph CLI\n")
	fmt.Printf("  Version:    %s\n", version.Version)
	fmt.Printf("  Commit:     %s\n", version.CommitSHA)
	fmt.Printf("  Built:      %s\n", version.BuildDate)
	fmt.Printf("  Go version: %s\n", runtime.Version())
	fmt.Printf("  OS/Arch:    %s/%s\n", runtime.GOOS, runtime.GOARCH)
}
