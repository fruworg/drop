# THE NULL POINTER

A temporary file hosting service built with Echo framework, inspired by [0x0.st](https://0x0.st/).
This service is primarily designed for personal use, providing a simple, self-hosted solution for temporary file sharing.

## Features

- Upload files up to 100MB
- Dynamic expiration based on file size
- Secret URLs for more privacy
- File management (delete, update expiration)
- REST API with JSON responses
- Clean, responsive web interface using the templ templating engine

## Dependencies

- [Echo](https://echo.labstack.com/) - High performance, minimalist Go web framework
- [templ](https://github.com/a-h/templ) - Typed templating language for Go

## Project Structure

- `main.go` - Main application file with Echo server setup
- `config/` - Configuration related code
  - `config.go` - Configuration management
  - `config.json` - Default configuration settings
- `expiration/` - File expiration management
  - `expiration.go` - Expiration calculation and management
  - `expiration_test.go` - Tests for expiration functions
- `templates/` - HTML templates using the templ templating engine
  - `home.templ` - Homepage template source
  - `home_templ.go` - Generated template code
- `uploads/` - Directory for stored files
- `Dockerfile` - Container definition for Docker deployment
- `Taskfile.yml` - Task definitions for build and deployment automation

## Setup

1. Install dependencies:
   ```
   go get github.com/labstack/echo/v4
   ```

2. Install templ for template compilation:
   ```
   go install github.com/a-h/templ/cmd/templ@latest
   ```

3. Update your go.mod file (or use the provided one):
   ```
   require (
     github.com/labstack/echo/v4 v4.10.2
     github.com/a-h/templ v0.2.501
   )
   ```

4. Build and run:
   ```
   templ generate
   go build
   ./nullpointer
   ```

5. Alternatively, use the Taskfile:
   ```
   task build
   task run
   ```

6. For Docker deployment:
   ```
   docker build -t nullpointer .
   docker run -p 8080:8080 -v ./uploads:/app/uploads nullpointer
   ```

## API Usage

### Uploading Files

```bash
# Upload a file
curl -F'file=@yourfile.png' https://domain.com/upload

# Upload from URL
curl -F'url=http://example.com/image.jpg' https://domain.com/upload

# Upload with a secret URL (harder to guess)
curl -F'file=@yourfile.png' -F'secret=' https://domain.com/upload

# Upload with custom expiration (24 hours)
curl -F'file=@yourfile.png' -F'expires=24' https://domain.com/upload
```

### Managing Files

```bash
# Delete a file
curl -X POST -F'token=your_token_here' -F'delete=' https://domain.com/filename.ext

# Update expiration
curl -X POST -F'token=your_token_here' -F'expires=48' https://domain.com/filename.ext
```

## File Retention

- Minimum age: Configured via config.json (default: 30 days)
- Maximum age: Configured via config.json (default: 365 days)
- Retention formula: `min_age + (min_age - max_age) * pow((file_size / max_size - 1), 3)`

## Configuration

The service uses a JSON configuration file located at `./config/config.json` with the following format:

```json
{
  "min_age": 30,
  "max_age": 365,
  "max_size": 100,
  "check_interval": 60
}
```

Parameters:
- `min_age`: Minimum file retention in days
- `max_age`: Maximum file retention in days
- `max_size`: Maximum file size in MiB that gets the maximum retention
- `check_interval`: How often to check for expired files (in minutes)


## Running with Docker

After building the Docker image, you can run the service with:

```bash
docker run -p 8080:8080 -v $(pwd)/uploads:/uploads -v $(pwd)/config:/config nullpointer --rm
```

This command:
- Maps port 8080 from the container to port 8080 on your host
- Mounts your local `uploads` directory to the container's `/uploads` directory for persistent storage
- Mounts your local `config` directory to the container's `/config` directory for custom configuration
- Uses the `--rm` flag to automatically remove the container when it exits

For production deployment, you might want to use a specific network and add environment variables:

```bash
docker build -t dump .

docker run -d --name dump \
  -p 80:8080 \
  -v $(pwd)/uploads:/uploads \
  -v $(pwd)/config:/config \
  --restart unless-stopped \
  dump
```

## License

MIT License
