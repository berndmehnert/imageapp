package handlers

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"imageapp/internal/services"

	"github.com/jackc/pgx/v5/pgxpool"
	pgvector "github.com/pgvector/pgvector-go"
)

const (
	maxUploadSize = 50 * 1024 * 1024 // 50 MB for images, this should be enough ...
	storageDir    = "./storage"
)

type UploadHandler struct {
	db             *pgxpool.Pool
	imageProcessor *services.ImageProcessor
}

func NewUploadHandler(db *pgxpool.Pool, processor *services.ImageProcessor) *UploadHandler {
	os.MkdirAll(storageDir, 0o755)
	return &UploadHandler{
		db:             db,
		imageProcessor: processor,
	}
}

func (h *UploadHandler) Upload(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	r.Body = http.MaxBytesReader(w, r.Body, maxUploadSize)
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		http.Error(w, "invalid multipart form: "+err.Error(), http.StatusBadRequest)
		return
	}

	title := r.FormValue("title")
	if title == "" {
		http.Error(w, "title is required", http.StatusBadRequest)
		return
	}

	var tags []string
	tagsRaw := r.FormValue("tags")
	if tagsRaw != "" {
		if err := json.Unmarshal([]byte(tagsRaw), &tags); err != nil {
			http.Error(w, "invalid tags format", http.StatusBadRequest)
			return
		}
	}
	if len(tags) == 0 {
		http.Error(w, "at least one tag is required", http.StatusBadRequest)
		return
	}

	file, fh, err := r.FormFile("image")
	if err != nil {
		http.Error(w, "missing image field: "+err.Error(), http.StatusBadRequest)
		return
	}
	defer file.Close()

	mime := fh.Header.Get("Content-Type")
	if !isAllowedMime(mime) {
		http.Error(w, "unsupported image format", http.StatusBadRequest)
		return
	}

	bytes, err := io.ReadAll(file)
	if err != nil {
		http.Error(w, "failed to read file", http.StatusInternalServerError)
		return
	}

	// use the core function
	result, err := h.processUpload(ctx, bytes, fh.Filename, mime, title, tags)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(result)
}

func (h *UploadHandler) SeedImage(ctx context.Context, imagePath, title string, tags []string) error {
	bytes, err := os.ReadFile(imagePath)
	if err != nil {
		return fmt.Errorf("read seed image: %w", err)
	}

	filename := filepath.Base(imagePath)
	mime := detectMime(filename)

	_, err = h.processUpload(ctx, bytes, filename, mime, title, tags)
	return err
}

func (h *UploadHandler) processUpload(ctx context.Context, bytes []byte, filename, mime, title string, tags []string) (map[string]any, error) {
	// Checksum
	hash := sha256.Sum256(bytes)
	checksum := hex.EncodeToString(hash[:])

	// Duplicate check
	var exists bool
	err := h.db.QueryRow(ctx,
		"SELECT EXISTS(SELECT 1 FROM images WHERE checksum = $1)", checksum,
	).Scan(&exists)
	if err != nil {
		return nil, fmt.Errorf("database error: %w", err)
	}
	if exists {
		return nil, fmt.Errorf("image already exists")
	}

	// Save to disk
	finalName := fmt.Sprintf("%d_%s", time.Now().UnixNano(), filepath.Base(filename))
	storagePath := filepath.Join(storageDir, finalName)
	if err := os.WriteFile(storagePath, bytes, 0o644); err != nil {
		return nil, fmt.Errorf("save file: %w", err)
	}

	// Insert into database
	imageURL := fmt.Sprintf("/uploads/%s", finalName)
	var id int64
	var createdAt time.Time

	err = h.db.QueryRow(ctx, `
		INSERT INTO images (title, tags, filename, size, mime, checksum,
		                    storage_path, image_url, embedding, thumbnail_status)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, 'pending')
		RETURNING id, created_at
	`,
		title,
		tags,
		filename,
		int64(len(bytes)),
		mime,
		checksum,
		storagePath,
		imageURL,
		pgvector.NewVector(make([]float32, 384)),
	).Scan(&id, &createdAt)

	if err != nil {
		os.Remove(storagePath)
		return nil, fmt.Errorf("db insert: %w", err)
	}

	// the downloaded image will now be processed ...
	h.imageProcessor.Queue(services.ImageJob{
		FileID:   id,
		FilePath: storagePath,
		Filename: filename,
		Title:    title,
		Tags:     tags,
	})

	return map[string]any{
		"id":        id,
		"title":     title,
		"tags":      tags,
		"image_url": imageURL,
		"status":    "processing",
	}, nil
}

// previously I had here a pipe based save installed, which seems not necessary now since we are dealing with images ..
func saveFile(ctx context.Context, src multipart.File, fh *multipart.FileHeader) (string, string, int64, error) {
	if err := os.MkdirAll(storageDir, 0o755); err != nil {
		return "", "", 0, err
	}

	bytes, err := io.ReadAll(src)
	if err != nil {
		return "", "", 0, err
	}

	hash := sha256.Sum256(bytes)
	checksum := hex.EncodeToString(hash[:])

	finalName := fmt.Sprintf("%d_%s", time.Now().UnixNano(), filepath.Base(fh.Filename))
	finalPath := filepath.Join(storageDir, finalName)

	if err := os.WriteFile(finalPath, bytes, 0o644); err != nil {
		return "", "", 0, err
	}

	return finalPath, checksum, int64(len(bytes)), nil
}

func detectMime(filename string) string {
	switch filepath.Ext(filename) {
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".webp":
		return "image/webp"
	case ".gif":
		return "image/gif"
	default:
		return "application/octet-stream"
	}
}

func isAllowedMime(mime string) bool {
	allowed := map[string]bool{
		"image/jpeg": true,
		"image/png":  true,
		"image/webp": true,
		"image/gif":  true,
	}
	return allowed[mime]
}
