package services

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"noahjalex.ute/internal/models"
)

// VideoService handles video operations
type VideoService struct {
	DownloadsDir string
	MetadataFile string
	videos       map[string]*models.Video
}

// NewVideoService creates a new video service
func NewVideoService(downloadsDir string) *VideoService {
	metadataFile := filepath.Join(downloadsDir, "metadata.json")
	vs := &VideoService{
		DownloadsDir: downloadsDir,
		MetadataFile: metadataFile,
		videos:       make(map[string]*models.Video),
	}

	// Ensure downloads directory exists
	os.MkdirAll(downloadsDir, 0755)

	// Load existing metadata
	vs.LoadMetadata()

	// Scan for existing videos not in metadata
	vs.ScanForExistingVideos()

	return vs
}

// SecurePath validates and returns a secure path within the downloads directory
func (vs *VideoService) SecurePath(requestedPath string) (string, error) {
	// Clean the path to resolve any .. or . components
	cleanPath := filepath.Clean(requestedPath)

	// Join with downloads directory
	fullPath := filepath.Join(vs.DownloadsDir, cleanPath)

	// Get absolute paths for comparison
	absDownloadsDir, err := filepath.Abs(vs.DownloadsDir)
	if err != nil {
		return "", fmt.Errorf("failed to get absolute downloads directory: %w", err)
	}

	absFullPath, err := filepath.Abs(fullPath)
	if err != nil {
		return "", fmt.Errorf("failed to get absolute requested path: %w", err)
	}

	// Ensure the resolved path is within the downloads directory
	if !strings.HasPrefix(absFullPath, absDownloadsDir+string(filepath.Separator)) &&
		absFullPath != absDownloadsDir {
		return "", fmt.Errorf("path traversal attempt detected: %s", requestedPath)
	}

	return absFullPath, nil
}

// LoadMetadata loads video metadata from file
func (vs *VideoService) LoadMetadata() error {
	if _, err := os.Stat(vs.MetadataFile); os.IsNotExist(err) {
		return nil // No metadata file yet
	}

	data, err := os.ReadFile(vs.MetadataFile)
	if err != nil {
		return err
	}

	var videos []*models.Video
	if err := json.Unmarshal(data, &videos); err != nil {
		return err
	}

	vs.videos = make(map[string]*models.Video)
	for _, video := range videos {
		vs.videos[video.ID] = video
	}

	return nil
}

// SaveMetadata saves video metadata to file
func (vs *VideoService) SaveMetadata() error {
	videos := make([]*models.Video, 0, len(vs.videos))
	for _, video := range vs.videos {
		videos = append(videos, video)
	}

	data, err := json.MarshalIndent(videos, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(vs.MetadataFile, data, 0644)
}

// ScanForExistingVideos scans the downloads directory for video files not in metadata
func (vs *VideoService) ScanForExistingVideos() error {
	extensions := []string{".mp4", ".mkv", ".webm", ".avi", ".mov", ".flv", ".m4v"}

	return filepath.WalkDir(vs.DownloadsDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() || d.Name() == "metadata.json" {
			return nil
		}

		filename := d.Name()

		// Skip partial files
		if strings.Contains(filename, ".part") || strings.Contains(filename, ".ytdl") {
			return nil
		}

		ext := filepath.Ext(filename)
		isVideo := false
		for _, videoExt := range extensions {
			if strings.EqualFold(ext, videoExt) {
				isVideo = true
				break
			}
		}

		if !isVideo {
			return nil
		}

		// Check if we already have this file in metadata
		for _, video := range vs.videos {
			fmt.Printf("vid: %#v\n", video)
			if video.FilePath == path {
				return nil // Already indexed
			}
		}

		// Create a basic video record for existing files
		info, err := d.Info()
		if err != nil {
			return nil // Skip files we can't stat
		}

		// Generate a simple ID based on filename
		baseFilename := strings.TrimSuffix(filename, ext)
		videoID := fmt.Sprintf("existing_%s_%d", baseFilename, info.ModTime().Unix())

		// Look for thumbnail
		thumbnailPath := vs.findThumbnailFile(baseFilename, videoID)

		video := &models.Video{
			ID:          videoID,
			Title:       baseFilename,
			Filename:    filename,
			FilePath:    path,
			FileSize:    info.Size(),
			Duration:    0, // Unknown for existing files
			Thumbnail:   thumbnailPath,
			UploadDate:  "",
			Uploader:    "",
			Description: "Existing video file",
			URL:         "",
			CreatedAt:   info.ModTime(),
		}

		vs.videos[videoID] = video

		return nil
	})
}

