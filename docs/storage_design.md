# Atomicbase Storage - Design Document

Simple object storage service. Files stored at paths, served with correct content types.

## Philosophy

Storage is a separate service from the database. You store file URLs/paths in regular TEXT columns. No special abstractions - just PUT/GET/DELETE objects at paths.

## Metadata Schema

One table per database, storing object metadata:

```sql
CREATE TABLE _storage_objects (
    path TEXT PRIMARY KEY,      -- "avatars/user123.png"
    size INTEGER NOT NULL,
    mime_type TEXT NOT NULL,    -- "image/png"
    created_at INTEGER NOT NULL -- unix epoch
);
```

Actual file bytes live on disk. Metadata lives in the database.

## API

### Upload

```
PUT /storage/{db}/{path}
Content-Type: image/png
Body: <file bytes>
```

- Detects mime type from content (don't trust Content-Type header)
- Stores bytes to disk
- Inserts row into `_storage_objects`

Response: `201 Created`

```json
{
  "path": "avatars/user123.png",
  "size": 24680,
  "mime_type": "image/png"
}
```

### Download

```
GET /storage/{db}/{path}
```

- Looks up `mime_type` from `_storage_objects`
- Returns file bytes with correct `Content-Type` header

Response: File bytes with headers:

- `Content-Type`: from stored mime_type
- `Content-Length`: from stored size

### Delete

```
DELETE /storage/{db}/{path}
```

- Removes file from disk
- Deletes row from `_storage_objects`

Response: `204 No Content`

### List (optional)

```
GET /storage/{db}?prefix=avatars/
```

Response:

```json
{
  "objects": [
    {
      "path": "avatars/user123.png",
      "size": 24680,
      "mime_type": "image/png",
      "created_at": 1705312200
    },
    {
      "path": "avatars/user456.png",
      "size": 31240,
      "mime_type": "image/png",
      "created_at": 1705312500
    }
  ]
}
```

## Storage Layout

```
data/
  storage/
    {db_name}/
      avatars/
        user123.png
        user456.png
      documents/
        report.pdf
```

Path structure: `data/storage/{db}/{path}`

## Future Considerations

Not in v0, but may add later:

- **Access control**: Permissions/policies for who can read/write
- **Automatic cleanup**: Delete files when referenced rows are deleted
- **S3 backend**: Store files in S3-compatible storage instead of local disk
- **Thumbnails**: On-demand image resizing
- **Protected files**: Signed URLs for private files

## SDK Interface

```typescript
// Upload
const { data, error } = await client.storage.upload(
  "avatars/user123.png",
  file
);

// Get URL
const url = client.storage.getUrl("avatars/user123.png");
// => "https://api.example.com/storage/mydb/avatars/user123.png"

// Delete
await client.storage.delete("avatars/user123.png");

// List
const { data, error } = await client.storage.list({ prefix: "avatars/" });
```

## Implementation Notes

### MIME Type Detection

Detect from file content, not from filename or Content-Type header:

```go
func detectMimeType(file io.Reader) string {
    buffer := make([]byte, 512)
    n, _ := file.Read(buffer)
    return http.DetectContentType(buffer[:n])
}
```

### Path Sanitization

Prevent path traversal attacks:

```go
func sanitizePath(path string) (string, error) {
    // Clean the path
    cleaned := filepath.Clean(path)

    // Reject paths that try to escape
    if strings.HasPrefix(cleaned, "..") || strings.HasPrefix(cleaned, "/") {
        return "", ErrInvalidPath
    }

    return cleaned, nil
}
```

### Serving Files

```go
func serveFile(w http.ResponseWriter, path string, mimeType string) {
    w.Header().Set("Content-Type", mimeType)
    w.Header().Set("X-Content-Type-Options", "nosniff")
    http.ServeFile(w, r, path)
}
```
