# OreGo

Context-Aware Screenshot Tool for Hyprland.

**OreGo** is a CLI tool I wrote to make screenshots a bit smarter on Hyprland. Instead of just dumping files into a folder, it captures metadata like the window title, class, and workspace info and saves it to a local SQLite database. This makes it easier to find old screenshots later without scrolling through thousands of files named `screenshot_123.png`.

## Core Purpose

I wanted a way to search my screenshots.
By storing metadata alongside the file path, you can:
- **Search** for screenshots by app name (e.g., all "Firefox" screenshots).
- **Filter** by window title.
- Keep track of context that is usually lost when you save an image.

## Architecture

It's a static Go binary using Cobra for the CLI and SQLite for storage.

*   **Language:** Go
*   **Database:** SQLite
*   **Tools Used:**
    *   `grim` (for capturing)
    *   `satty` (for editing/annotation) - *Hardcoded for now, but I might make this modular in the future.*
    *   `hyprctl` (to get window info)
    *   `tesseract` (Optional, only for OCR)
    *   `wl-copy` (for clipboard support)

## Features

### 📸 Context-Aware Capture
Fetches the active window's class and title before taking the shot.

### 🔍 Searchable Database
Everything goes into `~/.local/share/orego/orego.db`.
*   **List:** See recent captures.
*   **Filter:** `orego list --filter-by app firefox`

### 🧠 OCR (Optional)
If you have `tesseract` installed, you can use `orego capture --ocr` to grab text from the screen and copy it to your clipboard. It doesn't save the image to the DB in this mode.

### 🧹 Cleanup
`orego cleanup` checks if the files still exist and removes dead rows from the database.

## Installation

### Prerequisites
You'll need `grim`, `satty`, `wl-clipboard`, and `go`. `tesseract` is optional if you want OCR.
```bash
pacman -S grim satty wl-clipboard go
# Optional for OCR:
pacman -S tesseract tesseract-data-eng
```

### Build
```bash
git clone https://github.com/iMithrellas/OreGo.git
cd OreGo
go build -o orego cmd/orego/main.go
sudo mv orego /usr/local/bin/
```

## Usage

### Capture
Takes a screenshot and opens `satty`. The database entry is created only after you save the file.
```bash
# Standard capture
orego capture

# Capture all visible workspaces
orego capture --all

# OCR (Copy text to clipboard)
orego capture --ocr
```

### List & Search
```bash
# List recent
orego list

# Filter
orego list --filter-by app firefox
orego list --filter-by title "GitHub"
```

### View
Open a screenshot by ID.
```bash
orego view 42
```

### Cleanup
```bash
orego cleanup
```

## Configuration (Hyprland)

Put this in your `hyprland.conf`:

```conf
bind = $mainMod, S, exec, orego capture
bind = $mainMod SHIFT, S, exec, orego capture --ocr
bind = $mainMod CTRL, S, exec, orego capture --all
```