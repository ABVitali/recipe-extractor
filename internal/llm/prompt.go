package llm

import (
	"github.com/openai/openai-go"
	"github.com/openai/openai-go/shared"
)

// SystemPrompt is the system-level instruction sent to the LLM for recipe extraction.
var SystemPrompt = `You are a recipe extraction assistant. Given pages from a cookbook, extract ALL recipes found in the text.

Call the save_recipes tool with every recipe you find. Be precise with quantities and measurements.
Preserve the original language for recipe titles and ingredient names.
If no recipes are found, call save_recipes with an empty array.`

// RecipeSchema defines the JSON schema for the save_recipes tool.
// This forces the LLM to produce valid, structured output.
var RecipeSchema = shared.FunctionDefinitionParam{
	Name:        "save_recipes",
	Description: openai.String("Save extracted recipes to the database"),
	Parameters: shared.FunctionParameters{
		"type": "object",
		"properties": map[string]any{
			"recipes": map[string]any{
				"type": "array",
				"items": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"title": map[string]any{
							"type":        "string",
							"description": "Recipe name",
						},
						"ingredients": map[string]any{
							"type": "array",
							"items": map[string]any{
								"type": "object",
								"properties": map[string]any{
									"name":     map[string]any{"type": "string", "description": "Ingredient name"},
									"quantity": map[string]any{"type": "string", "description": "Amount (e.g. '500', '2', 'q.b.')"},
									"unit":     map[string]any{"type": "string", "description": "Unit of measure (e.g. 'g', 'ml', 'pieces')"},
									"notes":    map[string]any{"type": "string", "description": "Additional notes"},
								},
								"required": []string{"name", "quantity"},
							},
						},
						"preparation": map[string]any{
							"type":        "array",
							"items":       map[string]any{"type": "string"},
							"description": "Ordered preparation steps",
						},
						"prep_time":   map[string]any{"type": "string", "description": "Preparation time (e.g. '15 minutes')"},
						"cook_time":   map[string]any{"type": "string", "description": "Cooking time"},
						"total_time":  map[string]any{"type": "string", "description": "Total time"},
						"servings":    map[string]any{"type": "integer", "description": "Number of servings"},
						"difficulty":  map[string]any{"type": "string", "enum": []string{"easy", "medium", "hard"}},
						"category":    map[string]any{"type": "string", "description": "e.g. appetizer, main course, dessert, soup"},
						"cuisine":     map[string]any{"type": "string", "description": "e.g. Italian, French, Japanese"},
						"source_page": map[string]any{"type": "integer", "description": "Page number where the recipe starts"},
					},
					"required": []string{"title", "ingredients", "preparation"},
				},
			},
		},
		"required": []string{"recipes"},
	},
}
