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

func handleVideoDownload(link string) error {
	cmd := exec.Command("yt-dlp", link, "--output", "videos/%(upload_date)s-%(uploader)s-%(title)s-[%(id)s].%(ext)s")
	return cmd.Run()
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
		for _, entry := range entries {
			if !entry.IsDir() {
				info, err := entry.Info()
				if err != nil {
					continue
				}
				videos = append(videos, map[string]interface{}{
					"name":     entry.Name(),
					"size":     info.Size(),
					"modified": info.ModTime().Format("2006-01-02 15:04:05"),
				})
			}
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
