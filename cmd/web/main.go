package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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

func handleVideoDownload(link string) error {
	cmd := exec.Command("yt-dlp",
		link,
		"--output", "videos/%(id)s.%(ext)s",
		"--write-info-json", // Saves full metadata
		"--embed-metadata",  // Basic info in media file
		"--embed-thumbnail", // Optional: cover art
		"--no-mtime",        // Don't modify timestamps
	)
	return cmd.Run()
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
	addr := flag.String("addr", ":8080", "port to host on (default: ':8080')")

	mux := http.NewServeMux()

	fs := http.FileServer(http.Dir("./static"))
	mux.Handle("/static/", http.StripPrefix("/static/", fs))

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "" || r.Method == "GET" {
			http.ServeFile(w, r, "./static/index.html")
			return
		}

		if r.Method == "POST" {
			d := json.NewDecoder(r.Body)
			linkBod := struct {
				Link string `json:"link"`
			}{}
			err := d.Decode(&linkBod)
			if err != nil || linkBod.Link == "" {
				http.Error(w, "could not parse form", http.StatusBadRequest)
				return
			}
			link := linkBod.Link
			fmt.Printf("Got link: %s\n", link)

			err = handleVideoDownload(link)
			errS := struct {
				Error error `json:"error"`
			}{
				Error: err,
			}
			err = json.NewEncoder(w).Encode(&errS)
			if err != nil || linkBod.Link == "" {
				http.Error(w, "could not write error from command", http.StatusInternalServerError)
				return
			}
			return
		}

		http.Error(w, "method not supported", http.StatusBadRequest)
	})

	// API endpoint to list videos
	mux.HandleFunc("/api/videos", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			http.Error(w, "method not supported", http.StatusMethodNotAllowed)
			return
		}

		baseDir := "./videos"

		// Check if shared directory exists
		if _, err := os.Stat(baseDir); os.IsNotExist(err) {
			// Return empty list if directory doesn't exist
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode([]string{})
			return
		}

		entries, err := os.ReadDir(baseDir)
		if err != nil {
			http.Error(w, "could not read directory", http.StatusInternalServerError)
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
				continue
			}

			metadata, err := loadVideoInfo(videoPath)
			if err != nil {
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

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(videos)
	})

	mux.HandleFunc("/videos/", func(w http.ResponseWriter, r *http.Request) {
		// Base directory to serve from
		baseDir := "./videos"

		// Clean the path and join with baseDir
		relPath := strings.TrimPrefix(r.URL.Path, "/videos/")
		targetDir := filepath.Join(baseDir, relPath)

		fmt.Println(targetDir)
		fi, err := os.Stat(targetDir)
		if err != nil {
			http.NotFound(w, r)
			return
		}

		// If it’s a file -> serve directly for download
		if !fi.IsDir() {
			w.Header().Set("Content-Disposition", "attachment; filename="+fi.Name())
			http.ServeFile(w, r, targetDir)
			return
		}
		return

	})

	fmt.Printf("Listening on http://localhost%s\n", *addr)
	if err := http.ListenAndServe(*addr, mux); err != nil {
		log.Fatalf("server error: %w", err)
	}
}
