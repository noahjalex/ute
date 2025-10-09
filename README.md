# UTE - Video Download Service

A simple, video download service built with Go that uses yt-dlp to download videos from various platforms.

## Features

- Download videos from YouTube, Vimeo, TikTok, and many other platforms
- Responsive web interface with real-time feedback
- Video metadata extraction and display

## Quick Start

### Using Docker (Recommended)

1. **Clone the repository:**
   ```bash
   git clone <your-repo-url>
   cd ute
   ```

2. **Run with Docker Compose:**
   ```bash
   docker-compose up -d
   ```

3. **Access the service:**
   Open http://localhost:8591 in your browser

### Using Docker Build

```bash
# Build the image
docker build -t ute-video-downloader .

# Run the container
docker run -d \
  --name ute \
  -p 8591:8591 \
  -v $(pwd)/videos:/app/videos \
  ute-video-downloader
```

### Manual Installation

1. **Prerequisites:**
   - Go 1.23+
   - Python 3.x
   - yt-dlp (`pip install yt-dlp`)
   - ffmpeg (optional, for better format support)

2. **Build and run:**
   ```bash
   go build -o main ./cmd/web
   ./main
   ```

## Configuration

### Environment Variables

- `PORT`: Server port (default: 8591)

### Docker Environment

```bash
# Custom port
PORT=3000 docker-compose up -d

# Or set in .env file
echo "PORT=3000" > .env
docker-compose up -d
```

## Usage

1. **Open the web interface** at http://localhost:8591
2. **Paste a video URL** in the input field
3. **Click Download** and wait for the process to complete
4. **View downloaded videos** in the list below
5. **Click Download** next to any video to save it locally

### Supported Platforms

- YouTube (youtube.com, youtu.be)
- Vimeo (vimeo.com)
- TikTok (tiktok.com)
- Instagram (instagram.com)
- Twitter/X (twitter.com, x.com)
- Twitch (twitch.tv)
- Dailymotion (dailymotion.com)
- And many more supported by yt-dlp

## API Endpoints

- `GET /` - Web interface
- `POST /` - Submit video URL for download
- `GET /api/videos` - List downloaded videos
- `GET /videos/{filename}` - Download video file

## Error Handling

The service provides detailed error messages for common issues:

- **Invalid URLs**: Format validation and helpful suggestions
- **Network Issues**: Timeout handling and retry mechanisms
- **Video Unavailable**: Clear messages for private/deleted content
- **Permission Errors**: Access and authentication issues
- **System Issues**: Missing dependencies, disk space, etc.

## Development

### Local Development

```bash
# Install air for hot reloading (optional)
go install github.com/cosmtrek/air@latest

# Run with hot reloading
air

# Or run normally
go run cmd/web/main.go
```

### Project Structure

```
ute/
├── cmd/web/main.go     # Main application
├── static/             # Web assets
│   ├── index.html
│   ├── script.js
│   └── styles.css
├── videos/             # Downloaded videos (mounted in Docker)
├── Dockerfile          # Docker build configuration
├── docker-compose.yml  # Docker Compose configuration
└── README.md
```

## Security Notes

This service is designed for **internal use only**. While it includes basic security measures:

- Input validation and sanitization
- Directory traversal protection
- Non-root user in Docker container
- Resource limits in Docker

**Do not expose this service directly to the internet** without additional security measures.

## Troubleshooting

### Common Issues

1. **"yt-dlp binary not found"**
   - Install yt-dlp: `pip install yt-dlp`
   - Ensure it's in your PATH

2. **Permission denied on videos directory**
   - Check directory permissions: `chmod 755 videos/`
   - In Docker: volumes are automatically configured

3. **Port already in use**
   - Change port: `PORT=3000 docker-compose up -d`
   - Or kill existing process: `lsof -ti:8591 | xargs kill`

4. **Video download fails**
   - Check the error message in the web interface
   - Some videos may require authentication or be geo-blocked
   - Try updating yt-dlp: `pip install --upgrade yt-dlp`

### Logs

```bash
# Docker logs
docker logs ute-video-downloader

# Follow logs
docker logs -f ute-video-downloader
```

## License

This project is for personal/internal use. Please respect the terms of service of video platforms and applicable copyright laws.
