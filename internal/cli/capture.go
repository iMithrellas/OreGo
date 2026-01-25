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
	"orego/internal/config"
	"orego/internal/db"
	"orego/pkg/hyprland"
)

var (
	timeout      time.Duration
	ocr          bool
	all          bool
	grimCmd      string
	editorCmd    string
	ocrCmd       string
	clipboardCmd string
	notifyCmd    string
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
	captureCmd.Flags().StringVar(&grimCmd, "grim-cmd", "grim", "Command used to capture screenshots")
	captureCmd.Flags().StringVar(&editorCmd, "editor-cmd", "satty", "Command used to edit/annotate screenshots")
	captureCmd.Flags().StringVar(&ocrCmd, "ocr-cmd", "tesseract", "Command used to perform OCR")
	captureCmd.Flags().StringVar(&clipboardCmd, "clipboard-cmd", "wl-copy", "Command used to copy OCR text to clipboard")
	captureCmd.Flags().StringVar(&notifyCmd, "notify-cmd", "notify-send", "Command used to send OCR notifications")
	rootCmd.AddCommand(captureCmd)
}

func runOCRFlow(cmd *cobra.Command, tmpPath string) error {
	ocrPath := filepath.Join(
		os.TempDir(),
		fmt.Sprintf("orego-ocr-%d.png", time.Now().UnixNano()),
	)

	cfg, err := config.Load()
	if err != nil {
		return err
	}

	fmt.Println("Opening editor for OCR... (Crop if needed, then click Save)")

	editorCmdToUse := editorCmd
	if !cmd.Flags().Changed("editor-cmd") && cfg.Capture.Editor.Cmd != "" {
		editorCmdToUse = cfg.Capture.Editor.Cmd
	}

	ocrEditorArgs, err := config.RenderArgs(cfg.Capture.Editor.ArgsOCR, map[string]string{
		"Input":  tmpPath,
		"Output": ocrPath,
	})
	if err != nil {
		return err
	}

	if err := exec.Command(editorCmdToUse, ocrEditorArgs...).Run(); err != nil {
		return fmt.Errorf("editor exited: %w", err)
	}

	if _, err := os.Stat(ocrPath); err != nil {
		return nil // user cancelled, not an error
	}
	defer os.Remove(ocrPath)

	ocrCmdToUse := ocrCmd
	if !cmd.Flags().Changed("ocr-cmd") && cfg.Capture.OCR.Cmd != "" {
		ocrCmdToUse = cfg.Capture.OCR.Cmd
	}

	ocrArgs, err := config.RenderArgs(cfg.Capture.OCR.Args, map[string]string{
		"Input": ocrPath,
	})
	if err != nil {
		return err
	}

	ocrCommand := exec.Command(ocrCmdToUse, ocrArgs...)
	text, err := ocrCommand.Output()
	if err != nil {
		return fmt.Errorf("ocr command failed: %w", err)
	}

	clipboardCmdToUse := clipboardCmd
	if !cmd.Flags().Changed("clipboard-cmd") && cfg.Capture.Clipboard.Cmd != "" {
		clipboardCmdToUse = cfg.Capture.Clipboard.Cmd
	}

	clipboardArgs, err := config.RenderArgs(cfg.Capture.Clipboard.Args, map[string]string{})
	if err != nil {
		return err
	}

	wl := exec.Command(clipboardCmdToUse, clipboardArgs...)
	wl.Stdin = bytes.NewReader(text)
	if err := wl.Run(); err != nil {
		return fmt.Errorf("clipboard command failed: %w", err)
	}

	notifyCmdToUse := notifyCmd
	if !cmd.Flags().Changed("notify-cmd") && cfg.Capture.Notify.Cmd != "" {
		notifyCmdToUse = cfg.Capture.Notify.Cmd
	}

	notifyArgs, err := config.RenderArgs(cfg.Capture.Notify.Args, map[string]string{
		"Title": "OCR",
		"Body":  "Text copied to clipboard",
	})
	if err != nil {
		return err
	}

	exec.Command(notifyCmdToUse, notifyArgs...).Run()
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

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
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

	grimCmdToUse := grimCmd
	if !cmd.Flags().Changed("grim-cmd") && cfg.Capture.Grim.Cmd != "" {
		grimCmdToUse = cfg.Capture.Grim.Cmd
	}

	grimArgsTemplate := cfg.Capture.Grim.ArgsAll
	if !all && monitor != "" && monitor != "all-visible" {
		grimArgsTemplate = cfg.Capture.Grim.ArgsSingle
	}

	grimArgs, err := config.RenderArgs(grimArgsTemplate, map[string]string{
		"Monitor": monitor,
		"Output":  tmpPath,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error rendering grim args: %v\n", err)
		os.Exit(1)
	}

	if err := exec.Command(grimCmdToUse, grimArgs...).Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error running grim: %v\n", err)
		os.Exit(1)
	}

	if ocr {
		if err := runOCRFlow(cmd, tmpPath); err != nil {
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

	fmt.Println("Opening editor... (Waiting for you to save and close the window)")
	editorCmdToUse := editorCmd
	if !cmd.Flags().Changed("editor-cmd") && cfg.Capture.Editor.Cmd != "" {
		editorCmdToUse = cfg.Capture.Editor.Cmd
	}

	editorArgs, err := config.RenderArgs(cfg.Capture.Editor.Args, map[string]string{
		"Input":  tmpPath,
		"Output": targetPath,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error rendering editor args: %v\n", err)
		os.Exit(1)
	}

	sattyCmd := exec.Command(editorCmdToUse, editorArgs...)
	sattyCmd.Stdin = os.Stdin
	sattyCmd.Stdout = os.Stdout
	sattyCmd.Stderr = os.Stderr

	if err := sattyCmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Editor exited with: %v\n", err)
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
