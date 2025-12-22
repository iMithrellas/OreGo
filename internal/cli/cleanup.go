package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"orego/internal/db"
)

var cleanupCmd = &cobra.Command{
	Use:   "cleanup",
	Short: "Remove database records for missing files",
	Run:   runCleanup,
}

func init() {
	rootCmd.AddCommand(cleanupCmd)
}

func runCleanup(cmd *cobra.Command, args []string) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error getting home dir: %v\n", err)
		os.Exit(1)
	}

	dbPath := filepath.Join(homeDir, ".local/share/orego/orego.db")
	store, err := db.New(dbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error initializing DB: %v\n", err)
		os.Exit(1)
	}
	defer store.Close()

	paths, err := store.ListAllPaths()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error listing paths: %v\n", err)
		os.Exit(1)
	}

	deletedCount := 0
	for id, path := range paths {
		if _, err := os.Stat(path); os.IsNotExist(err) {
			// File is missing, delete record
			fmt.Printf("Removing record for missing file: %s (ID: %d)\n", path, id)
			if err := store.DeleteScreenshot(id); err != nil {
				fmt.Fprintf(os.Stderr, "Error deleting record %d: %v\n", id, err)
			} else {
				deletedCount++
			}
		}
	}

	if deletedCount == 0 {
		fmt.Println("Database is clean. No missing files found.")
	} else {
		fmt.Printf("Cleaned up %d records.\n", deletedCount)
	}
}
