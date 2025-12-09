// backfill-chunk-counts.go
//
// One-time script to backfill chunk_count for existing sync_files records.
// Queries all files where chunk_count IS NULL, counts S3 chunks, and updates DB.
//
// Usage:
//   DATABASE_URL=... S3_ENDPOINT=... AWS_ACCESS_KEY_ID=... AWS_SECRET_ACCESS_KEY=... BUCKET_NAME=... go run scripts/backfill-chunk-counts.go
//
// Flags:
//   -dry-run    Print what would be updated without making changes
//   -batch      Batch size for processing (default: 100)

package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type syncFile struct {
	sessionID  string
	fileName   string
	userID     int64
	externalID string
}

func main() {
	dryRun := flag.Bool("dry-run", false, "Print what would be updated without making changes")
	batchSize := flag.Int("batch", 100, "Batch size for processing")
	flag.Parse()

	// Load config from environment
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		log.Fatal("DATABASE_URL is required")
	}

	s3Endpoint := os.Getenv("S3_ENDPOINT")
	if s3Endpoint == "" {
		log.Fatal("S3_ENDPOINT is required")
	}

	accessKey := os.Getenv("AWS_ACCESS_KEY_ID")
	if accessKey == "" {
		log.Fatal("AWS_ACCESS_KEY_ID is required")
	}

	secretKey := os.Getenv("AWS_SECRET_ACCESS_KEY")
	if secretKey == "" {
		log.Fatal("AWS_SECRET_ACCESS_KEY is required")
	}

	bucketName := os.Getenv("BUCKET_NAME")
	if bucketName == "" {
		log.Fatal("BUCKET_NAME is required")
	}

	useSSL := os.Getenv("S3_USE_SSL") != "false"

	// Connect to database
	db, err := sql.Open("pgx", dbURL)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		log.Fatalf("Failed to ping database: %v", err)
	}
	log.Println("Connected to database")

	// Connect to S3
	s3Client, err := minio.New(s3Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure: useSSL,
	})
	if err != nil {
		log.Fatalf("Failed to create S3 client: %v", err)
	}
	log.Println("Connected to S3")

	// Query files with NULL chunk_count
	ctx := context.Background()
	query := `
		SELECT sf.session_id, sf.file_name, s.user_id, s.external_id
		FROM sync_files sf
		JOIN sessions s ON sf.session_id = s.id
		WHERE sf.chunk_count IS NULL
		ORDER BY sf.created_at
		LIMIT $1
	`

	totalProcessed := 0
	totalUpdated := 0
	totalErrors := 0

	for {
		rows, err := db.QueryContext(ctx, query, *batchSize)
		if err != nil {
			log.Fatalf("Failed to query sync_files: %v", err)
		}

		var files []syncFile
		for rows.Next() {
			var f syncFile
			if err := rows.Scan(&f.sessionID, &f.fileName, &f.userID, &f.externalID); err != nil {
				log.Printf("Error scanning row: %v", err)
				continue
			}
			files = append(files, f)
		}
		rows.Close()

		if len(files) == 0 {
			break
		}

		log.Printf("Processing batch of %d files...", len(files))

		for _, f := range files {
			totalProcessed++

			// Count chunks in S3
			prefix := fmt.Sprintf("%d/claude-code/%s/chunks/%s/", f.userID, f.externalID, f.fileName)
			chunkCount := 0

			objectCh := s3Client.ListObjects(ctx, bucketName, minio.ListObjectsOptions{
				Prefix:    prefix,
				Recursive: true,
			})

			for obj := range objectCh {
				if obj.Err != nil {
					log.Printf("Error listing %s: %v", prefix, obj.Err)
					totalErrors++
					continue
				}
				chunkCount++
			}

			if *dryRun {
				log.Printf("[DRY-RUN] Would set chunk_count=%d for session=%s file=%s", chunkCount, f.sessionID, f.fileName)
			} else {
				updateQuery := `UPDATE sync_files SET chunk_count = $1, updated_at = NOW() WHERE session_id = $2 AND file_name = $3`
				_, err := db.ExecContext(ctx, updateQuery, chunkCount, f.sessionID, f.fileName)
				if err != nil {
					log.Printf("Error updating session=%s file=%s: %v", f.sessionID, f.fileName, err)
					totalErrors++
					continue
				}
				totalUpdated++
				log.Printf("Updated session=%s file=%s chunk_count=%d", f.sessionID, f.fileName, chunkCount)
			}
		}

		// Small delay to avoid overwhelming the systems
		time.Sleep(100 * time.Millisecond)
	}

	log.Println("========================================")
	log.Printf("Backfill complete:")
	log.Printf("  Total processed: %d", totalProcessed)
	if *dryRun {
		log.Printf("  Would update: %d", totalProcessed-totalErrors)
	} else {
		log.Printf("  Updated: %d", totalUpdated)
	}
	log.Printf("  Errors: %d", totalErrors)
}
