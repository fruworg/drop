# The Nil Pointer

A temporary file hosting service built with Echo, inspired by [0x0.st](https://0x0.st/). Perfect for personal, self-hosted temporary file sharing.

## Features

- Upload files up to 100MB (configurable)
- Dynamic file expiration based on size
- File management (delete, update expiration)
- Metadata persistence with BadgerDB

## Dependencies

- [Echo](https://echo.labstack.com/) - High performance, minimalist Go web framework
- [templ](https://github.com/a-h/templ) - Typed templating language for Go
- [BadgerDB](https://github.com/dgraph-io/badger) - Fast key-value database for metadata storage

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
   # Generate templates and run
   task dev

   # Or build and then run
   task build
   task serve
   ```

4. Docker deployment:
   ```
   docker build -t nilpointer .
   docker run -p 8080:8080 -v ./uploads:/uploads -v ./config:/config nilpointer
   ```

## Configuration

Edit `./config/config.json`:

```json
{
  "min_age_days": 30,
  "max_age_days": 365,
  "max_size_mib": 512,
  "upload_path": "./uploads",
  "check_interval_min": 60,
  "enabled": true,
  "base_url": "http://localhost:8080/",
  "badger_path": "./badger",
  "max_upload_size": 104857600,
  "id_length": 8
}
```

## API Usage

### Upload

```bash
# Upload a file
curl -F'file=@yourfile.png' http://localhost:8080/

# From URL
curl -F'url=http://example.com/image.jpg' http://localhost:8080/

# Secret URL
curl -F'file=@yourfile.png' -F'secret=' http://localhost:8080/

# Custom expiration (24 hours)
curl -F'file=@yourfile.png' -F'expires=24' http://localhost:8080/

# JSON response
curl -H "Accept: application/json" -F'file=@yourfile.png' http://localhost:8080/
```

### Manage Files

```bash
# Delete a file
curl -X POST -F'token=your_token_here' -F'delete=' http://localhost:8080/filename.ext

# Update expiration
curl -X POST -F'token=your_token_here' -F'expires=48' http://localhost:8080/filename.ext
```

## License

MIT License - See [LICENSE](LICENSE) file for details
