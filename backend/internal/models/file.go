package models

import (
	"time"

	pgvector "github.com/pgvector/pgvector-go"
)

type Image struct {
	ID              int64           `db:"id" json:"id"`
	Title           string          `db:"title" json:"title"`
	Tags            []string        `db:"tags" json:"tags"`
	Filename        string          `db:"filename" json:"filename"`
	Size            int64           `db:"size" json:"size"`
	Mime            string          `db:"mime" json:"mime"`
	Checksum        string          `db:"checksum" json:"checksum"`
	StoragePath     string          `db:"storage_path" json:"-"`
	ImageURL        string          `db:"image_url" json:"image_url"`
	Embedding       pgvector.Vector `db:"embedding" json:"-"`
	ThumbnailPath   *string         `db:"thumbnail_path" json:"-"`
	ThumbnailStatus string          `db:"thumbnail_status" json:"thumbnail_status"`
	CreatedAt       time.Time       `db:"created_at" json:"created_at"`
}