// DownloadVideo downloads a video and returns progress updates via channel
func (vs *VideoService) DownloadVideo(url string, progressChan chan<- string) error {
	defer close(progressChan)

	// First, extract metadata
	progressChan <- "Extracting video information..."
	metadata, err := models.ExtractVideoMetadata(url)
	if err != nil {
		return fmt.Errorf("failed to extract metadata: %w", err)
	}

	// Sanitize filename
	filename := models.SanitizeFilename(metadata.Title)
	if filename == "" {
		filename = metadata.ID
	}

	progressChan <- fmt.Sprintf("Starting download: %s", metadata.Title)

	// Download the video
	cmd := exec.Command("yt-dlp", url, "-P", vs.DownloadsDir, "-o", "%(title)s.%(ext)s", "--write-thumbnail")

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}

	if err := cmd.Start(); err != nil {
		return err
	}

	// Stream output
	go func() {
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			progressChan <- scanner.Text()
		}
	}()

	go func() {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			progressChan <- scanner.Text()
		}
	}()

	if err := cmd.Wait(); err != nil {
		return fmt.Errorf("download failed: %w", err)
	}

	progressChan <- "Download completed, processing metadata..."

	// Find the downloaded file
	downloadedFile, err := vs.findDownloadedFile(metadata.Title, metadata.ID)
	if err != nil {
		return fmt.Errorf("failed to find downloaded file: %w", err)
	}

	// Get file info
	fileInfo, err := os.Stat(downloadedFile)
	if err != nil {
		return fmt.Errorf("failed to get file info: %w", err)
	}

	// Create video record
	video := &models.Video{
		ID:          metadata.ID,
		Title:       metadata.Title,
		Filename:    filepath.Base(downloadedFile),
		FilePath:    downloadedFile,
		FileSize:    fileInfo.Size(),
		Duration:    metadata.Duration,
		Thumbnail:   vs.findThumbnailFile(metadata.Title, metadata.ID),
		UploadDate:  metadata.UploadDate,
		Uploader:    metadata.Uploader,
		Description: metadata.Description,
		URL:         url,
		CreatedAt:   time.Now(),
	}

	// Store metadata
	vs.videos[video.ID] = video
	if err := vs.SaveMetadata(); err != nil {
		return fmt.Errorf("failed to save metadata: %w", err)
	}

	progressChan <- "Video successfully downloaded and indexed!"

	return nil
}

