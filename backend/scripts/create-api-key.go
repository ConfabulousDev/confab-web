package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/santaclaude2025/confab/backend/internal/auth"
	"github.com/santaclaude2025/confab/backend/internal/db"
)

func main() {
	// Connect to database
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = "postgres://confab:confab@localhost:5432/confab?sslmode=disable"
	}

	database, err := db.Connect(dsn)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer database.Close()

	// Generate API key
	rawKey, keyHash, err := auth.GenerateAPIKey()
	if err != nil {
		log.Fatalf("Failed to generate API key: %v", err)
	}

	// Create API key for default user (ID=1)
	userID := int64(1)
	name := "default-cli-key"
	if len(os.Args) > 1 {
		name = os.Args[1]
	}

	ctx := context.Background()
	_, _, err = database.CreateAPIKeyWithReturn(ctx, userID, keyHash, name)
	if err != nil {
		log.Fatalf("Failed to create API key: %v", err)
	}

	fmt.Println("=== API Key Created ===")
	fmt.Printf("User ID: %d\n", userID)
	fmt.Printf("Name: %s\n", name)
	fmt.Println()
	fmt.Println("API Key (save this, it won't be shown again):")
	fmt.Println(rawKey)
	fmt.Println()
	fmt.Println("Configure confab CLI with:")
	fmt.Printf("  confab cloud configure --backend-url http://localhost:8080 --api-key %s --enable\n", rawKey)
}
