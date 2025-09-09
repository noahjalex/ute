package handlers

import (
	"bytes"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"noahjalex.ute/internal/models"
	"noahjalex.ute/internal/services"
)

// VideoHandler handles video-related requests
type VideoHandler struct {
	VideoService *services.VideoService
	Templates    *template.Template
}

// NewVideoHandler creates a new video handler
func NewVideoHandler(videoService *services.VideoService) *VideoHandler {
	// Load templates
	templates := template.Must(template.ParseGlob("templates/*.html"))

	return &VideoHandler{
		VideoService: videoService,
		Templates:    templates,
	}
}

// HandleVideoList displays the video library
func (h *VideoHandler) HandleVideoList(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	sortBy := r.URL.Query().Get("sort")

	var videos []*models.Video
	if query != "" {
		videos = h.VideoService.SearchVideos(query)
	} else {
		videos = h.VideoService.GetAllVideos()
	}

	// Sort videos
	switch sortBy {
	case "title":
		sort.Slice(videos, func(i, j int) bool {
			return strings.ToLower(videos[i].Title) < strings.ToLower(videos[j].Title)
		})
	case "date":
		sort.Slice(videos, func(i, j int) bool {
			return videos[i].CreatedAt.After(videos[j].CreatedAt)
		})
	case "size":
		sort.Slice(videos, func(i, j int) bool {
			return videos[i].FileSize > videos[j].FileSize
		})
	case "duration":
		sort.Slice(videos, func(i, j int) bool {
			return videos[i].Duration > videos[j].Duration
		})
	default:
		// Default sort by creation date (newest first)
		sort.Slice(videos, func(i, j int) bool {
			return videos[i].CreatedAt.After(videos[j].CreatedAt)
		})
	}

	data := struct {
		Videos []models.Video
		Query  string
		Sort   string
	}{
		Videos: make([]models.Video, len(videos)),
		Query:  query,
		Sort:   sortBy,
	}

	// Convert pointers to values for template
	for i, video := range videos {
		data.Videos[i] = *video
	}

	// Execute template to buffer first to avoid header conflicts
	var buf bytes.Buffer
	if err := h.Templates.ExecuteTemplate(&buf, "video_list.html", data); err != nil {
		http.Error(w, "Template error", http.StatusInternalServerError)
		log.Fatal("err executing template: ", err)
		return
	}

	// Only write to response if template execution succeeded
	w.Header().Set("Content-Type", "text/html")
	w.Write(buf.Bytes())
}

// HandleVideoSearch handles HTMX search requests
func (h *VideoHandler) HandleVideoSearch(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	sortBy := r.URL.Query().Get("sort")

	var videos []*models.Video
	if query != "" {
		videos = h.VideoService.SearchVideos(query)
	} else {
		videos = h.VideoService.GetAllVideos()
	}

	// Sort videos (same logic as HandleVideoList)
	switch sortBy {
	case "title":
		sort.Slice(videos, func(i, j int) bool {
			return strings.ToLower(videos[i].Title) < strings.ToLower(videos[j].Title)
		})
	case "date":
		sort.Slice(videos, func(i, j int) bool {
			return videos[i].CreatedAt.After(videos[j].CreatedAt)
		})
	case "size":
		sort.Slice(videos, func(i, j int) bool {
			return videos[i].FileSize > videos[j].FileSize
		})
	case "duration":
		sort.Slice(videos, func(i, j int) bool {
			return videos[i].Duration > videos[j].Duration
		})
	default:
		sort.Slice(videos, func(i, j int) bool {
			return videos[i].CreatedAt.After(videos[j].CreatedAt)
		})
	}

	data := struct {
		Videos []models.Video
	}{
		Videos: make([]models.Video, len(videos)),
	}

	for i, video := range videos {
		data.Videos[i] = *video
	}

	// Execute template to buffer first to avoid header conflicts
	var buf bytes.Buffer
	if err := h.Templates.ExecuteTemplate(&buf, "video_grid.html", data); err != nil {
		http.Error(w, "Template error", http.StatusInternalServerError)
		return
	}

	// Only write to response if template execution succeeded
	w.Header().Set("Content-Type", "text/html")
	w.Write(buf.Bytes())
}

// HandleVideoDownload serves video files for download
func (h *VideoHandler) HandleVideoDownload(w http.ResponseWriter, r *http.Request) {
	videoID := r.URL.Query().Get("id")
	if videoID == "" {
		http.Error(w, "Video ID required", http.StatusBadRequest)
		return
	}

	video, exists := h.VideoService.GetVideo(videoID)
	if !exists {
		http.Error(w, "Video not found", http.StatusNotFound)
		return
	}

	// Validate the file path for security
	securePath, err := h.VideoService.SecurePath(video.GetRelativePath(h.VideoService.DownloadsDir))
	if err != nil {
		http.Error(w, "Invalid file path", http.StatusBadRequest)
		return
	}

	// Check if file exists
	if _, err := os.Stat(securePath); os.IsNotExist(err) {
		http.Error(w, "File not found", http.StatusNotFound)
		return
	}

	// Set headers for download
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", video.Filename))
	w.Header().Set("Content-Type", "application/octet-stream")

	// Serve the file
	http.ServeFile(w, r, securePath)
}

// HandleVideoDelete handles video deletion
func (h *VideoHandler) HandleVideoDelete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	videoID := r.URL.Query().Get("id")
	if videoID == "" {
		http.Error(w, "Video ID required", http.StatusBadRequest)
		return
	}

	if err := h.VideoService.DeleteVideo(videoID); err != nil {
		http.Error(w, fmt.Sprintf("Failed to delete video: %v", err), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

// HandleThumbnail serves thumbnail images
func (h *VideoHandler) HandleThumbnail(w http.ResponseWriter, r *http.Request) {
	videoID := r.URL.Query().Get("id")
	if videoID == "" {
		http.Error(w, "Video ID required", http.StatusBadRequest)
		return
	}

	video, exists := h.VideoService.GetVideo(videoID)
	if !exists {
		http.Error(w, "Video not found", http.StatusNotFound)
		return
	}

	if video.Thumbnail == "" {
		// Serve a default thumbnail or 404
		http.NotFound(w, r)
		return
	}

	// Validate the thumbnail path for security
	thumbnailPath := video.Thumbnail
	if !strings.HasPrefix(thumbnailPath, h.VideoService.DownloadsDir) {
		http.Error(w, "Invalid thumbnail path", http.StatusBadRequest)
		return
	}

	// Check if file exists
	if _, err := os.Stat(thumbnailPath); os.IsNotExist(err) {
		http.NotFound(w, r)
		return
	}

	// Determine content type based on file extension
	ext := strings.ToLower(filepath.Ext(thumbnailPath))
	switch ext {
	case ".jpg", ".jpeg":
		w.Header().Set("Content-Type", "image/jpeg")
	case ".png":
		w.Header().Set("Content-Type", "image/png")
	case ".webp":
		w.Header().Set("Content-Type", "image/webp")
	default:
		w.Header().Set("Content-Type", "image/jpeg")
	}

	// Serve the thumbnail
	http.ServeFile(w, r, thumbnailPath)
}