// findDownloadedFile finds the downloaded video file
func (vs *VideoService) findDownloadedFile(title, id string) (string, error) {
	// Common video extensions
	extensions := []string{".mp4", ".mkv", ".webm", ".avi", ".mov", ".flv", ".m4v"}

	// Try with exact title first
	for _, ext := range extensions {
		path := filepath.Join(vs.DownloadsDir, title+ext)
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
	}

	// Try with sanitized title
	sanitizedTitle := models.SanitizeFilename(title)
	for _, ext := range extensions {
		path := filepath.Join(vs.DownloadsDir, sanitizedTitle+ext)
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
	}

	// Try with ID
	for _, ext := range extensions {
		path := filepath.Join(vs.DownloadsDir, id+ext)
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
	}

	// Search directory for any video files (not just recent ones)
	var foundFile string
	filepath.WalkDir(vs.DownloadsDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if !d.IsDir() {
			ext := filepath.Ext(path)
			filename := d.Name()

			// Skip partial files
			if strings.Contains(filename, ".part") || strings.Contains(filename, ".ytdl") {
				return nil
			}

			for _, videoExt := range extensions {
				if strings.EqualFold(ext, videoExt) {
					// Check if filename contains the title or ID
					if strings.Contains(strings.ToLower(filename), strings.ToLower(title)) ||
						strings.Contains(strings.ToLower(filename), strings.ToLower(id)) {
						foundFile = path
						return fmt.Errorf("found") // Use error to break out of walk
					}
				}
			}
		}
		return nil
	})

	if foundFile != "" {
		return foundFile, nil
	}

	// If still not found, try to find any recent video file
	filepath.WalkDir(vs.DownloadsDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if !d.IsDir() {
			ext := filepath.Ext(path)
			filename := d.Name()

			// Skip partial files
			if strings.Contains(filename, ".part") || strings.Contains(filename, ".ytdl") {
				return nil
			}

			for _, videoExt := range extensions {
				if strings.EqualFold(ext, videoExt) {
					info, err := d.Info()
					if err == nil && time.Since(info.ModTime()) < 10*time.Minute {
						foundFile = path
						return fmt.Errorf("found")
					}
				}
			}
		}
		return nil
	})

	if foundFile != "" {
		return foundFile, nil
	}

	return "", fmt.Errorf("downloaded file not found")
}

// findThumbnailFile finds the thumbnail file for a video
func (vs *VideoService) findThumbnailFile(title, id string) string {
	extensions := []string{".jpg", ".jpeg", ".png", ".webp"}

	// Try with exact title first
	for _, ext := range extensions {
		path := filepath.Join(vs.DownloadsDir, title+ext)
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}

	// Try with sanitized title
	sanitizedTitle := models.SanitizeFilename(title)
	for _, ext := range extensions {
		path := filepath.Join(vs.DownloadsDir, sanitizedTitle+ext)
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}

	// Try with ID
	for _, ext := range extensions {
		path := filepath.Join(vs.DownloadsDir, id+ext)
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}

	// Search for thumbnail files that contain the title
	var foundFile string
	filepath.WalkDir(vs.DownloadsDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if !d.IsDir() {
			ext := filepath.Ext(path)
			filename := d.Name()

			for _, thumbExt := range extensions {
				if strings.EqualFold(ext, thumbExt) {
					if strings.Contains(strings.ToLower(filename), strings.ToLower(title)) ||
						strings.Contains(strings.ToLower(filename), strings.ToLower(id)) {
						foundFile = path
						return fmt.Errorf("found")
					}
				}
			}
		}
		return nil
	})

	return foundFile
}

// GetAllVideos returns all videos
func (vs *VideoService) GetAllVideos() []*models.Video {
	videos := make([]*models.Video, 0, len(vs.videos))
	for _, video := range vs.videos {
		videos = append(videos, video)
	}
	return videos
}

// SearchVideos searches videos by title, uploader, or description
func (vs *VideoService) SearchVideos(query string) []*models.Video {
	query = strings.ToLower(query)
	var results []*models.Video

	for _, video := range vs.videos {
		if strings.Contains(strings.ToLower(video.Title), query) ||
			strings.Contains(strings.ToLower(video.Uploader), query) ||
			strings.Contains(strings.ToLower(video.Description), query) {
			results = append(results, video)
		}
	}

	return results
}

// GetVideo returns a video by ID
func (vs *VideoService) GetVideo(id string) (*models.Video, bool) {
	video, exists := vs.videos[id]
	return video, exists
}

// DeleteVideo removes a video and its files
func (vs *VideoService) DeleteVideo(id string) error {
	video, exists := vs.videos[id]
	if !exists {
		return fmt.Errorf("video not found")
	}

	// Remove video file
	if err := os.Remove(video.FilePath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove video file: %w", err)
	}

	// Remove thumbnail if it exists
	if video.Thumbnail != "" {
		os.Remove(video.Thumbnail) // Ignore errors for thumbnail
	}

	// Remove from memory
	delete(vs.videos, id)

	// Save updated metadata
	return vs.SaveMetadata()
}
