package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "orego",
	Short: "Context-Aware Screenshot Tool",
	Long:  `orego captures screenshots with rich metadata (window class, title, workspace state) and stores them in a searchable SQLite database.`,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
