package handlers

import (
	"fmt"
	"net/http"
	"path/filepath"

	"github.com/go-chi/chi/v5"
)

func ThumbnailHandler(thumbDir string) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        fileID := chi.URLParam(r, "fileID")
        http.ServeFile(w, r, filepath.Join(thumbDir, fmt.Sprintf("thumb_%s.jpg", fileID)))
    }
}