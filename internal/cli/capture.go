package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"
	"orego/internal/db"
	"orego/pkg/hyprland"
)

var (
	timeout time.Duration
	ocr     bool
	all     bool
)

var captureCmd = &cobra.Command{
	Use:   "capture",
	Short: "Capture a screenshot with metadata",
	Run:   runCapture,
}

func init() {
	captureCmd.Flags().DurationVar(&timeout, "timeout", 5*time.Second, "Max time to wait for file creation after editor closes")
	captureCmd.Flags().BoolVar(&ocr, "ocr", false, "Perform OCR and copy to clipboard (no DB save)")
	captureCmd.Flags().BoolVar(&all, "all", false, "Capture all visible workspaces")
	rootCmd.AddCommand(captureCmd)
}

func runOCRFlow(tmpPath string) error {
	ocrPath := filepath.Join(
		os.TempDir(),
		fmt.Sprintf("orego-ocr-%d.png", time.Now().UnixNano()),
	)

	fmt.Println("Opening satty for OCR... (Crop if needed, then click Save)")

	sattyArgs := []string{
		"-f", tmpPath,
		"-d", "--disable-notifications",
		"--output-filename", ocrPath,
	}

	if err := exec.Command("satty", sattyArgs...).Run(); err != nil {
		return fmt.Errorf("satty exited: %w", err)
	}

	if _, err := os.Stat(ocrPath); err != nil {
		return nil // user cancelled, not an error
	}
	defer os.Remove(ocrPath)

	ocrCmd := exec.Command(
		"tesseract",
		ocrPath,
		"stdout",
		"-l", "eng+ces",
		"--psm", "6",
	)
	text, err := ocrCmd.Output()
	if err != nil {
		return fmt.Errorf("tesseract failed: %w", err)
	}

	wl := exec.Command("wl-copy")
	wl.Stdin = bytes.NewReader(text)
	if err := wl.Run(); err != nil {
		return fmt.Errorf("wl-copy failed: %w", err)
	}

	exec.Command("notify-send", "OCR", "Text copied to clipboard").Run()
	return nil
}

func runCapture(cmd *cobra.Command, args []string) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error getting home dir: %v\n", err)
		os.Exit(1)
	}

	data, err := hyprland.GetScreenshotData(all)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error fetching data: %v\n", err)
		os.Exit(1)
	}

	tmpFile, err := os.CreateTemp("", "orego-raw-*.png")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating temp file: %v\n", err)
		os.Exit(1)
	}
	tmpPath := tmpFile.Name()
	tmpFile.Close()
	defer os.Remove(tmpPath)

	monitor := data.Workspace.Monitor

	grimArgs := []string{tmpPath}
	if !all && monitor != "" && monitor != "all-visible" {
		grimArgs = []string{"-o", monitor, tmpPath}
	}

	if err := exec.Command("grim", grimArgs...).Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error running grim: %v\n", err)
		os.Exit(1)
	}

	if ocr {
		if err := runOCRFlow(tmpPath); err != nil {
			fmt.Fprintf(os.Stderr, "OCR failed: %v\n", err)
			os.Exit(1)
		}
		return // Exit without saving to DB
	}

	dbPath := filepath.Join(homeDir, ".local/share/orego/orego.db")
	store, err := db.New(dbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error initializing DB: %v\n", err)
		os.Exit(1)
	}
	defer store.Close()

	screenshotsDir := filepath.Join(homeDir, "Pictures", "Screenshots")
	if err := os.MkdirAll(screenshotsDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Error creating screenshots dir: %v\n", err)
		os.Exit(1)
	}

	timestamp := time.Now().Format("2006-01-02_15-04-05")
	targetFilename := fmt.Sprintf("%s_orego.png", timestamp)
	targetPath := filepath.Join(screenshotsDir, targetFilename)

	fmt.Println("Opening satty... (Waiting for you to save and close the window)")
	sattyCmd := exec.Command("satty", "-f", tmpPath, "--output-filename", targetPath)
	sattyCmd.Stdin = os.Stdin
	sattyCmd.Stdout = os.Stdout
	sattyCmd.Stderr = os.Stderr

	if err := sattyCmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Satty exited with: %v\n", err)
	}

	start := time.Now()
	found := false
	for {
		if _, err := os.Stat(targetPath); err == nil {
			found = true
			break
		}
		if time.Since(start) > timeout {
			break
		}
		time.Sleep(500 * time.Millisecond)
		fmt.Print(".") // Feedback dot
	}
	if !found {
		fmt.Println()
		fmt.Fprintln(os.Stderr, "Screenshot discarded (not saved within timeout).")
		return
	}
	fmt.Println()

	data.FilePath = targetPath
	if err := store.Save(data); err != nil {
		fmt.Fprintf(os.Stderr, "Error saving to DB: %v\n", err)
		os.Exit(1)
	}

	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	encoder.Encode(data)
}
