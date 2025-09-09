package handlers

import (
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
	"noahjalex.ute/internal/services"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow connections from any origin for development
	},
}

// DownloadHandler handles video download requests
type DownloadHandler struct {
	VideoService *services.VideoService
}

// NewDownloadHandler creates a new download handler
func NewDownloadHandler(videoService *services.VideoService) *DownloadHandler {
	return &DownloadHandler{
		VideoService: videoService,
	}
}

// HandleDownload handles the download form submission
func (h *DownloadHandler) HandleDownload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Failed to parse form", http.StatusBadRequest)
		return
	}

	url := r.FormValue("url")
	if url == "" {
		http.Error(w, "URL is required", http.StatusBadRequest)
		return
	}

	// Return the download page with WebSocket connection
	w.Header().Set("Content-Type", "text/html")
	fmt.Fprintf(w, `
<!DOCTYPE html>
<html>
<head>
    <title>Downloading Video</title>
    <script src="https://cdn.tailwindcss.com"></script>
    <script src="https://unpkg.com/htmx.org@1.9.6"></script>
</head>
<body class="bg-gray-100 min-h-screen">
    <div class="container mx-auto px-4 py-8">
        <div class="max-w-2xl mx-auto bg-white rounded-lg shadow-md p-6">
            <h1 class="text-2xl font-bold mb-4">Downloading Video</h1>
            <div id="progress" class="bg-gray-200 rounded-lg p-4 h-64 overflow-y-auto font-mono text-sm">
                <div>Connecting...</div>
            </div>
            <div id="status" class="mt-4 text-center">
                <div class="inline-flex items-center">
                    <div class="animate-spin rounded-full h-4 w-4 border-b-2 border-blue-600 mr-2"></div>
                    <span>Downloading...</span>
                </div>
            </div>
            <div class="mt-4 text-center">
                <a href="/videos" class="text-blue-600 hover:text-blue-800">← Back to Videos</a>
            </div>
        </div>
    </div>
    
    <script>
        const ws = new WebSocket('ws://localhost:8080/ws/download?url=' + encodeURIComponent('%s'));
        const progress = document.getElementById('progress');
        const status = document.getElementById('status');
        
        ws.onmessage = function(event) {
            const data = JSON.parse(event.data);
            
            if (data.type === 'progress') {
                const div = document.createElement('div');
                div.textContent = data.message;
                progress.appendChild(div);
                progress.scrollTop = progress.scrollHeight;
            } else if (data.type === 'complete') {
                status.innerHTML = '<div class="text-green-600 font-semibold">✓ Download Complete!</div>';
                setTimeout(() => {
                    window.location.href = '/videos';
                }, 2000);
            } else if (data.type === 'error') {
                status.innerHTML = '<div class="text-red-600 font-semibold">✗ Download Failed: ' + data.message + '</div>';
            }
        };
        
        ws.onerror = function(error) {
            status.innerHTML = '<div class="text-red-600 font-semibold">✗ Connection Error</div>';
        };
        
        ws.onclose = function() {
            if (status.innerHTML.includes('Downloading')) {
                status.innerHTML = '<div class="text-red-600 font-semibold">✗ Connection Lost</div>';
            }
        };
    </script>
</body>
</html>`, url)
}

// HandleWebSocket handles WebSocket connections for download progress
func (h *DownloadHandler) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade error: %v", err)
		return
	}
	defer conn.Close()

	url := r.URL.Query().Get("url")
	if url == "" {
		conn.WriteJSON(map[string]string{
			"type":    "error",
			"message": "URL is required",
		})
		return
	}

	// Create progress channel
	progressChan := make(chan string, 100)

	// Start download in goroutine
	go func() {
		if err := h.VideoService.DownloadVideo(url, progressChan); err != nil {
			conn.WriteJSON(map[string]string{
				"type":    "error",
				"message": err.Error(),
			})
			return
		}

		conn.WriteJSON(map[string]string{
			"type": "complete",
		})
	}()

	// Send progress updates
	for {
		select {
		case message, ok := <-progressChan:
			if !ok {
				return // Channel closed
			}

			if err := conn.WriteJSON(map[string]string{
				"type":    "progress",
				"message": message,
			}); err != nil {
				log.Printf("WebSocket write error: %v", err)
				return
			}

		case <-time.After(30 * time.Second):
			// Ping to keep connection alive
			if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
