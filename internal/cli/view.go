package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"

	"github.com/spf13/cobra"
	"orego/internal/db"
)

var viewCmd = &cobra.Command{
	Use:   "view [id]",
	Short: "Open a screenshot in the default viewer",
	Args:  cobra.ExactArgs(1),
	Run:   runView,
}

func init() {
	rootCmd.AddCommand(viewCmd)
}

func runView(cmd *cobra.Command, args []string) {
	id, err := strconv.ParseInt(args[0], 10, 64)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Invalid ID: %v\n", err)
		os.Exit(1)
	}

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

	path, err := store.GetScreenshotPath(id)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if _, err := os.Stat(path); os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "File no longer exists: %s\n", path)
		fmt.Println("Tip: Run 'orego cleanup' to remove stale records.")
		os.Exit(1)
	}

	fmt.Printf("Opening %s...\n", path)
	if err := exec.Command("xdg-open", path).Start(); err != nil {
		fmt.Fprintf(os.Stderr, "Error opening viewer: %v\n", err)
		os.Exit(1)
	}
}
