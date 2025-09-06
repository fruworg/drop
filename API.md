# Drop API Documentation

Complete API reference for the Drop file hosting service.

## Table of Contents

- [Upload API](#upload-api)
- [Chunked Upload API](#chunked-upload-api)
- [File Management API](#file-management-api)
- [Response Formats](#response-formats)
- [Expiration Formats](#expiration-formats)

## Upload API

### Regular Upload

Upload files using multipart/form-data POST requests to the root endpoint.

**Endpoint:** `POST /`

**Parameters:**
- `file` - File data (multipart/form-data)
- `url` - Remote URL to download from (mutually exclusive with `file`)
- `secret` - Generate hard-to-guess URL (optional)
- `one_time` - Delete file after first download/view (optional)
- `expires` - Custom expiration time (optional)

**Examples:**

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

# Combining options
curl -F'file=@yourfile.png' -F'one_time=' -F'secret=' -F'expires=24' http://localhost:3000/
```

## Chunked Upload API

For large files, use the chunked upload feature which provides resume capability, progress tracking, and memory efficiency.

### Initialize Upload

**Endpoint:** `POST /upload/init`

**Parameters:**
- `filename` - Original filename
- `size` - Total file size in bytes
- `chunk_size` - Custom chunk size in bytes (optional, default: 4MB)

**Example:**
```bash
curl -X POST http://localhost:3000/upload/init \
    -F "filename=large-video.mp4" \
    -F "size=104857600" \
    -F "chunk_size=4194304"
```

**Response:**
```json
{
  "upload_id": "abc123",
  "chunk_size": 4194304,
  "total_chunks": 25,
  "uploaded_chunks": []
}
```

### Upload Chunks

**Endpoint:** `POST /upload/chunk/{upload_id}/{chunk_index}`

**Parameters:**
- `chunk` - Chunk data (multipart/form-data)

**Example:**
```bash
curl -X POST http://localhost:3000/upload/chunk/abc123/0 \
    -F "chunk=@chunk_0.bin"
```

### Check Upload Status

**Endpoint:** `GET /upload/status/{upload_id}`

**Example:**
```bash
curl http://localhost:3000/upload/status/abc123
```

**Response:**
```json
{
  "upload_id": "abc123",
  "filename": "large-file.zip",
  "total_size": 104857600,
  "chunk_size": 4194304,
  "total_chunks": 25,
  "uploaded_chunks": [0,1,2,3,4],
  "progress": 20,
  "created_at": "2024-01-01T10:00:00Z",
  "expires_at": "2024-01-02T10:00:00Z"
}
```

### Resume Upload

```bash
# Check which chunks are missing
curl http://localhost:3000/upload/status/abc123

# Upload only the missing chunks
curl -X POST http://localhost:3000/upload/chunk/abc123/15 \
    -F "chunk=@chunk_15.bin"
```

## File Management API

### Delete File

**Endpoint:** `POST /{filename}`

**Parameters:**
- `token` - Management token (from upload response)
- `delete` - Delete flag (any value)

**Example:**
```bash
curl -X POST -F'token=your_token_here' -F'delete=' http://localhost:3000/filename.ext
```

### Update Expiration

**Endpoint:** `POST /{filename}`

**Parameters:**
- `token` - Management token (from upload response)
- `expires` - New expiration time

**Example:**
```bash
curl -X POST -F'token=your_token_here' -F'expires=48' http://localhost:3000/filename.ext
```

## Response Formats

### Regular Upload Response (JSON)

When uploading with `Accept: application/json` header:

```json
{
  "url": "http://localhost:3000/abc123.txt",
  "size": 1024,
  "token": "management_token_here",
  "md5": "d41d8cd98f00b204e9800998ecf8427e",
  "expires_at": "2024-12-31T23:59:59Z",
  "expires_in_days": 30
}
```

### Chunked Upload Completion Response (JSON)

When chunked upload completes successfully:

```json
{
  "message": "Upload completed",
  "progress": 100,
  "file_url": "http://localhost:3000/abc123.txt",
  "md5": "d41d8cd98f00b204e9800998ecf8427e",
  "token": "management_token_here",
  "expires_at": "2024-12-31T23:59:59Z",
  "expires_in_days": 30
}
```

### Response Fields

| Field | Type | Description |
|-------|------|-------------|
| `url` / `file_url` | string | Direct file access URL |
| `size` | integer | File size in bytes |
| `token` | string | Management token for file operations |
| `md5` | string | MD5 hash of the uploaded file |
| `expires_at` | string | Expiration date (RFC3339 format) |
| `expires_in_days` | integer | Days until expiration |
| `message` | string | Status message (chunked uploads) |
| `progress` | integer | Upload progress percentage (0-100) |

### MD5 Hash Benefits

- **File Integrity**: Verify uploaded files haven't been corrupted
- **Duplicate Detection**: Compare MD5 hashes to identify duplicate files
- **Data Validation**: Ensure file integrity during transfer
- **Audit Trail**: Hash can be used for file tracking and verification

**Note:** MD5 hash is calculated automatically after upload completion. If calculation fails, the field will be an empty string.

## Expiration Formats

The `expires` parameter accepts multiple formats:

- **Hours as integer** (e.g., `24`)
- **Unix timestamp in milliseconds** (e.g., `1681996320000`)
- **RFC3339** (e.g., `2023-04-20T10:15:30Z`)
- **ISO date** (e.g., `2023-04-20`)
- **ISO datetime** (e.g., `2023-04-20T10:15:30`)
- **SQL datetime** (e.g., `2023-04-20 15:04:05`)

### Examples

```bash
# Hours
curl -F'file=@file.png' -F'expires=24' http://localhost:3000/

# Unix timestamp
curl -F'file=@file.png' -F'expires=1681996320000' http://localhost:3000/

# RFC3339
curl -F'file=@file.png' -F'expires=2023-04-20T10:15:30Z' http://localhost:3000/

# ISO date
curl -F'file=@file.png' -F'expires=2023-04-20' http://localhost:3000/

# ISO datetime
curl -F'file=@file.png' -F'expires=2023-04-20T10:15:30' http://localhost:3000/

# SQL datetime
curl -F'file=@file.png' -F'expires="2023-04-20 10:15:30"' http://localhost:3000/
```

## Error Responses

All endpoints return appropriate HTTP status codes:

- `200 OK` - Success
- `400 Bad Request` - Invalid request parameters
- `404 Not Found` - File or upload session not found
- `413 Payload Too Large` - File exceeds size limit
- `500 Internal Server Error` - Server error

Error responses include a JSON object with an `error` field:

```json
{
  "error": "File too large"
}
```
