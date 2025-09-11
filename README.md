# Drop

A temporary file hosting service built with Echo, inspired by [0x0.st](https://0x0.st/). Perfect for personal, self-hosted temporary file sharing.

**Disclaimer**: While this project is inspired by 0x0.st, it is by no means intended to provide full API compatibility with their implementation. This is a personal project with its own features and limitations.

## Features

- Upload files up to configurable size limit (default 1024MB)
- Dynamic file expiration based on size
- One-time download links
- Secret (hard-to-guess) URLs
- File management (delete, update expiration)
- Metadata persistence using SQLite
- Preview protection for one-time links
- Docker deployment support
- Chunked uploads for large files with resume capability
- **Admin panel** for simple file management 

## Quick Start

1. **Clone and setup:**
   ```bash
   git clone https://github.com/marianozunino/drop.git
   cd drop
   go mod tidy
   ```

2. **Run with task:**
   ```bash
   # Generate templates and run development server
   task dev
   
   # Or build and run
   task build
   task serve
   ```

3. **Docker deployment:**
   ```bash
   # Option 1: Use pre-built image (recommended)
   docker pull ghcr.io/marianozunino/drop:latest
   docker run -p 3000:3000 -v ./uploads:/uploads -v ./config:/config -v ./data:/data ghcr.io/marianozunino/drop:latest
   
   # Option 2: Build from source
   docker build -t drop .
   docker run -p 3000:3000 -v ./uploads:/uploads -v ./config:/config -v ./data:/data drop
   ```

## Configuration

Edit `./config/config.yaml`:

```yaml
port: 3000
min_age_days: 30
max_age_days: 365
max_size_mib: 1024
upload_path: ./uploads
check_interval_min: 60
expiration_manager_enabled: true
base_url: "http://localhost:3000/"
sqlite_path: "./data/dump.db"
id_length: 4
chunk_size_mib: 4
preview_bots:
  - slack
  - slackbot
  - facebookexternalhit
  - twitterbot
  - discordbot
  - whatsapp
  - googlebot
  - linkedinbot
  - telegram
  - skype
  - viber
streaming_buffer_size_kb: 64
```

### Configuration Options

- `port` - HTTP server port
- `min_age_days` - Minimum file retention period in days
- `max_age_days` - Maximum file retention period in days
- `max_size_mib` - Maximum file size in MiB
- `upload_path` - Directory for uploaded files
- `check_interval_min` - Interval to check for expired files in minutes
- `expiration_manager_enabled` - Enable/disable automatic file expiration
- `base_url` - Base URL for generated file links
- `sqlite_path` - Path to SQLite database file
- `id_length` - Length of generated file IDs
- `chunk_size_mib` - Size of chunks for chunked uploads in MiB
- `preview_bots` - List of user-agent substrings to identify preview bots
- `streaming_buffer_size_kb` - Buffer size for streaming file content (in KB)

## API Usage

### Basic Upload

```bash
# Upload a file
curl -F'file=@yourfile.png' http://localhost:3000/

# Upload from URL
curl -F'url=http://example.com/image.jpg' http://localhost:3000/

# Generate a secret URL
curl -F'file=@yourfile.png' -F'secret=' http://localhost:3000/

# Create a one-time download link
curl -F'file=@yourfile.png' -F'one_time=' http://localhost:3000/

# Set custom expiration (24 hours)
curl -F'file=@yourfile.png' -F'expires=24' http://localhost:3000/

# JSON response
curl -H "Accept: application/json" -F'file=@yourfile.png' http://localhost:3000/
```

### File Management

```bash
# Delete a file
curl -X POST -F'token=your_token_here' -F'delete=' http://localhost:3000/filename.ext

# Update expiration
curl -X POST -F'token=your_token_here' -F'expires=48' http://localhost:3000/filename.ext
```

### Chunked Upload (Large Files)

For large files, use the chunked upload feature:

```bash
# Initialize chunked upload
curl -X POST http://localhost:3000/upload/init \
    -F "filename=large-video.mp4" \
    -F "size=104857600" \
    -F "chunk_size=4194304"

# Upload individual chunks
curl -X POST http://localhost:3000/upload/chunk/abc123/0 \
    -F "chunk=@chunk_0.bin"

# Check upload progress
curl http://localhost:3000/upload/status/abc123
```

## Documentation

- **[Complete API Documentation](API.md)** - Detailed API reference with examples
- **[Admin Panel Documentation](ADMIN.md)** - Comprehensive admin panel guide
- **[Web Interface](https://drop.mz.uy/)** - Interactive upload interface
- **[Chunked Upload Interface](https://drop.mz.uy/chunked)** - Drag & drop for large files

## License

MIT License - See [LICENSE](LICENSE) file for details
