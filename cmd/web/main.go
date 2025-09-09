package main

import (
	"flag"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"path/filepath"

	"github.com/gorilla/mux"
	"noahjalex.ute/internal/handlers"
	"noahjalex.ute/internal/services"
)

func main() {
	protocol := flag.String("protocol", "http", "protocol to use (default: 'http')")
	addr := flag.String("addr", ":8080", "port to host on (default: ':8080')")
	host := flag.String("host", "localhost", "host name (default: 'localhost')")
	downloadsDir := flag.String("downloads", "./downloads", "directory to store downloaded videos")
	flag.Parse()

	// Initialize services
	videoService := services.NewVideoService(*downloadsDir)

	// Initialize handlers
	downloadHandler := handlers.NewDownloadHandler(videoService)
	videoHandler := handlers.NewVideoHandler(videoService)

	// Setup router
	r := mux.NewRouter()

	// Home page - download form
	r.HandleFunc("/", func(w http.ResponseWriter, req *http.Request) {
		if req.Method == http.MethodGet {
			tmpl := template.Must(template.ParseFiles("templates/base.html", "templates/index.html"))
			w.Header().Set("Content-Type", "text/html")
			if err := tmpl.ExecuteTemplate(w, "base.html", nil); err != nil {
				http.Error(w, "Template error", http.StatusInternalServerError)
				return
			}
		} else {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	}).Methods("GET")

	// Download endpoints
	r.HandleFunc("/download", downloadHandler.HandleDownload).Methods("POST")
	r.HandleFunc("/ws/download", downloadHandler.HandleWebSocket)

	// Video endpoints
	r.HandleFunc("/videos", videoHandler.HandleVideoList).Methods("GET")
	r.HandleFunc("/videos/search", videoHandler.HandleVideoSearch).Methods("GET")
	r.HandleFunc("/download-video", videoHandler.HandleVideoDownload).Methods("GET")
	r.HandleFunc("/video", videoHandler.HandleVideoDelete).Methods("DELETE")
	r.HandleFunc("/thumbnail", videoHandler.HandleThumbnail).Methods("GET")

	// Static files
	r.PathPrefix("/static/").Handler(http.StripPrefix("/static/", http.FileServer(http.Dir("./static/"))))

	// Legacy support for /videos/ path (redirect to new structure)
	r.PathPrefix("/videos/").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Extract the path after /videos/
		path := r.URL.Path[8:] // Remove "/videos/"

		if path == "" {
			// Redirect to new videos page
			http.Redirect(w, r, "/videos", http.StatusMovedPermanently)
			return
		}

		// For now, serve files directly (this maintains some backward compatibility)
		// but we should encourage users to use the new download system
		securePath, err := videoService.SecurePath(path)
		if err != nil {
			http.Error(w, "Invalid path", http.StatusBadRequest)
			return
		}

		// Check if it's a directory or file
		info, err := filepath.Abs(securePath)
		if err != nil {
			http.NotFound(w, r)
			return
		}

		// Serve the file
		http.ServeFile(w, r, info)
	})

	fmt.Printf("Starting UTE server...\n")
	fmt.Printf("Downloads directory: %s\n", *downloadsDir)
	fmt.Printf("Listening on %s://%s%s\n", *protocol, *host, *addr)
	fmt.Printf("Open your browser to %s://%s%s\n", *protocol, *host, *addr)

	switch *protocol {
	case "http":
		if err := http.ListenAndServe(*addr, r); err != nil {
			log.Fatalf("Server error: %v", err)
		}
	case "https":
		if err := http.ListenAndServeTLS(*addr, "", "", r); err != nil {
			log.Fatalf("Server error: %v", err)
		}
	}
}
