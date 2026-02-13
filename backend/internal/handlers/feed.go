package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"imageapp/internal/services"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	pgvector "github.com/pgvector/pgvector-go"
)

type FeedItem struct {
	ID           int64     `json:"id"`
	Title        string    `json:"title"`
	Tags         []string  `json:"tags"`
	ImageURL     string    `json:"image_url"`
	ThumbnailURL string    `json:"thumbnail_url"`
	CreatedAt    time.Time `json:"created_at"`
	Score        *float64  `json:"score,omitempty"`
}

type FeedHandler struct {
	db       *pgxpool.Pool
	embedder *services.EmbeddingService
}

func NewFeedHandler(db *pgxpool.Pool, embedder *services.EmbeddingService) *FeedHandler {
	return &FeedHandler{
		db:       db,
		embedder: embedder,
	}
}

func (h *FeedHandler) Feed(w http.ResponseWriter, r *http.Request) {
	cursor := r.URL.Query().Get("cursor")
	filter := r.URL.Query().Get("filter")
	limitStr := r.URL.Query().Get("limit")

	limit := 20
	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 50 {
			limit = l
		}
	}

	var items []FeedItem
	var err error

	if filter != "" {
		items, err = h.filteredFeed(r.Context(), filter, cursor, limit)
	} else {
		items, err = h.normalFeed(r.Context(), cursor, limit)
	}

	if err != nil {
		log.Printf("Feed error: %v", err) // add logging!
		http.Error(w, "query failed", http.StatusInternalServerError)
		return
	}

	var nextCursor string
	if len(items) == limit {
		nextCursor = items[len(items)-1].CreatedAt.Format(time.RFC3339Nano)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"items":       items,
		"next_cursor": nextCursor,
		"filter":      filter,
	})
}

func (h *FeedHandler) normalFeed(ctx context.Context, cursor string, limit int) ([]FeedItem, error) {
	var rows pgx.Rows
	var err error

	if cursor == "" {
		rows, err = h.db.Query(ctx, `
			SELECT id, title, tags, image_url, thumbnail_path, created_at
			FROM images
			WHERE thumbnail_status = 'ready'
			ORDER BY created_at DESC
			LIMIT $1
		`, limit)
	} else {
		cursorTime, parseErr := time.Parse(time.RFC3339Nano, cursor)
		if parseErr != nil {
			return nil, fmt.Errorf("invalid cursor: %w", parseErr)
		}
		rows, err = h.db.Query(ctx, `
			SELECT id, title, tags, image_url, thumbnail_path, created_at
			FROM images
			WHERE thumbnail_status = 'ready'
			  AND created_at < $1
			ORDER BY created_at DESC
			LIMIT $2
		`, cursorTime, limit)
	}

	if err != nil {
		return nil, fmt.Errorf("query: %w", err)
	}
	defer rows.Close()

	return scanFeedItems(rows)
}

func (h *FeedHandler) filteredFeed(ctx context.Context, filter, cursor string, limit int) ([]FeedItem, error) {
	filterVec, err := h.embedder.EmbedTags(filter)
	if err != nil {
		return nil, fmt.Errorf("embed filter: %w", err)
	}

	var rows pgx.Rows

	if cursor == "" {
		rows, err = h.db.Query(ctx, `
			SELECT id, title, tags, image_url, thumbnail_path, created_at,
			       1 - (embedding <=> $1) AS similarity
			FROM images
			WHERE thumbnail_status = 'ready'
			  AND 1 - (embedding <=> $1) > 0.3
			ORDER BY similarity DESC
			LIMIT $2
		`, pgvector.NewVector(filterVec), limit)
	} else {
		cursorTime, parseErr := time.Parse(time.RFC3339Nano, cursor)
		if parseErr != nil {
			return nil, fmt.Errorf("invalid cursor: %w", parseErr)
		}
		rows, err = h.db.Query(ctx, `
			SELECT id, title, tags, image_url, thumbnail_path, created_at,
			       1 - (embedding <=> $1) AS similarity
			FROM images
			WHERE thumbnail_status = 'ready'
			  AND 1 - (embedding <=> $1) > 0.3
			  AND created_at < $2
			ORDER BY similarity DESC
			LIMIT $3
		`, pgvector.NewVector(filterVec), cursorTime, limit)
	}

	if err != nil {
		return nil, fmt.Errorf("query: %w", err)
	}
	defer rows.Close()

	return scanFeedItemsWithScore(rows)
}

func scanFeedItems(rows pgx.Rows) ([]FeedItem, error) {
	var items []FeedItem
	for rows.Next() {
		var item FeedItem
		var thumbPath *string
		if err := rows.Scan(&item.ID, &item.Title, &item.Tags,
			&item.ImageURL, &thumbPath, &item.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan: %w", err)
		}
		if thumbPath != nil {
			item.ThumbnailURL = fmt.Sprintf("/thumbnails/thumb_%d.jpg", item.ID)
		}
		items = append(items, item)
	}
	return items, nil
}

func scanFeedItemsWithScore(rows pgx.Rows) ([]FeedItem, error) {
	var items []FeedItem
	for rows.Next() {
		var item FeedItem
		var thumbPath *string
		var score float64
		if err := rows.Scan(&item.ID, &item.Title, &item.Tags,
			&item.ImageURL, &thumbPath, &item.CreatedAt, &score); err != nil {
			return nil, fmt.Errorf("scan: %w", err)
		}
		item.Score = &score
		if thumbPath != nil {
			item.ThumbnailURL = fmt.Sprintf("/thumbnails/thumb_%d.jpg", item.ID)
		}
		items = append(items, item)
	}
	return items, nil
}
