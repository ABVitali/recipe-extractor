package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ABVitali/recipe-extractor/internal/pdf"
)

// --- cleanJSON tests ---

func TestCleanJSON_PlainJSON(t *testing.T) {
	input := `[{"title": "Test"}]`
	result := cleanJSON(input)
	if result != input {
		t.Fatalf("expected %q, got %q", input, result)
	}
}

func TestCleanJSON_WithJsonCodeFence(t *testing.T) {
	input := "```json\n[{\"title\": \"Test\"}]\n```"
	expected := `[{"title": "Test"}]`
	result := cleanJSON(input)
	if result != expected {
		t.Fatalf("expected %q, got %q", expected, result)
	}
}

func TestCleanJSON_WithPlainCodeFence(t *testing.T) {
	input := "```\n[{\"title\": \"Test\"}]\n```"
	expected := `[{"title": "Test"}]`
	result := cleanJSON(input)
	if result != expected {
		t.Fatalf("expected %q, got %q", expected, result)
	}
}

func TestCleanJSON_WithLeadingTrailingWhitespace(t *testing.T) {
	input := "  \n  [{\"title\": \"Test\"}]  \n  "
	expected := `[{"title": "Test"}]`
	result := cleanJSON(input)
	if result != expected {
		t.Fatalf("expected %q, got %q", expected, result)
	}
}

func TestCleanJSON_CodeFenceWithWhitespace(t *testing.T) {
	input := "  ```json\n[{\"title\": \"Test\"}]\n```  "
	expected := `[{"title": "Test"}]`
	result := cleanJSON(input)
	if result != expected {
		t.Fatalf("expected %q, got %q", expected, result)
	}
}

func TestCleanJSON_EmptyString(t *testing.T) {
	result := cleanJSON("")
	if result != "" {
		t.Fatalf("expected empty string, got %q", result)
	}
}

func TestCleanJSON_OnlyCodeFences(t *testing.T) {
	result := cleanJSON("```json\n```")
	if result != "" {
		t.Fatalf("expected empty string, got %q", result)
	}
}

func TestCleanJSON_EmptyArray(t *testing.T) {
	result := cleanJSON("```json\n[]\n```")
	if result != "[]" {
		t.Fatalf("expected %q, got %q", "[]", result)
	}
}

func TestCleanJSON_NestedCodeFenceMarkers(t *testing.T) {
	// Only outermost should be stripped
	input := "```json\n{\"code\": \"some value\"}\n```"
	expected := `{"code": "some value"}`
	result := cleanJSON(input)
	if result != expected {
		t.Fatalf("expected %q, got %q", expected, result)
	}
}

// --- ExtractRecipes tests with httptest ---

// chatResponse builds a minimal OpenAI-compatible chat completion JSON response.
func chatResponse(content string) string {
	return fmt.Sprintf(`{
		"id": "chatcmpl-test",
		"object": "chat.completion",
		"created": 1700000000,
		"model": "test-model",
		"choices": [{
			"index": 0,
			"message": {
				"role": "assistant",
				"content": %s
			},
			"finish_reason": "stop"
		}],
		"usage": {"prompt_tokens": 10, "completion_tokens": 20, "total_tokens": 30}
	}`, content)
}

func testChunk() []pdf.Page {
	return []pdf.Page{
		{Number: 1, Text: "Recipe: Pasta\nIngredients: spaghetti, eggs\nSteps: boil, mix"},
	}
}

func TestExtractRecipes_Success(t *testing.T) {
	recipesJSON := `[{"title":"Pasta Carbonara","ingredients":[{"name":"Spaghetti","quantity":"400","unit":"g"}],"preparation":["Boil water","Cook pasta"],"servings":4,"source_page":1}]`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if !strings.HasSuffix(r.URL.Path, "/chat/completions") {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}

		body, _ := io.ReadAll(r.Body)
		var req map[string]interface{}
		json.Unmarshal(body, &req)
		if req["model"] != "test-model" {
			t.Errorf("expected model 'test-model', got %v", req["model"])
		}

		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, chatResponse(jsonString(recipesJSON)))
	}))
	defer server.Close()

	client := New(server.URL+"/v1", "test-key", "test-model")
	recipes, err := client.ExtractRecipes(context.Background(), testChunk(), "My Cookbook")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(recipes) != 1 {
		t.Fatalf("expected 1 recipe, got %d", len(recipes))
	}
	if recipes[0].Title != "Pasta Carbonara" {
		t.Errorf("expected title 'Pasta Carbonara', got %q", recipes[0].Title)
	}
	if recipes[0].SourceBook != "My Cookbook" {
		t.Errorf("expected source book 'My Cookbook', got %q", recipes[0].SourceBook)
	}
	if recipes[0].SourcePage != 1 {
		t.Errorf("expected source page 1, got %d", recipes[0].SourcePage)
	}
	if len(recipes[0].Ingredients) != 1 {
		t.Errorf("expected 1 ingredient, got %d", len(recipes[0].Ingredients))
	}
	if recipes[0].Servings != 4 {
		t.Errorf("expected servings 4, got %d", recipes[0].Servings)
	}
}

