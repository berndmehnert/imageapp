package services

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"

	"github.com/disintegration/imaging"
	"github.com/jackc/pgx/v5/pgxpool"
	pgvector "github.com/pgvector/pgvector-go"
)

type ImageJob struct {
	FileID   int64
	FilePath string
	Filename string
	Title    string
	Tags     []string
}

type OnComplete func(job ImageJob)
type ImageProcessor struct {
	jobs       chan ImageJob
	wg         sync.WaitGroup
	db         *pgxpool.Pool
	thumbDir   string
	maxWorkers int
	embedder   *EmbeddingService
	onComplete OnComplete
	once       sync.Once
}

func NewImageProcessor(db *pgxpool.Pool, baseDir string, maxWorkers int, embedder *EmbeddingService, onComplete OnComplete) *ImageProcessor {
	thumbDir := filepath.Join(baseDir, "thumbnails")
	os.MkdirAll(thumbDir, 0o755)

	p := &ImageProcessor{
		jobs:       make(chan ImageJob, 100),
		db:         db,
		thumbDir:   thumbDir,
		maxWorkers: maxWorkers,
		embedder:   embedder,
		onComplete: onComplete,
	}

	p.startWorkers()
	return p
}

func (p *ImageProcessor) startWorkers() {
	for i := 0; i < p.maxWorkers; i++ {
		p.wg.Add(1)
		go p.worker(i)
	}
}

func (p *ImageProcessor) worker(id int) {
	defer p.wg.Done()

	for job := range p.jobs {
		if err := p.processJob(job); err != nil {
			log.Printf("Worker %d: processing failed for file %d: %v", id, job.FileID, err)
			p.updateStatus(job.FileID, "failed")
		} else {
			log.Printf("Worker %d: processing complete for file %d", id, job.FileID)

			if p.onComplete != nil {
				p.onComplete(job)
			}
		}
	}
}
func (p *ImageProcessor) processJob(job ImageJob) error {
	p.updateStatus(job.FileID, "processing")

	thumbPath, err := p.createThumbnail(job)
	if err != nil {
		return fmt.Errorf("thumbnail: %w", err)
	}

	embedding, err := p.embedder.EmbedTags(job.Tags...)
	if err != nil {
		return fmt.Errorf("embedding: %w", err)
	}

	_, err = p.db.Exec(context.Background(), `
		UPDATE images 
		SET thumbnail_path = $1,
		    thumbnail_status = 'ready',
		    embedding = $2
		WHERE id = $3
	`, thumbPath, pgvector.NewVector(embedding), job.FileID)
	if err != nil {
		return fmt.Errorf("db update: %w", err)
	}

	return nil
}

func (p *ImageProcessor) createThumbnail(job ImageJob) (string, error) {
	src, err := imaging.Open(job.FilePath)
	if err != nil {
		return "", fmt.Errorf("open image: %w", err)
	}

	thumb := imaging.Fill(src, 512, 512, imaging.Center, imaging.Lanczos)

	thumbPath := filepath.Join(p.thumbDir, fmt.Sprintf("thumb_%d.jpg", job.FileID))
	if err := imaging.Save(thumb, thumbPath, imaging.JPEGQuality(80)); err != nil {
		return "", fmt.Errorf("save thumbnail: %w", err)
	}

	return thumbPath, nil
}

func (p *ImageProcessor) updateStatus(id int64, status string) {
	_, err := p.db.Exec(context.Background(), `
		UPDATE images 
		SET thumbnail_status = $1
		WHERE id = $2
	`, status, id)
	if err != nil {
		log.Printf("Failed to update status for image %d: %v", id, err)
	}
}
func (p *ImageProcessor) Queue(job ImageJob) {
	select {
	case p.jobs <- job:
	default:
		log.Printf("Warning: job queue full, skipping image %d", job.FileID)
	}
}

func (p *ImageProcessor) Shutdown() {
	p.once.Do(func() {
		close(p.jobs)
		p.wg.Wait()
		p.embedder.Close()
	})
}
