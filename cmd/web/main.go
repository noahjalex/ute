package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

type VideoInfo struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Uploader    string `json:"uploader"`
	UploadDate  string `json:"upload_date"`
	Description string `json:"description"`
	ViewCount   int    `json:"view_count"`
	WebpageURL  string `json:"webpage_url"`
}

// DownloadError represents a structured error response
type DownloadError struct {
	Type    string `json:"type"`
	Message string `json:"message"`
	Details string `json:"details,omitempty"`
	Code    int    `json:"code"`
}

// Error types
const (
	ErrorTypeValidation = "validation_error"
	ErrorTypeNetwork    = "network_error"
	ErrorTypeNotFound   = "not_found_error"
	ErrorTypeBinary     = "binary_error"
	ErrorTypePermission = "permission_error"
	ErrorTypeFileSystem = "filesystem_error"
	ErrorTypeUnknown    = "unknown_error"
)

// Response structures
type SuccessResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

type ErrorResponse struct {
	Success bool           `json:"success"`
	Error   *DownloadError `json:"error"`
}

// validateURL performs basic URL validation
func validateURL(urlStr string) *DownloadError {
	if strings.TrimSpace(urlStr) == "" {
		return &DownloadError{
			Type:    ErrorTypeValidation,
			Message: "URL cannot be empty",
			Code:    http.StatusBadRequest,
		}
	}

	// Parse URL
	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return &DownloadError{
			Type:    ErrorTypeValidation,
			Message: "Invalid URL format",
			Details: err.Error(),
			Code:    http.StatusBadRequest,
		}
	}

	// Check if it has a valid scheme
	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return &DownloadError{
			Type:    ErrorTypeValidation,
			Message: "URL must use http or https protocol",
			Code:    http.StatusBadRequest,
		}
	}

	// Check if it has a host
	if parsedURL.Host == "" {
		return &DownloadError{
			Type:    ErrorTypeValidation,
			Message: "URL must have a valid host",
			Code:    http.StatusBadRequest,
		}
	}

	// Basic pattern matching for supported sites (can be extended)
	supportedPatterns := []string{
		`youtube\.com`,
		`youtu\.be`,
		`vimeo\.com`,
		`dailymotion\.com`,
		`twitch\.tv`,
		`tiktok\.com`,
		`instagram\.com`,
		`twitter\.com`,
		`x\.com`,
	}

	host := strings.ToLower(parsedURL.Host)
	for _, pattern := range supportedPatterns {
		matched, _ := regexp.MatchString(pattern, host)
		if matched {
			return nil // Valid URL
		}
	}

	log.Printf("Warning: URL %s may not be supported by yt-dlp, but attempting download", urlStr)
	return nil // Allow unsupported sites to be attempted
}

// ensureVideosDirectory creates the videos directory if it doesn't exist
func ensureVideosDirectory() *DownloadError {
	videosDir := "./videos"

	// Check if directory exists
	if _, err := os.Stat(videosDir); os.IsNotExist(err) {
		log.Printf("Creating videos directory: %s", videosDir)
		if err := os.MkdirAll(videosDir, 0755); err != nil {
			return &DownloadError{
				Type:    ErrorTypeFileSystem,
				Message: "Failed to create videos directory",
				Details: err.Error(),
				Code:    http.StatusInternalServerError,
			}
		}
	} else if err != nil {
		return &DownloadError{
			Type:    ErrorTypeFileSystem,
			Message: "Failed to check videos directory",
			Details: err.Error(),
			Code:    http.StatusInternalServerError,
		}
	}

	// Test write permissions
	testFile := filepath.Join(videosDir, ".write_test")
	if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
		return &DownloadError{
			Type:    ErrorTypePermission,
			Message: "No write permission to videos directory",
			Details: err.Error(),
			Code:    http.StatusInternalServerError,
		}
	}
	os.Remove(testFile) // Clean up test file

	return nil
}

// checkYtDlpBinary verifies that yt-dlp is available
func checkYtDlpBinary() *DownloadError {
	cmd := exec.Command("yt-dlp", "--version")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return &DownloadError{
			Type:    ErrorTypeBinary,
			Message: "yt-dlp binary not found or not executable",
			Details: fmt.Sprintf("Error: %v, Stderr: %s", err, stderr.String()),
			Code:    http.StatusInternalServerError,
		}
	}

	log.Printf("yt-dlp version: %s", strings.TrimSpace(stdout.String()))
	return nil
}

