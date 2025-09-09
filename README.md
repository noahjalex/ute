# UTE - YouTube Video Manager

A modern, secure YouTube video downloader and manager built with Go, featuring a responsive web interface with real-time download progress, video search, and thumbnail support.

## Features

- **Real-time Download Progress**: WebSocket-based streaming of download progress
- **Modern UI**: Built with Tailwind CSS and HTMX for responsive, dynamic interactions
- **Video Library**: Browse, search, and manage downloaded videos
- **Thumbnail Support**: Automatic thumbnail extraction and display
- **Secure File Handling**: Path traversal protection and secure file operations
- **Video Metadata**: Display duration, file size, uploader, and upload date
- **Search & Filter**: Real-time search and sorting by title, date, size, or duration
- **Download to Client**: Download videos from server to your local device

## Prerequisites

- Go 1.23.4 or later
- [yt-dlp](https://github.com/yt-dlp/yt-dlp) installed and available in PATH

### Installing yt-dlp

```bash
# Using pip
pip install yt-dlp

# Using homebrew (macOS)
brew install yt-dlp

# Using package manager (Ubuntu/Debian)
sudo apt install yt-dlp
```

## Installation

1. Clone the repository:
```bash
git clone <repository-url>
cd ute
```

2. Install Go dependencies:
```bash
go mod tidy
```

3. Build the application:
```bash
go build -o ute ./cmd/web/
```

## Usage

1. Start the server:
```bash
./ute
```

2. Open your browser and navigate to `http://localhost:8080`

### Command Line Options

- `-addr`: Server address (default: `:8080`)
- `-host`: Host name (default: `localhost`)
- `-protocol`: Protocol to use - `http` or `https` (default: `http`)
- `-downloads`: Directory to store downloaded videos (default: `./downloads`)

Example:
```bash
./ute -addr :3000 -downloads /path/to/videos
```

## Project Structure

```
ute/
├── cmd/web/                 # Main application entry point
├── internal/
│   ├── handlers/           # HTTP handlers
│   ├── models/            # Data models
│   └── services/          # Business logic
├── templates/             # HTML templates
├── static/               # Static assets (CSS, JS)
├── downloads/            # Default download directory
└── README.md
```

## Security Features

- **Path Traversal Protection**: All file operations are validated to prevent directory traversal attacks
- **Secure File Serving**: Files are served through controlled endpoints with validation
- **Input Sanitization**: User inputs are sanitized and validated
- **Filename Sanitization**: Downloaded filenames are cleaned of dangerous characters

## API Endpoints

- `GET /` - Home page with download form
- `POST /download` - Submit video URL for download
- `GET /ws/download` - WebSocket endpoint for download progress
- `GET /videos` - Video library page
- `GET /videos/search` - HTMX search endpoint
- `GET /download-video?id=<id>` - Download video file to client
- `DELETE /video?id=<id>` - Delete video
- `GET /thumbnail?id=<id>` - Serve video thumbnail

## Technology Stack

- **Backend**: Go with Gorilla Mux router and WebSocket support
- **Frontend**: HTML templates with Tailwind CSS and HTMX
- **Video Processing**: yt-dlp for downloading and metadata extraction
- **Real-time Updates**: WebSockets for download progress streaming

## Development

To run in development mode with auto-reload, you can use tools like `air`:

```bash
# Install air
go install github.com/cosmtrek/air@latest

# Run with auto-reload
air
```

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests if applicable
5. Submit a pull request

## License

This project is open source. Please check the license file for details.

## Troubleshooting

### yt-dlp not found
Make sure yt-dlp is installed and available in your PATH:
```bash
which yt-dlp
yt-dlp --version
```

### Permission denied on downloads directory
Ensure the application has write permissions to the downloads directory:
```bash
chmod 755 ./downloads
```

### WebSocket connection issues
If you're running behind a proxy or on a different host, update the WebSocket URL in the download handler to match your setup.