func TestExtractRecipes_EmptyResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, chatResponse(jsonString("[]")))
	}))
	defer server.Close()

	client := New(server.URL+"/v1", "test-key", "test-model")
	recipes, err := client.ExtractRecipes(context.Background(), testChunk(), "My Cookbook")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(recipes) != 0 {
		t.Fatalf("expected 0 recipes, got %d", len(recipes))
	}
}

func TestExtractRecipes_MalformedJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, chatResponse(jsonString("this is not json at all")))
	}))
	defer server.Close()

	client := New(server.URL+"/v1", "test-key", "test-model")
	_, err := client.ExtractRecipes(context.Background(), testChunk(), "My Cookbook")
	if err == nil {
		t.Fatal("expected error for malformed JSON response")
	}
	if !strings.Contains(err.Error(), "parsing LLM response as JSON") {
		t.Fatalf("expected JSON parse error, got: %v", err)
	}
}

func TestExtractRecipes_APIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, `{"error": {"message": "server error", "type": "server_error"}}`)
	}))
	defer server.Close()

	client := New(server.URL+"/v1", "test-key", "test-model")
	_, err := client.ExtractRecipes(context.Background(), testChunk(), "My Cookbook")
	if err == nil {
		t.Fatal("expected error for API failure")
	}
	if !strings.Contains(err.Error(), "calling LLM API") {
		t.Fatalf("expected LLM API error, got: %v", err)
	}
}

func TestExtractRecipes_NoChoices(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{
			"id": "chatcmpl-test",
			"object": "chat.completion",
			"created": 1700000000,
			"model": "test-model",
			"choices": [],
			"usage": {"prompt_tokens": 10, "completion_tokens": 0, "total_tokens": 10}
		}`)
	}))
	defer server.Close()

	client := New(server.URL+"/v1", "test-key", "test-model")
	_, err := client.ExtractRecipes(context.Background(), testChunk(), "My Cookbook")
	if err == nil {
		t.Fatal("expected error for no choices")
	}
	if !strings.Contains(err.Error(), "LLM returned no choices") {
		t.Fatalf("expected 'no choices' error, got: %v", err)
	}
}

func TestExtractRecipes_ResponseWithCodeFences(t *testing.T) {
	recipesJSON := `[{"title":"Test Recipe","ingredients":[{"name":"Flour","quantity":"200","unit":"g"}],"preparation":["Mix"]}]`
	wrappedJSON := "```json\n" + recipesJSON + "\n```"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, chatResponse(jsonString(wrappedJSON)))
	}))
	defer server.Close()

	client := New(server.URL+"/v1", "test-key", "test-model")
	recipes, err := client.ExtractRecipes(context.Background(), testChunk(), "My Cookbook")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(recipes) != 1 {
		t.Fatalf("expected 1 recipe, got %d", len(recipes))
	}
	if recipes[0].Title != "Test Recipe" {
		t.Errorf("expected title 'Test Recipe', got %q", recipes[0].Title)
	}
}

func TestExtractRecipes_SetsSourceBookOnAllRecipes(t *testing.T) {
	recipesJSON := `[
		{"title":"Recipe A","ingredients":[{"name":"A","quantity":"1"}],"preparation":["Do A"]},
		{"title":"Recipe B","ingredients":[{"name":"B","quantity":"2"}],"preparation":["Do B"],"source_page":5}
	]`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, chatResponse(jsonString(recipesJSON)))
	}))
	defer server.Close()

	chunk := []pdf.Page{
		{Number: 3, Text: "text from page 3"},
		{Number: 4, Text: "text from page 4"},
	}

	client := New(server.URL+"/v1", "test-key", "test-model")
	recipes, err := client.ExtractRecipes(context.Background(), chunk, "Great Cookbook")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(recipes) != 2 {
		t.Fatalf("expected 2 recipes, got %d", len(recipes))
	}
	// Both should have SourceBook set
	for i, r := range recipes {
		if r.SourceBook != "Great Cookbook" {
			t.Errorf("recipe %d: expected source book 'Great Cookbook', got %q", i, r.SourceBook)
		}
	}
	// Recipe A has no source_page, should default to startPage (3)
	if recipes[0].SourcePage != 3 {
		t.Errorf("recipe 0: expected source page 3 (default from chunk start), got %d", recipes[0].SourcePage)
	}
	// Recipe B has source_page=5, should keep it
	if recipes[1].SourcePage != 5 {
		t.Errorf("recipe 1: expected source page 5 (from LLM response), got %d", recipes[1].SourcePage)
	}
}

func TestExtractRecipes_ContextCancelled(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Respond normally, but context should be cancelled before we get here
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, chatResponse(jsonString("[]")))
	}))
	defer server.Close()

	client := New(server.URL+"/v1", "test-key", "test-model")
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	_, err := client.ExtractRecipes(ctx, testChunk(), "My Cookbook")
	if err == nil {
		t.Fatal("expected error for cancelled context")
	}
}

func TestNew_ReturnsClient(t *testing.T) {
	client := New("http://localhost:1234/v1", "key", "model")
	if client == nil {
		t.Fatal("expected non-nil client")
	}
	if client.model != "model" {
		t.Errorf("expected model 'model', got %q", client.model)
	}
}

// jsonString returns a JSON-encoded string value (with quotes and escaping).
func jsonString(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}