// parseYtDlpError analyzes stderr output to categorize the error
func parseYtDlpError(stderr string) *DownloadError {
	stderrLower := strings.ToLower(stderr)

	// Network-related errors
	if strings.Contains(stderrLower, "network") ||
		strings.Contains(stderrLower, "connection") ||
		strings.Contains(stderrLower, "timeout") ||
		strings.Contains(stderrLower, "dns") {
		return &DownloadError{
			Type:    ErrorTypeNetwork,
			Message: "Network error occurred during download",
			Details: stderr,
			Code:    http.StatusBadGateway,
		}
	}

	// Video not found or unavailable
	if strings.Contains(stderrLower, "video unavailable") ||
		strings.Contains(stderrLower, "not available") ||
		strings.Contains(stderrLower, "private video") ||
		strings.Contains(stderrLower, "removed") ||
		strings.Contains(stderrLower, "deleted") ||
		strings.Contains(stderrLower, "404") {
		return &DownloadError{
			Type:    ErrorTypeNotFound,
			Message: "Video not found or unavailable",
			Details: stderr,
			Code:    http.StatusNotFound,
		}
	}

	// Permission/access errors
	if strings.Contains(stderrLower, "permission") ||
		strings.Contains(stderrLower, "access denied") ||
		strings.Contains(stderrLower, "forbidden") ||
		strings.Contains(stderrLower, "401") ||
		strings.Contains(stderrLower, "403") {
		return &DownloadError{
			Type:    ErrorTypePermission,
			Message: "Access denied or permission error",
			Details: stderr,
			Code:    http.StatusForbidden,
		}
	}

	// Unsupported URL
	if strings.Contains(stderrLower, "unsupported url") ||
		strings.Contains(stderrLower, "no video formats") ||
		strings.Contains(stderrLower, "extractor") {
		return &DownloadError{
			Type:    ErrorTypeValidation,
			Message: "Unsupported URL or no extractors available",
			Details: stderr,
			Code:    http.StatusBadRequest,
		}
	}

	// Default to unknown error
	return &DownloadError{
		Type:    ErrorTypeUnknown,
		Message: "Unknown error occurred during download",
		Details: stderr,
		Code:    http.StatusInternalServerError,
	}
}

// handleVideoDownload performs the video download with enhanced error handling
func handleVideoDownload(link string) *DownloadError {
	log.Printf("Starting download for URL: %s", link)

	// Validate URL
	if err := validateURL(link); err != nil {
		log.Printf("URL validation failed: %s", err.Message)
		return err
	}

	// Ensure videos directory exists
	if err := ensureVideosDirectory(); err != nil {
		log.Printf("Directory setup failed: %s", err.Message)
		return err
	}

	// Check yt-dlp binary
	if err := checkYtDlpBinary(); err != nil {
		log.Printf("Binary check failed: %s", err.Message)
		return err
	}

	// Prepare command with enhanced options
	cmd := exec.Command("yt-dlp",
		link,
		"--output", "videos/%(id)s.%(ext)s",
		"--write-info-json", // Saves full metadata
		"--embed-metadata",  // Basic info in media file
		"--embed-thumbnail", // Optional: cover art
		"--no-mtime",        // Don't modify timestamps
		"--no-warnings",     // Reduce noise in stderr
		"--newline",         // Progress on new lines
	)

	// Capture both stdout and stderr
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// Set timeout for the command (30 minutes)
	timeout := 30 * time.Minute
	done := make(chan error, 1)

	go func() {
		done <- cmd.Run()
	}()

	select {
	case err := <-done:
		if err != nil {
			log.Printf("yt-dlp command failed: %v", err)
			log.Printf("Stderr: %s", stderr.String())
			log.Printf("Stdout: %s", stdout.String())

			// Parse the error to provide better context
			return parseYtDlpError(stderr.String())
		}

		log.Printf("Download completed successfully for: %s", link)
		log.Printf("Output: %s", stdout.String())
		return nil

	case <-time.After(timeout):
		// Kill the process if it's still running
		if cmd.Process != nil {
			cmd.Process.Kill()
		}

		return &DownloadError{
			Type:    ErrorTypeNetwork,
			Message: "Download timeout exceeded",
			Details: fmt.Sprintf("Download took longer than %v", timeout),
			Code:    http.StatusRequestTimeout,
		}
	}
}

