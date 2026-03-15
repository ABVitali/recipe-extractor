package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/ABVitali/recipe-extractor/internal/db"
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

	dbURL := requireEnv("DATABASE_URL")
	repo, err := db.New(dbURL)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer repo.Close()

	migrationsPath := envOrDefault("MIGRATIONS_PATH", "migrations")

	switch direction {
	case "up":
		if err := repo.MigrateUp(migrationsPath); err != nil {
			log.Fatalf("Migration up failed: %v", err)
		}
		fmt.Println("Migrations applied successfully.")
	case "down":
		if err := repo.MigrateDown(migrationsPath); err != nil {
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

	dbURL := requireEnv("DATABASE_URL")
	llmBaseURL := requireEnv("LLM_BASE_URL")
	llmAPIKey := requireEnv("LLM_API_KEY")
	chunkSize := envOrDefaultInt("MAX_PAGES_PER_CHUNK", 5)
	llmModel := envOrDefault("LLM_MODEL", "claude-opus-4-6")
	llmMaxTokens := envOrDefaultInt64("LLM_MAX_TOKENS", 128000)

	// Create output directory for this run
	runID := time.Now().Format("2006-01-02_15-04-05")
	outputDir := filepath.Join("output", runID)
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		log.Fatalf("Failed to create output directory: %v", err)
	}

	log.Printf("[config] pdf=%s book=%q", pdfPath, bookName)
	log.Printf("[config] model=%s max_tokens=%d chunk_size=%d", llmModel, llmMaxTokens, chunkSize)
	log.Printf("[config] output_dir=%s", outputDir)

	ctx := context.Background()

	// Extract text from PDF
	log.Printf("[pdf] extracting text from %s...", pdfPath)
	pdfStart := time.Now()
	pages, err := pdf.ExtractPages(pdfPath)
	if err != nil {
		log.Fatalf("Failed to extract PDF: %v", err)
	}

	totalChars := 0
	for _, p := range pages {
		totalChars += len(p.Text)
	}
	log.Printf("[pdf] extracted %d pages (%d total chars) in %s", len(pages), totalChars, time.Since(pdfStart))

	// Save extracted text for reference
	saveExtractedText(outputDir, pages)

	// Chunk pages for LLM processing (0 = send all pages at once)
	var chunks [][]pdf.Page
	if chunkSize <= 0 {
		chunks = [][]pdf.Page{pages}
		log.Printf("[chunk] sending all %d pages in a single request", len(pages))
	} else {
		chunks = pdf.ChunkPages(pages, chunkSize)
		log.Printf("[chunk] split into %d chunks of up to %d pages each", len(chunks), chunkSize)
	}

	// Connect to DB
	log.Printf("[db] connecting to database...")
	repo, err := db.New(dbURL)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer repo.Close()
	log.Printf("[db] connected successfully")

	// Process each chunk
	client := llm.New(llmBaseURL, llmAPIKey, llmModel, llmMaxTokens, outputDir)
	var totalRecipes, totalSaved, totalInvalid, totalDBErrors int
	extractStart := time.Now()

	for i, chunk := range chunks {
		log.Printf("[extract] --- chunk %d/%d (pages %d-%d) ---",
			i+1, len(chunks), chunk[0].Number, chunk[len(chunk)-1].Number)

		result, err := client.ExtractRecipes(ctx, chunk, bookName, i+1)
		if err != nil {
			log.Printf("[extract] chunk %d FAILED: %v", i+1, err)
			continue
		}

		totalRecipes += len(result.Recipes)
		log.Printf("[extract] chunk %d: got %d recipes (truncated=%v)", i+1, len(result.Recipes), result.Truncated)

		for j := range result.Recipes {
			result.Recipes[j].ExtractedAt = time.Now()

			if err := result.Recipes[j].Validate(); err != nil {
				log.Printf("[validate] INVALID recipe %q: %v", result.Recipes[j].Title, err)
				totalInvalid++
				continue
			}

			id, err := repo.InsertRecipe(ctx, &result.Recipes[j])
			if err != nil {
				log.Printf("[db] FAILED to insert %q: %v", result.Recipes[j].Title, err)
				totalDBErrors++
				continue
			}

			log.Printf("[db] saved: %q (id=%d)", result.Recipes[j].Title, id)
			totalSaved++
		}
	}

	totalDuration := time.Since(extractStart)

	// Write run summary
	summary := fmt.Sprintf(`# Extraction Summary
Run:             %s
PDF:             %s
Book:            %s
Model:           %s
Max tokens:      %d
Chunk size:      %d
Pages:           %d
Total chars:     %d
Chunks:          %d
Total duration:  %s

## Results
Recipes found:   %d
Saved to DB:     %d
Invalid:         %d
DB errors:       %d
`,
		runID, pdfPath, bookName, llmModel, llmMaxTokens, chunkSize,
		len(pages), totalChars, len(chunks), totalDuration,
		totalRecipes, totalSaved, totalInvalid, totalDBErrors)

	summaryPath := filepath.Join(outputDir, "summary.txt")
	if err := os.WriteFile(summaryPath, []byte(summary), 0644); err != nil {
		log.Printf("[output] WARNING: failed to write summary: %v", err)
	}

	log.Printf("[done] %s", summary)
	fmt.Printf("\nOutput saved to: %s\n", outputDir)
}

func saveExtractedText(outputDir string, pages []pdf.Page) {
	// Save full extracted text
	var buf []byte
	for _, p := range pages {
		buf = append(buf, []byte(fmt.Sprintf("=== Page %d ===\n%s\n\n", p.Number, p.Text))...)
	}
	path := filepath.Join(outputDir, "extracted_text.txt")
	if err := os.WriteFile(path, buf, 0644); err != nil {
		log.Printf("[output] WARNING: failed to save extracted text: %v", err)
	} else {
		log.Printf("[output] saved extracted PDF text to %s (%d bytes)", path, len(buf))
	}

	// Save page stats as JSON
	type pageStat struct {
		Page  int `json:"page"`
		Chars int `json:"chars"`
	}
	var stats []pageStat
	for _, p := range pages {
		stats = append(stats, pageStat{Page: p.Number, Chars: len(p.Text)})
	}
	statsJSON, _ := json.MarshalIndent(stats, "", "  ")
	statsPath := filepath.Join(outputDir, "page_stats.json")
	if err := os.WriteFile(statsPath, statsJSON, 0644); err != nil {
		log.Printf("[output] WARNING: failed to save page stats: %v", err)
	}
}

func requireEnv(key string) string {
	val := os.Getenv(key)
	if val == "" {
		log.Fatalf("Required environment variable %s is not set", key)
	}
	return val
}

func envOrDefault(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}

func envOrDefaultInt(key string, defaultVal int) int {
	if val := os.Getenv(key); val != "" {
		n, err := strconv.Atoi(val)
		if err != nil {
			log.Fatalf("Environment variable %s must be an integer, got: %s", key, val)
		}
		return n
	}
	return defaultVal
}

func envOrDefaultInt64(key string, defaultVal int64) int64 {
	if val := os.Getenv(key); val != "" {
		n, err := strconv.ParseInt(val, 10, 64)
		if err != nil {
			log.Fatalf("Environment variable %s must be an integer, got: %s", key, val)
		}
		return n
	}
	return defaultVal
}
