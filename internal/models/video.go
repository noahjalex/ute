package models

import (
	"encoding/json"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Video represents a downloaded video with metadata
type Video struct {
	ID          string    `json:"id"`
	Title       string    `json:"title"`
	Filename    string    `json:"filename"`
	FilePath    string    `json:"file_path"`
	FileSize    int64     `json:"file_size"`
	Duration    float64   `json:"duration"`
	Thumbnail   string    `json:"thumbnail"`
	UploadDate  string    `json:"upload_date"`
	Uploader    string    `json:"uploader"`
	Description string    `json:"description"`
	URL         string    `json:"url"`
	CreatedAt   time.Time `json:"created_at"`
}

// VideoMetadata represents metadata extracted from yt-dlp
type VideoMetadata struct {
	ID          string  `json:"id"`
	Title       string  `json:"title"`
	Duration    float64 `json:"duration"`
	Uploader    string  `json:"uploader"`
	UploadDate  string  `json:"upload_date"`
	Description string  `json:"description"`
	Thumbnail   string  `json:"thumbnail"`
	WebpageURL  string  `json:"webpage_url"`
}

// ExtractVideoMetadata uses yt-dlp to extract metadata without downloading
func ExtractVideoMetadata(url string) (*VideoMetadata, error) {
	cmd := exec.Command("yt-dlp", "--dump-json", "--no-download", url)
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	var metadata VideoMetadata
	if err := json.Unmarshal(output, &metadata); err != nil {
		return nil, err
	}

	return &metadata, nil
}

// SanitizeFilename removes potentially dangerous characters from filenames
func SanitizeFilename(filename string) string {
	// Remove path separators and other dangerous characters
	filename = strings.ReplaceAll(filename, "/", "_")
	filename = strings.ReplaceAll(filename, "\\", "_")
	filename = strings.ReplaceAll(filename, "..", "_")
	filename = strings.ReplaceAll(filename, ":", "_")
	filename = strings.ReplaceAll(filename, "*", "_")
	filename = strings.ReplaceAll(filename, "?", "_")
	filename = strings.ReplaceAll(filename, "\"", "_")
	filename = strings.ReplaceAll(filename, "<", "_")
	filename = strings.ReplaceAll(filename, ">", "_")
	filename = strings.ReplaceAll(filename, "|", "_")

	return filename
}

// FormatDuration converts seconds to human-readable format
func (v *Video) FormatDuration() string {
	if v.Duration <= 0 {
		return "Unknown"
	}

	hours := int(v.Duration) / 3600
	minutes := (int(v.Duration) % 3600) / 60
	seconds := int(v.Duration) % 60

	if hours > 0 {
		return strconv.Itoa(hours) + "h " + strconv.Itoa(minutes) + "m " + strconv.Itoa(seconds) + "s"
	} else if minutes > 0 {
		return strconv.Itoa(minutes) + "m " + strconv.Itoa(seconds) + "s"
	}
	return strconv.Itoa(seconds) + "s"
}

// FormatFileSize converts bytes to human-readable format
func (v *Video) FormatFileSize() string {
	if v.FileSize <= 0 {
		return "Unknown"
	}

	const unit = 1024
	if v.FileSize < unit {
		return strconv.FormatInt(v.FileSize, 10) + " B"
	}

	div, exp := int64(unit), 0
	for n := v.FileSize / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}

	return strconv.FormatFloat(float64(v.FileSize)/float64(div), 'f', 1, 64) + " " + []string{"B", "KB", "MB", "GB", "TB"}[exp+1]
}

// GetRelativePath returns the path relative to the downloads directory
func (v *Video) GetRelativePath(downloadsDir string) string {
	rel, err := filepath.Rel(downloadsDir, v.FilePath)
	if err != nil {
		return v.Filename
	}
	return rel
}
