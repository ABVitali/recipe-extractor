package extract

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/ABVitali/recipe-extractor/internal/config"
	"github.com/ABVitali/recipe-extractor/internal/db"
	"github.com/ABVitali/recipe-extractor/internal/llm"
	"github.com/ABVitali/recipe-extractor/internal/pdf"
)

// Orchestrator coordinates the extraction pipeline: PDF -> chunks -> LLM -> validate -> DB.
type Orchestrator struct {
	llm  llm.Extractor
	repo db.RecipeRepository
	cfg  *config.Config
}

// New creates an Orchestrator with the given dependencies.
func New(extractor llm.Extractor, repo db.RecipeRepository, cfg *config.Config) *Orchestrator {
	return &Orchestrator{
		llm:  extractor,
		repo: repo,
		cfg:  cfg,
	}
}

// RunResult summarizes the extraction run.
type RunResult struct {
	RunID         string
	OutputDir     string
	TotalPages    int
	TotalChars    int
	TotalChunks   int
	TotalRecipes  int
	TotalSaved    int
	TotalInvalid  int
	TotalDBErrors int
	Duration      time.Duration
}

// Run performs the full extraction pipeline for a given PDF and book name.
func (o *Orchestrator) Run(ctx context.Context, pdfPath, bookName string) (*RunResult, error) {
	// Create output directory for this run
	runID := time.Now().Format("2006-01-02_15-04-05")
	outputDir := filepath.Join("output", runID)
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return nil, fmt.Errorf("creating output directory: %w", err)
	}

	log.Printf("[config] pdf=%s book=%q", pdfPath, bookName)
	log.Printf("[config] model=%s max_tokens=%d chunk_size=%d", o.cfg.LLMModel, o.cfg.LLMMaxTokens, o.cfg.MaxPagesPerChunk)
	log.Printf("[config] output_dir=%s", outputDir)

	// Extract text from PDF
	log.Printf("[pdf] extracting text from %s...", pdfPath)
	pdfStart := time.Now()
	pages, err := pdf.ExtractPages(pdfPath)
	if err != nil {
		return nil, fmt.Errorf("extracting PDF: %w", err)
	}

	totalChars := 0
	for _, p := range pages {
		totalChars += len(p.Text)
	}
	log.Printf("[pdf] extracted %d pages (%d total chars) in %s", len(pages), totalChars, time.Since(pdfStart))

	// Save extracted text for reference
	saveExtractedText(outputDir, pages)

	// Chunk pages for LLM processing (0 = send all pages at once)
	chunkSize := o.cfg.MaxPagesPerChunk
	var chunks [][]pdf.Page
	if chunkSize <= 0 {
		chunks = [][]pdf.Page{pages}
		log.Printf("[chunk] sending all %d pages in a single request", len(pages))
	} else {
		chunks = pdf.ChunkPages(pages, chunkSize)
		log.Printf("[chunk] split into %d chunks of up to %d pages each", len(chunks), chunkSize)
	}

	// Process each chunk
	var totalRecipes, totalSaved, totalInvalid, totalDBErrors int
	extractStart := time.Now()

	for i, chunk := range chunks {
		log.Printf("[extract] --- chunk %d/%d (pages %d-%d) ---",
			i+1, len(chunks), chunk[0].Number, chunk[len(chunk)-1].Number)

		result, err := o.llm.ExtractRecipes(ctx, chunk, bookName, i+1)
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

			id, err := o.repo.InsertRecipe(ctx, &result.Recipes[j])
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

	res := &RunResult{
		RunID:         runID,
		OutputDir:     outputDir,
		TotalPages:    len(pages),
		TotalChars:    totalChars,
		TotalChunks:   len(chunks),
		TotalRecipes:  totalRecipes,
		TotalSaved:    totalSaved,
		TotalInvalid:  totalInvalid,
		TotalDBErrors: totalDBErrors,
		Duration:      totalDuration,
	}

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
		runID, pdfPath, bookName, o.cfg.LLMModel, o.cfg.LLMMaxTokens, chunkSize,
		len(pages), totalChars, len(chunks), totalDuration,
		totalRecipes, totalSaved, totalInvalid, totalDBErrors)

	summaryPath := filepath.Join(outputDir, "summary.txt")
	if err := os.WriteFile(summaryPath, []byte(summary), 0644); err != nil {
		log.Printf("[output] WARNING: failed to write summary: %v", err)
	}

	log.Printf("[done] %s", summary)

	return res, nil
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
