package pdf

import (
	"fmt"
	"os"

	"github.com/ledongthuc/pdf"
)

type Page struct {
	Number int
	Text   string
}

func ExtractPages(filePath string) ([]Page, error) {
	f, r, err := pdf.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("opening PDF %s: %w", filePath, err)
	}
	defer f.Close()

	totalPages := r.NumPage()
	if totalPages == 0 {
		return nil, fmt.Errorf("PDF %s has no pages", filePath)
	}

	var pages []Page
	for i := 1; i <= totalPages; i++ {
		p := r.Page(i)
		if p.V.IsNull() {
			continue
		}
		text, err := p.GetPlainText(nil)
		if err != nil {
			return nil, fmt.Errorf("extracting text from page %d: %w", i, err)
		}
		pages = append(pages, Page{Number: i, Text: text})
	}
	return pages, nil
}

func ChunkPages(pages []Page, chunkSize int) [][]Page {
	var chunks [][]Page
	for i := 0; i < len(pages); i += chunkSize {
		end := i + chunkSize
		if end > len(pages) {
			end = len(pages)
		}
		chunks = append(chunks, pages[i:end])
	}
	return chunks
}

func ValidatePath(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("file not found: %s", path)
	}
	if info.IsDir() {
		return fmt.Errorf("path is a directory, not a file: %s", path)
	}
	return nil
}
