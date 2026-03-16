package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"

	"github.com/ABVitali/recipe-extractor/internal/model"
	"github.com/ABVitali/recipe-extractor/internal/pdf"
)

// Extractor defines the interface for extracting recipes from PDF page chunks.
type Extractor interface {
	ExtractRecipes(ctx context.Context, chunk []pdf.Page, sourceBook string, chunkIndex int) (*ExtractionResult, error)
}

// Client is an LLM-backed implementation of the Extractor interface.
type Client struct {
	client    openai.Client
	model     string
	maxTokens int64
	outputDir string
}

// New creates a new LLM client.
func New(baseURL, apiKey, model string, maxTokens int64, outputDir string) *Client {
	client := openai.NewClient(
		option.WithBaseURL(baseURL),
		option.WithAPIKey(apiKey),
	)
	return &Client{
		client:    client,
		model:     model,
		maxTokens: maxTokens,
		outputDir: outputDir,
	}
}

// ExtractionResult holds the output of a single chunk extraction.
type ExtractionResult struct {
	Recipes      []model.Recipe
	FinishReason string
	InputChars   int
	OutputChars  int
	OutputTokens int64
	Duration     time.Duration
	Truncated    bool
}

func (c *Client) ExtractRecipes(ctx context.Context, chunk []pdf.Page, sourceBook string, chunkIndex int) (*ExtractionResult, error) {
	var textParts []string
	for _, p := range chunk {
		textParts = append(textParts, fmt.Sprintf("--- Page %d ---\n%s", p.Number, p.Text))
	}
	fullText := strings.Join(textParts, "\n\n")

	userMessage := fmt.Sprintf("Extract all recipes from these cookbook pages (from \"%s\"):\n\n%s", sourceBook, fullText)

	log.Printf("[llm] chunk=%d pages=%d-%d input_chars=%d model=%s max_tokens=%d",
		chunkIndex, chunk[0].Number, chunk[len(chunk)-1].Number, len(userMessage), c.model, c.maxTokens)

	start := time.Now()

	chat, err := c.client.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
		Model:     c.model,
		MaxTokens: openai.Int(c.maxTokens),
		Tools: []openai.ChatCompletionToolParam{
			{Function: RecipeSchema},
		},
		ToolChoice: openai.ChatCompletionToolChoiceOptionParamOfChatCompletionNamedToolChoice(
			openai.ChatCompletionNamedToolChoiceFunctionParam{Name: "save_recipes"},
		),
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.SystemMessage(SystemPrompt),
			openai.UserMessage(userMessage),
		},
	})
	duration := time.Since(start)

	if err != nil {
		log.Printf("[llm] chunk=%d ERROR after %s: %v", chunkIndex, duration, err)
		return nil, fmt.Errorf("calling LLM API: %w", err)
	}

	if len(chat.Choices) == 0 {
		log.Printf("[llm] chunk=%d ERROR: no choices returned after %s", chunkIndex, duration)
		return nil, fmt.Errorf("LLM returned no choices")
	}

	finishReason := string(chat.Choices[0].FinishReason)
	outputTokens := chat.Usage.CompletionTokens

	log.Printf("[llm] chunk=%d finish_reason=%s output_tokens=%d duration=%s",
		chunkIndex, finishReason, outputTokens, duration)

	if chat.Usage.TotalTokens > 0 {
		log.Printf("[llm] chunk=%d usage: prompt_tokens=%d completion_tokens=%d total_tokens=%d",
			chunkIndex, chat.Usage.PromptTokens, chat.Usage.CompletionTokens, chat.Usage.TotalTokens)
	}

	// Extract the tool call arguments
	toolCalls := chat.Choices[0].Message.ToolCalls
	if len(toolCalls) == 0 {
		log.Printf("[llm] chunk=%d WARNING: no tool calls in response (finish_reason=%s)", chunkIndex, finishReason)
		// Fall back to content if present (some proxies may not support tool calls properly)
		if chat.Choices[0].Message.Content != "" {
			log.Printf("[llm] chunk=%d falling back to message content", chunkIndex)
			return c.parseFromContent(chat.Choices[0].Message.Content, sourceBook, chunk, chunkIndex, finishReason, outputTokens, duration)
		}
		return nil, fmt.Errorf("LLM returned no tool calls and no content")
	}

	arguments := toolCalls[0].Function.Arguments
	outputChars := len(arguments)

	log.Printf("[llm] chunk=%d tool_call=%s arguments_chars=%d", chunkIndex, toolCalls[0].Function.Name, outputChars)

	// Save raw response
	c.saveRawResponse(chunkIndex, arguments, finishReason, duration, chat.Usage)

	truncated := finishReason == "length"
	if truncated {
		log.Printf("[llm] chunk=%d WARNING: response TRUNCATED (finish_reason=length). Output hit max_tokens=%d.", chunkIndex, c.maxTokens)
	}

	// Parse the tool call arguments
	var toolArgs struct {
		Recipes []model.Recipe `json:"recipes"`
	}
	if err := json.Unmarshal([]byte(arguments), &toolArgs); err != nil {
		c.saveFile(fmt.Sprintf("chunk_%03d_PARSE_ERROR.txt", chunkIndex),
			fmt.Sprintf("Error: %v\n\nRaw arguments:\n%s", err, arguments))
		return nil, fmt.Errorf("parsing tool call arguments: %w (finish_reason=%s, output_chars=%d, see %s)",
			err, finishReason, outputChars, c.outputDir)
	}

	log.Printf("[llm] chunk=%d parsed %d recipes from tool call", chunkIndex, len(toolArgs.Recipes))

	for i := range toolArgs.Recipes {
		toolArgs.Recipes[i].SourceBook = sourceBook
		if toolArgs.Recipes[i].SourcePage == 0 {
			toolArgs.Recipes[i].SourcePage = chunk[0].Number
		}
	}

	return &ExtractionResult{
		Recipes:      toolArgs.Recipes,
		FinishReason: finishReason,
		InputChars:   len(userMessage),
		OutputChars:  outputChars,
		OutputTokens: outputTokens,
		Duration:     duration,
		Truncated:    truncated,
	}, nil
}

