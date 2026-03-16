package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/ABVitali/recipe-extractor/internal/config"
	"github.com/ABVitali/recipe-extractor/internal/db"
	"github.com/ABVitali/recipe-extractor/internal/extract"
	"github.com/ABVitali/recipe-extractor/internal/llm"
	"github.com/ABVitali/recipe-extractor/internal/pdf"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "Usage:\n")
		fmt.Fprintf(os.Stderr, "  recipe-extractor extract <pdf-file> <book-name>\n")
		fmt.Fprintf(os.Stderr, "  recipe-extractor migrate up|down\n")
		os.Exit(1)
	}

	switch os.Args[1] {
	case "migrate":
		runMigrate()
	case "extract":
		runExtract()
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", os.Args[1])
		os.Exit(1)
	}
}

func runMigrate() {
	if len(os.Args) < 3 {
		log.Fatal("Usage: recipe-extractor migrate up|down")
	}
	direction := os.Args[2]

	cfg, err := config.Load("db")
	if err != nil {
		log.Fatalf("Configuration error: %v", err)
	}

	repo, err := db.New(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer repo.Close()

	switch direction {
	case "up":
		if err := repo.MigrateUp(cfg.MigrationsPath); err != nil {
			log.Fatalf("Migration up failed: %v", err)
		}
		fmt.Println("Migrations applied successfully.")
	case "down":
		if err := repo.MigrateDown(cfg.MigrationsPath); err != nil {
			log.Fatalf("Migration down failed: %v", err)
		}
		fmt.Println("Migrations rolled back successfully.")
	default:
		log.Fatalf("Unknown migration direction: %s (use 'up' or 'down')", direction)
	}
}

func runExtract() {
	if len(os.Args) < 4 {
		log.Fatal("Usage: recipe-extractor extract <pdf-file> <book-name>")
	}
	pdfPath := os.Args[2]
	bookName := os.Args[3]

	if err := pdf.ValidatePath(pdfPath); err != nil {
		log.Fatal(err)
	}

	cfg, err := config.Load("all")
	if err != nil {
		log.Fatalf("Configuration error: %v", err)
	}

	// Build dependencies
	repo, err := db.New(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer repo.Close()
	log.Printf("[db] connected successfully")

	client := llm.New(cfg.LLMBaseURL, cfg.LLMAPIKey, cfg.LLMModel, cfg.LLMMaxTokens, "")

	// Run extraction
	orch := extract.New(client, repo, cfg)
	result, err := orch.Run(context.Background(), pdfPath, bookName)
	if err != nil {
		log.Fatalf("Extraction failed: %v", err)
	}

	fmt.Printf("\nOutput saved to: %s\n", result.OutputDir)
}
