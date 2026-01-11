package storage

import "net/http"

// RegisterRoutes registers all storage routes on the provided ServeMux.
// This is a placeholder for future storage implementation.
func RegisterRoutes(app *http.ServeMux) {
	// TODO: Implement storage routes
	// - PUT /storage/{bucket}/{path...} - Upload file
	// - GET /storage/{bucket}/{path...} - Download file
	// - DELETE /storage/{bucket}/{path...} - Delete file
	// - GET /storage/{bucket} - List files in bucket
}
