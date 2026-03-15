package pdf

import (
	"os"
	"path/filepath"
	"testing"
)

// --- ChunkPages tests ---

func TestChunkPages_EvenSplit(t *testing.T) {
	pages := []Page{
		{Number: 1, Text: "a"},
		{Number: 2, Text: "b"},
		{Number: 3, Text: "c"},
		{Number: 4, Text: "d"},
	}
	chunks := ChunkPages(pages, 2)
	if len(chunks) != 2 {
		t.Fatalf("expected 2 chunks, got %d", len(chunks))
	}
	if len(chunks[0]) != 2 || len(chunks[1]) != 2 {
		t.Fatalf("expected chunks of size 2, got %d and %d", len(chunks[0]), len(chunks[1]))
	}
	if chunks[0][0].Number != 1 || chunks[0][1].Number != 2 {
		t.Error("first chunk has wrong pages")
	}
	if chunks[1][0].Number != 3 || chunks[1][1].Number != 4 {
		t.Error("second chunk has wrong pages")
	}
}

func TestChunkPages_UnevenSplit(t *testing.T) {
	pages := []Page{
		{Number: 1, Text: "a"},
		{Number: 2, Text: "b"},
		{Number: 3, Text: "c"},
		{Number: 4, Text: "d"},
		{Number: 5, Text: "e"},
	}
	chunks := ChunkPages(pages, 2)
	if len(chunks) != 3 {
		t.Fatalf("expected 3 chunks, got %d", len(chunks))
	}
	if len(chunks[0]) != 2 || len(chunks[1]) != 2 || len(chunks[2]) != 1 {
		t.Fatalf("expected chunk sizes 2,2,1 got %d,%d,%d", len(chunks[0]), len(chunks[1]), len(chunks[2]))
	}
	if chunks[2][0].Number != 5 {
		t.Error("last chunk has wrong page")
	}
}

func TestChunkPages_ChunkSizeLargerThanPages(t *testing.T) {
	pages := []Page{
		{Number: 1, Text: "a"},
		{Number: 2, Text: "b"},
	}
	chunks := ChunkPages(pages, 10)
	if len(chunks) != 1 {
		t.Fatalf("expected 1 chunk, got %d", len(chunks))
	}
	if len(chunks[0]) != 2 {
		t.Fatalf("expected chunk of size 2, got %d", len(chunks[0]))
	}
}

func TestChunkPages_ChunkSizeOne(t *testing.T) {
	pages := []Page{
		{Number: 1, Text: "a"},
		{Number: 2, Text: "b"},
		{Number: 3, Text: "c"},
	}
	chunks := ChunkPages(pages, 1)
	if len(chunks) != 3 {
		t.Fatalf("expected 3 chunks, got %d", len(chunks))
	}
	for i, ch := range chunks {
		if len(ch) != 1 {
			t.Errorf("chunk %d: expected size 1, got %d", i, len(ch))
		}
		if ch[0].Number != i+1 {
			t.Errorf("chunk %d: expected page %d, got %d", i, i+1, ch[0].Number)
		}
	}
}

func TestChunkPages_EmptyPages(t *testing.T) {
	chunks := ChunkPages(nil, 5)
	if len(chunks) != 0 {
		t.Fatalf("expected 0 chunks for nil input, got %d", len(chunks))
	}

	chunks = ChunkPages([]Page{}, 5)
	if len(chunks) != 0 {
		t.Fatalf("expected 0 chunks for empty input, got %d", len(chunks))
	}
}

func TestChunkPages_SinglePage(t *testing.T) {
	pages := []Page{{Number: 1, Text: "only"}}
	chunks := ChunkPages(pages, 3)
	if len(chunks) != 1 {
		t.Fatalf("expected 1 chunk, got %d", len(chunks))
	}
	if len(chunks[0]) != 1 {
		t.Fatalf("expected chunk of size 1, got %d", len(chunks[0]))
	}
}

func TestChunkPages_ChunkSizeEqualsLength(t *testing.T) {
	pages := []Page{
		{Number: 1, Text: "a"},
		{Number: 2, Text: "b"},
		{Number: 3, Text: "c"},
	}
	chunks := ChunkPages(pages, 3)
	if len(chunks) != 1 {
		t.Fatalf("expected 1 chunk, got %d", len(chunks))
	}
	if len(chunks[0]) != 3 {
		t.Fatalf("expected chunk of size 3, got %d", len(chunks[0]))
	}
}

func TestChunkPages_PreservesPageData(t *testing.T) {
	pages := []Page{
		{Number: 10, Text: "recipe text here"},
		{Number: 11, Text: "more text"},
	}
	chunks := ChunkPages(pages, 5)
	if chunks[0][0].Number != 10 || chunks[0][0].Text != "recipe text here" {
		t.Error("chunk did not preserve page data")
	}
	if chunks[0][1].Number != 11 || chunks[0][1].Text != "more text" {
		t.Error("chunk did not preserve page data")
	}
}

// --- ValidatePath tests ---

func TestValidatePath_ValidFile(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "test-*.txt")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	if err := ValidatePath(tmpFile.Name()); err != nil {
		t.Fatalf("expected no error for valid file, got: %v", err)
	}
}

func TestValidatePath_NonExistentFile(t *testing.T) {
	err := ValidatePath("/tmp/definitely-does-not-exist-abc123xyz.pdf")
	if err == nil {
		t.Fatal("expected error for non-existent file")
	}
	if expected := "file not found:"; !contains(err.Error(), expected) {
		t.Fatalf("expected error to contain %q, got: %v", expected, err)
	}
}

func TestValidatePath_Directory(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "test-dir-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	err = ValidatePath(tmpDir)
	if err == nil {
		t.Fatal("expected error for directory path")
	}
	if expected := "path is a directory, not a file:"; !contains(err.Error(), expected) {
		t.Fatalf("expected error to contain %q, got: %v", expected, err)
	}
}

func TestValidatePath_NestedNonExistent(t *testing.T) {
	err := ValidatePath(filepath.Join("/tmp", "nonexistent-dir-xyz", "file.pdf"))
	if err == nil {
		t.Fatal("expected error for file in non-existent directory")
	}
}

// --- ExtractPages tests ---

func TestExtractPages_NonExistentFile(t *testing.T) {
	_, err := ExtractPages("/tmp/nonexistent-file-abc123.pdf")
	if err == nil {
		t.Fatal("expected error for non-existent PDF file")
	}
}

func TestExtractPages_NotAPDF(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "test-*.txt")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.WriteString("this is not a PDF")
	tmpFile.Close()

	_, err = ExtractPages(tmpFile.Name())
	if err == nil {
		t.Fatal("expected error for non-PDF file")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