// parseFromContent is a fallback for when tool calling doesn't work through the proxy.
func (c *Client) parseFromContent(content, sourceBook string, chunk []pdf.Page, chunkIndex int, finishReason string, outputTokens int64, duration time.Duration) (*ExtractionResult, error) {
	c.saveRawResponse(chunkIndex, content, finishReason, duration, openai.CompletionUsage{})

	cleaned := cleanJSON(content)
	if cleaned != content {
		c.saveFile(fmt.Sprintf("chunk_%03d_cleaned.json", chunkIndex), cleaned)
	}

	truncated := finishReason == "length"
	if truncated {
		log.Printf("[llm] chunk=%d WARNING: content response TRUNCATED", chunkIndex)
	}

	var recipes []model.Recipe
	if err := json.Unmarshal([]byte(cleaned), &recipes); err != nil {
		c.saveFile(fmt.Sprintf("chunk_%03d_PARSE_ERROR.txt", chunkIndex),
			fmt.Sprintf("Error: %v\n\nCleaned response:\n%s", err, cleaned))
		return nil, fmt.Errorf("parsing content as JSON: %w (finish_reason=%s, see %s)", err, finishReason, c.outputDir)
	}

	for i := range recipes {
		recipes[i].SourceBook = sourceBook
		if recipes[i].SourcePage == 0 {
			recipes[i].SourcePage = chunk[0].Number
		}
	}

	return &ExtractionResult{
		Recipes:      recipes,
		FinishReason: finishReason,
		InputChars:   0,
		OutputChars:  len(content),
		OutputTokens: outputTokens,
		Duration:     duration,
		Truncated:    truncated,
	}, nil
}

func (c *Client) saveRawResponse(chunkIndex int, response, finishReason string, duration time.Duration, usage openai.CompletionUsage) {
	header := fmt.Sprintf("# LLM Response - Chunk %d\n# Timestamp: %s\n# Model: %s\n# Duration: %s\n# Finish reason: %s\n# Prompt tokens: %d\n# Completion tokens: %d\n# Total tokens: %d\n# Response length (chars): %d\n\n",
		chunkIndex, time.Now().Format(time.RFC3339), c.model, duration,
		finishReason, usage.PromptTokens, usage.CompletionTokens, usage.TotalTokens, len(response))
	c.saveFile(fmt.Sprintf("chunk_%03d_raw.json", chunkIndex), header+response)
}

func (c *Client) saveFile(name, content string) {
	if c.outputDir == "" {
		return
	}
	path := filepath.Join(c.outputDir, name)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		log.Printf("[llm] WARNING: failed to save %s: %v", path, err)
	} else {
		log.Printf("[llm] saved %s (%d bytes)", path, len(content))
	}
}

func cleanJSON(s string) string {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "```json") {
		s = strings.TrimPrefix(s, "```json")
		s = strings.TrimSuffix(s, "```")
	} else if strings.HasPrefix(s, "```") {
		s = strings.TrimPrefix(s, "```")
		s = strings.TrimSuffix(s, "```")
	}
	return strings.TrimSpace(s)
}
