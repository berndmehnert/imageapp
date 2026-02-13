package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"imageapp/internal/handlers"
	mw "imageapp/internal/middleware"
	"imageapp/internal/services"
	"imageapp/internal/ws"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	ctx := context.Background()

	// Storage
	os.MkdirAll("./storage", 0o755)
	os.MkdirAll("./storage/thumbnails", 0o755)

	// Database
	dbPool, err := pgxpool.New(ctx, os.Getenv("DATABASE_URL"))
	if err != nil {
		log.Fatalf("connect to db: %v", err)
	}
	defer dbPool.Close()

	if err := migrate(ctx, dbPool); err != nil {
		log.Fatalf("migrate: %v", err)
	}

	// Embedding Service
	embedder, err := services.NewEmbeddingService(
		"./model/model.onnx",
		"./model/tokenizer.json",
	)
	if err != nil {
		log.Fatalf("embedding service: %v", err)
	}
	defer embedder.Close()

	// WebSocket Hub
	hub := ws.NewHub()
	go hub.Run()

	// Image Processor (thumbnail + embedding)
	processor := services.NewImageProcessor(
		dbPool,
		"./storage",
		3, // 3 workers at the moment ...
		embedder,
		func(job services.ImageJob) {
			hub.Broadcast(ws.Message{
				Type:         "thumbnail_ready",
				ID:           job.FileID,
				Title:        job.Title,
				Tags:         job.Tags,
				ThumbnailURL: fmt.Sprintf("/thumbnails/%d", job.FileID),
			})
		},
	)
	defer processor.Shutdown()

	// Process any pending images from previous run
	go processPending(ctx, dbPool, processor)

	// Handlers
	uploadHandler := handlers.NewUploadHandler(dbPool, processor)
	feedHandler := handlers.NewFeedHandler(dbPool, embedder)

	// Add three initial images if the database is empty:
	go seedInitialImages(ctx, dbPool, uploadHandler)

	// Router
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(mw.CorsMiddleware)

	// Static files
	r.Handle("/uploads/*", http.StripPrefix("/uploads/",
		http.FileServer(http.Dir("./storage"))))
	r.Handle("/thumbnails/*", http.StripPrefix("/thumbnails/",
		http.FileServer(http.Dir("./storage/thumbnails"))))

	// API
	r.Route("/api", func(r chi.Router) {
		r.Post("/upload", uploadHandler.Upload)
		r.Get("/feed", feedHandler.Feed)
	})

	// WebSocket
	r.Get("/ws", func(w http.ResponseWriter, r *http.Request) {
		ws.HandleWebSocket(hub, w, r)
	})

	// graceful shutdown!
	srv := &http.Server{
		Addr:    ":8080",
		Handler: r,
	}

	go func() {
		log.Println("Server starting on :8080 ...")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server: %v", err)
		}
	}()

	// Wait for interrupt
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down...")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	srv.Shutdown(shutdownCtx)
	hub.Shutdown()
	processor.Shutdown()
	embedder.Close()
	dbPool.Close()
}

func migrate(ctx context.Context, db *pgxpool.Pool) error {
	_, err := db.Exec(ctx, `
		CREATE EXTENSION IF NOT EXISTS vector;
		CREATE TABLE IF NOT EXISTS images (
			id                BIGSERIAL PRIMARY KEY,
			title             TEXT NOT NULL,
			tags              TEXT[] NOT NULL,
			filename          TEXT NOT NULL,
			size              BIGINT NOT NULL,
			mime              TEXT NOT NULL,
			checksum          TEXT NOT NULL UNIQUE,
			storage_path      TEXT NOT NULL,
			image_url         TEXT NOT NULL,
			embedding         vector(384) NOT NULL,
			thumbnail_path    TEXT,
			thumbnail_status  TEXT NOT NULL DEFAULT 'pending',
			created_at        TIMESTAMPTZ DEFAULT NOW()
		);

		CREATE INDEX IF NOT EXISTS images_embedding_idx 
			ON images USING hnsw (embedding vector_cosine_ops);
	`)
	return err
}

func processPending(ctx context.Context, db *pgxpool.Pool, processor *services.ImageProcessor) {
	rows, err := db.Query(ctx, `
		SELECT id, storage_path, filename, title, tags
		FROM images 
		WHERE thumbnail_status = 'pending'
	`)
	if err != nil {
		log.Printf("Failed to get pending images: %v", err)
		return
	}
	defer rows.Close()

	count := 0
	for rows.Next() {
		var job services.ImageJob
		if err := rows.Scan(&job.FileID, &job.FilePath, &job.Filename, &job.Title, &job.Tags); err != nil {
			log.Printf("Failed to scan pending image: %v", err)
			continue
		}
		processor.Queue(job)
		count++
	}

	if count > 0 {
		log.Printf("Queued %d pending images for processing", count)
	}
}

func seedInitialImages(ctx context.Context, dbPool *pgxpool.Pool, uploader *handlers.UploadHandler) {
	// Check if already seeded
	var count int
	err := dbPool.QueryRow(ctx, "SELECT COUNT(*) FROM images").Scan(&count)
	if err != nil || count > 0 {
		return
	}

	log.Println("Seeding initial images...")

	seeds := []struct {
		path  string
		title string
		tags  []string
	}{
		{"./seeds/cat.png", "Cute cat sleeping", []string{"cat", "cute", "sleeping"}},
		{"./seeds/dog.jpg", "Dog in the park", []string{"dog", "park", "outdoor"}},
		{"./seeds/sunset.jpg", "Sunset over ocean", []string{"sunset", "ocean", "nature"}},
	}

	for _, seed := range seeds {
		if err := uploader.SeedImage(ctx, seed.path, seed.title, seed.tags); err != nil {
			log.Printf("Failed to seed %s: %v", seed.path, err)
			continue
		}
		log.Printf("Seeded: %s", seed.title)
	}
}