func loadVideoInfo(videoPath string) (*VideoInfo, error) {
	jsonPath := strings.TrimSuffix(videoPath, filepath.Ext(videoPath)) + ".info.json"
	data, err := os.ReadFile(jsonPath)
	if err != nil {
		return nil, err
	}

	var info VideoInfo
	if err := json.Unmarshal(data, &info); err != nil {
		return nil, err
	}

	return &info, nil
}

func main() {
	// Support environment variable for port
	defaultPort := os.Getenv("PORT")
	if defaultPort == "" {
		defaultPort = "8591"
	}
	if !strings.HasPrefix(defaultPort, ":") {
		defaultPort = ":" + defaultPort
	}

	addr := flag.String("addr", defaultPort, "port to host on (default from PORT env or ':8591')")
	flag.Parse()

	mux := http.NewServeMux()

	fs := http.FileServer(http.Dir("./static"))
	mux.Handle("/static/", http.StripPrefix("/static/", fs))

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "" || r.Method == "GET" {
			http.ServeFile(w, r, "./static/index.html")
			return
		}

		if r.Method == "POST" {
			// Set content type for JSON responses
			w.Header().Set("Content-Type", "application/json")

			// Parse request body
			d := json.NewDecoder(r.Body)
			linkBod := struct {
				Link string `json:"link"`
			}{}

			if err := d.Decode(&linkBod); err != nil {
				log.Printf("Failed to decode request body: %v", err)
				w.WriteHeader(http.StatusBadRequest)
				json.NewEncoder(w).Encode(ErrorResponse{
					Success: false,
					Error: &DownloadError{
						Type:    ErrorTypeValidation,
						Message: "Invalid JSON in request body",
						Details: err.Error(),
						Code:    http.StatusBadRequest,
					},
				})
				return
			}

			// Validate that link is provided
			if strings.TrimSpace(linkBod.Link) == "" {
				log.Printf("Empty link provided in request")
				w.WriteHeader(http.StatusBadRequest)
				json.NewEncoder(w).Encode(ErrorResponse{
					Success: false,
					Error: &DownloadError{
						Type:    ErrorTypeValidation,
						Message: "Link field is required and cannot be empty",
						Code:    http.StatusBadRequest,
					},
				})
				return
			}

			link := strings.TrimSpace(linkBod.Link)
			log.Printf("Processing download request for URL: %s", link)

			// Attempt video download
			if downloadErr := handleVideoDownload(link); downloadErr != nil {
				log.Printf("Download failed for URL %s: %s", link, downloadErr.Message)
				w.WriteHeader(downloadErr.Code)
				json.NewEncoder(w).Encode(ErrorResponse{
					Success: false,
					Error:   downloadErr,
				})
				return
			}

			// Success response
			log.Printf("Download completed successfully for URL: %s", link)
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(SuccessResponse{
				Success: true,
				Message: "Video download completed successfully",
			})
			return
		}

		// Method not allowed
		log.Printf("Unsupported HTTP method: %s", r.Method)
		w.WriteHeader(http.StatusMethodNotAllowed)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(ErrorResponse{
			Success: false,
			Error: &DownloadError{
				Type:    ErrorTypeValidation,
				Message: "Method not supported",
				Details: fmt.Sprintf("Method %s is not allowed for this endpoint", r.Method),
				Code:    http.StatusMethodNotAllowed,
			},
		})
	})

	// API endpoint to list videos
	mux.HandleFunc("/api/videos", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if r.Method != "GET" {
			log.Printf("Invalid method %s for /api/videos endpoint", r.Method)
			w.WriteHeader(http.StatusMethodNotAllowed)
			json.NewEncoder(w).Encode(ErrorResponse{
				Success: false,
				Error: &DownloadError{
					Type:    ErrorTypeValidation,
					Message: "Method not supported",
					Details: fmt.Sprintf("Method %s is not allowed for this endpoint", r.Method),
					Code:    http.StatusMethodNotAllowed,
				},
			})
			return
		}

		baseDir := "./videos"
		log.Printf("Listing videos from directory: %s", baseDir)

		// Check if shared directory exists
		if _, err := os.Stat(baseDir); os.IsNotExist(err) {
			log.Printf("Videos directory does not exist, returning empty list")
			// Return empty list if directory doesn't exist
			json.NewEncoder(w).Encode([]map[string]interface{}{})
			return
		}

		entries, err := os.ReadDir(baseDir)
		if err != nil {
			log.Printf("Failed to read videos directory: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(ErrorResponse{
				Success: false,
				Error: &DownloadError{
					Type:    ErrorTypeFileSystem,
					Message: "Failed to read videos directory",
					Details: err.Error(),
					Code:    http.StatusInternalServerError,
				},
			})
			return
		}

		var videos []map[string]interface{}
		videoExtensions := map[string]bool{
			".mp4":  true,
			".mkv":  true,
			".webm": true,
			".mov":  true,
			".flv":  true,
			".avi":  true,
		}

		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}

			ext := strings.ToLower(filepath.Ext(entry.Name()))
			if !videoExtensions[ext] {
				continue
			}

			videoPath := filepath.Join(baseDir, entry.Name())

			info, err := entry.Info()
			if err != nil {
				log.Printf("Failed to get file info for %s: %v", entry.Name(), err)
				continue
			}

			metadata, err := loadVideoInfo(videoPath)
			if err != nil {
				log.Printf("Failed to load metadata for %s: %v", entry.Name(), err)
				// Fallback if .info.json is missing
				metadata = &VideoInfo{
					Title: entry.Name(),
				}
			}

			videos = append(videos, map[string]interface{}{
				"filename":    entry.Name(),
				"size":        info.Size(),
				"modified":    info.ModTime().Format("2006-01-02 15:04:05"),
				"title":       metadata.Title,
				"uploader":    metadata.Uploader,
				"uploadDate":  metadata.UploadDate,
				"views":       metadata.ViewCount,
				"url":         metadata.WebpageURL,
				"description": metadata.Description,
			})
		}

		log.Printf("Found %d video files", len(videos))
		json.NewEncoder(w).Encode(videos)
	})

	mux.HandleFunc("/videos/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			log.Printf("Invalid method %s for /videos/ endpoint", r.Method)
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// Base directory to serve from
		baseDir := "./videos"

		// Clean the path and join with baseDir
		relPath := strings.TrimPrefix(r.URL.Path, "/videos/")

		// Security check: prevent directory traversal
		if strings.Contains(relPath, "..") || strings.Contains(relPath, "/") {
			log.Printf("Potential directory traversal attempt: %s", relPath)
			http.Error(w, "Invalid file path", http.StatusBadRequest)
			return
		}

		targetPath := filepath.Join(baseDir, relPath)
		log.Printf("Serving file: %s", targetPath)

		fi, err := os.Stat(targetPath)
		if err != nil {
			if os.IsNotExist(err) {
				log.Printf("File not found: %s", targetPath)
				http.NotFound(w, r)
			} else {
				log.Printf("Error accessing file %s: %v", targetPath, err)
				http.Error(w, "Internal server error", http.StatusInternalServerError)
			}
			return
		}

		// If it's a directory, return error
		if fi.IsDir() {
			log.Printf("Attempted to access directory as file: %s", targetPath)
			http.Error(w, "Cannot serve directories", http.StatusBadRequest)
			return
		}

		// Serve file for download
		w.Header().Set("Content-Disposition", "attachment; filename="+fi.Name())
		w.Header().Set("Content-Length", fmt.Sprintf("%d", fi.Size()))

		log.Printf("Serving file %s (%d bytes)", fi.Name(), fi.Size())
		http.ServeFile(w, r, targetPath)
	})

	fmt.Printf("Listening on http://0.0.0.0%s\n", *addr)
	if err := http.ListenAndServe(*addr, mux); err != nil {
		log.Fatalf("server error: %w", err)
	}
}
