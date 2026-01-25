package config

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"text/template"
)

type CommandConfig struct {
	Cmd  string   `json:"cmd"`
	Args []string `json:"args"`
}

type GrimConfig struct {
	Cmd        string   `json:"cmd"`
	ArgsAll    []string `json:"args_all"`
	ArgsSingle []string `json:"args_single"`
}

type EditorConfig struct {
	Cmd     string   `json:"cmd"`
	Args    []string `json:"args"`
	ArgsOCR []string `json:"args_ocr"`
}

type CaptureConfig struct {
	Grim      GrimConfig    `json:"grim"`
	Editor    EditorConfig  `json:"editor"`
	OCR       CommandConfig `json:"ocr"`
	Clipboard CommandConfig `json:"clipboard"`
	Notify    CommandConfig `json:"notify"`
}

type Config struct {
	Capture CaptureConfig `json:"capture"`
}

func Default() Config {
	return Config{
		Capture: CaptureConfig{
			Grim: GrimConfig{
				Cmd:        "grim",
				ArgsAll:    []string{"{{.Output}}"},
				ArgsSingle: []string{"-o", "{{.Monitor}}", "{{.Output}}"},
			},
			Editor: EditorConfig{
				Cmd:     "satty",
				Args:    []string{"-f", "{{.Input}}", "--output-filename", "{{.Output}}"},
				ArgsOCR: []string{"-f", "{{.Input}}", "-d", "--disable-notifications", "--output-filename", "{{.Output}}"},
			},
			OCR: CommandConfig{
				Cmd:  "tesseract",
				Args: []string{"{{.Input}}", "stdout", "-l", "eng+ces", "--psm", "6"},
			},
			Clipboard: CommandConfig{
				Cmd:  "wl-copy",
				Args: []string{},
			},
			Notify: CommandConfig{
				Cmd:  "notify-send",
				Args: []string{"{{.Title}}", "{{.Body}}"},
			},
		},
	}
}

func Path() (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("failed to find config dir: %w", err)
	}

	return filepath.Join(configDir, "orego", "config.json"), nil
}

func Load() (Config, error) {
	cfg := Default()

	configPath, err := Path()
	if err != nil {
		return cfg, err
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return cfg, fmt.Errorf("failed to read config: %w", err)
	}

	if err := json.Unmarshal(data, &cfg); err != nil {
		return cfg, fmt.Errorf("failed to parse config: %w", err)
	}

	return cfg, nil
}

func RenderArgs(args []string, data map[string]string) ([]string, error) {
	rendered := make([]string, 0, len(args))
	for _, arg := range args {
		if arg == "" {
			rendered = append(rendered, arg)
			continue
		}
		tmpl, err := template.New("arg").Option("missingkey=error").Parse(arg)
		if err != nil {
			return nil, fmt.Errorf("failed to parse arg template: %w", err)
		}

		var buf bytes.Buffer
		if err := tmpl.Execute(&buf, data); err != nil {
			return nil, fmt.Errorf("failed to render arg template: %w", err)
		}
		rendered = append(rendered, buf.String())
	}

	return rendered, nil
}
