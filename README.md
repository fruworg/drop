# Drop

A temporary file hosting service built with Echo, inspired by [0x0.st](https://0x0.st/). Perfect for personal, self-hosted temporary file sharing.

**Disclaimer**: While this project is inspired by 0x0.st, it is by no means intended to provide full API compatibility with their implementation. This is a personal project with its own features and limitations.

## Features

- Upload files up to configurable size limit (default 512MB)
- Dynamic file expiration based on size
- One-time download links
- Secret (hard-to-guess) URLs
- File management (delete, update expiration)
- Metadata persistence using SQLite
- Preview protection for one-time links
- Docker deployment support

## Dependencies

- [Echo](https://echo.labstack.com/) - High performance, minimalist Go web framework
- [templ](https://github.com/a-h/templ) - Typed templating language for Go
- [SQLite](https://www.sqlite.org/) - Embedded database for metadata storage

## Setup

1. Clone the repo:

   ```
   git clone https://github.com/marianozunino/drop.git
   cd drop
   ```

2. Get dependencies:

   ```
   go mod tidy
   ```

3. Run with task:

   ```
   # Generate templates and run development server
   task dev

   # Or build and then run
   task build
   task serve
   ```

4. Docker deployment:
   ```
   docker build -t drop .
   docker run -p 8080:8080 -v ./uploads:/uploads -v ./config:/config -v ./data:/data drop
   ```

## Configuration

Edit `./config/config.json`:

```json
{
  "port": 8080,
  "min_age_days": 30,
  "max_age_days": 365,
  "max_size_mib": 512,
  "upload_path": "./uploads",
  "check_interval_min": 60,
  "expiration_manager_enabled": true,
  "base_url": "http://localhost:8080/",
  "sqlite_path": "/data/dump.db",
  "id_length": 8,
  "preview_bots": [
    "slack",
    "slackbot",
    "facebookexternalhit",
    "twitterbot",
    "discordbot",
    "whatsapp",
    "googlebot",
    "linkedinbot",
    "telegram",
    "skype",
    "viber"
  ]
}
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
- `preview_bots` - List of user-agent substrings to identify preview bots

## API Usage

### Upload

```bash
# Upload a file
curl -F'file=@yourfile.png' http://localhost:8080/

# Upload from URL
curl -F'url=http://example.com/image.jpg' http://localhost:8080/

# Generate a secret URL
curl -F'file=@yourfile.png' -F'secret=' http://localhost:8080/

# Create a one-time download link
curl -F'file=@yourfile.png' -F'one_time=' http://localhost:8080/

# Set custom expiration (24 hours)
curl -F'file=@yourfile.png' -F'expires=24' http://localhost:8080/

# JSON response
curl -H "Accept: application/json" -F'file=@yourfile.png' http://localhost:8080/

# Combining options
curl -F'file=@yourfile.png' -F'one_time=' -F'secret=' -F'expires=24' http://localhost:8080/
```

### Manage Files

```bash
# Delete a file
curl -X POST -F'token=your_token_here' -F'delete=' http://localhost:8080/filename.ext

# Update expiration
curl -X POST -F'token=your_token_here' -F'expires=48' http://localhost:8080/filename.ext
```

### Expiration Formats

The `expires` parameter accepts multiple formats:

- Hours as integer (e.g., `24`)
- Unix timestamp in milliseconds (e.g., `1681996320000`)
- RFC3339 (e.g., `2023-04-20T10:15:30Z`)
- ISO date (e.g., `2023-04-20`)
- ISO datetime (e.g., `2023-04-20T10:15:30`)
- SQL datetime (e.g., `2023-04-20 15:04:05`)

## License

MIT License - See [LICENSE](LICENSE) file for details
