package cmd

import (
	"github.com/spf13/cobra"
)

var uploadCmd = &cobra.Command{
	Use:   "upload",
	Short: "Upload data to MoMorph server",
	Long: `Upload test cases or specs from CSV files to MoMorph server.

Supported file path format:
  .momorph/{testcases|specs}/{file_key}/{frame_id}-{frame_name}.csv

Example:
  .momorph/testcases/i09vM3jClQiu8cwXsMo6uy/9276:19907-TOP_Channel.csv`,
	Example: `  momorph upload testcases .momorph/testcases/**/*.csv
  momorph upload specs --dir .momorph/specs/ -r`,
}

func init() {
	rootCmd.AddCommand(uploadCmd)
}
