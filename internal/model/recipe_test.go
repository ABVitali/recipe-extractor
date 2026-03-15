package model

import (
	"strings"
	"testing"
	"time"
)

func validRecipe() *Recipe {
	return &Recipe{
		Title: "Pasta Carbonara",
		Ingredients: []Ingredient{
			{Name: "Spaghetti", Quantity: "400", Unit: "g"},
			{Name: "Guanciale", Quantity: "200", Unit: "g"},
			{Name: "Eggs", Quantity: "4"},
		},
		Preparation: []string{
			"Boil water and cook spaghetti.",
			"Cook guanciale until crispy.",
			"Mix eggs with cheese.",
			"Combine everything.",
		},
		Servings:   4,
		SourceBook: "Italian Classics",
		SourcePage: 42,
		ExtractedAt: time.Now(),
	}
}

func TestValidate_ValidRecipe(t *testing.T) {
	r := validRecipe()
	if err := r.Validate(); err != nil {
		t.Fatalf("expected no error for valid recipe, got: %v", err)
	}
}

func TestValidate_MissingTitle(t *testing.T) {
	r := validRecipe()
	r.Title = ""
	err := r.Validate()
	if err == nil {
		t.Fatal("expected error for missing title")
	}
	if !strings.Contains(err.Error(), "title is required") {
		t.Fatalf("expected 'title is required' in error, got: %v", err)
	}
}

func TestValidate_WhitespaceOnlyTitle(t *testing.T) {
	r := validRecipe()
	r.Title = "   \t\n  "
	err := r.Validate()
	if err == nil {
		t.Fatal("expected error for whitespace-only title")
	}
	if !strings.Contains(err.Error(), "title is required") {
		t.Fatalf("expected 'title is required' in error, got: %v", err)
	}
}

func TestValidate_MissingIngredients(t *testing.T) {
	r := validRecipe()
	r.Ingredients = nil
	err := r.Validate()
	if err == nil {
		t.Fatal("expected error for missing ingredients")
	}
	if !strings.Contains(err.Error(), "at least one ingredient is required") {
		t.Fatalf("expected ingredient error, got: %v", err)
	}
}

func TestValidate_EmptyIngredients(t *testing.T) {
	r := validRecipe()
	r.Ingredients = []Ingredient{}
	err := r.Validate()
	if err == nil {
		t.Fatal("expected error for empty ingredients slice")
	}
	if !strings.Contains(err.Error(), "at least one ingredient is required") {
		t.Fatalf("expected ingredient error, got: %v", err)
	}
}

func TestValidate_MissingPreparation(t *testing.T) {
	r := validRecipe()
	r.Preparation = nil
	err := r.Validate()
	if err == nil {
		t.Fatal("expected error for missing preparation")
	}
	if !strings.Contains(err.Error(), "at least one preparation step is required") {
		t.Fatalf("expected preparation error, got: %v", err)
	}
}

func TestValidate_EmptyPreparation(t *testing.T) {
	r := validRecipe()
	r.Preparation = []string{}
	err := r.Validate()
	if err == nil {
		t.Fatal("expected error for empty preparation")
	}
	if !strings.Contains(err.Error(), "at least one preparation step is required") {
		t.Fatalf("expected preparation error, got: %v", err)
	}
}

func TestValidate_IngredientMissingName(t *testing.T) {
	r := validRecipe()
	r.Ingredients = []Ingredient{
		{Name: "", Quantity: "100", Unit: "g"},
	}
	err := r.Validate()
	if err == nil {
		t.Fatal("expected error for ingredient with missing name")
	}
	if !strings.Contains(err.Error(), "ingredient 1: name is required") {
		t.Fatalf("expected ingredient name error, got: %v", err)
	}
}

func TestValidate_IngredientWhitespaceOnlyName(t *testing.T) {
	r := validRecipe()
	r.Ingredients = []Ingredient{
		{Name: "  ", Quantity: "100", Unit: "g"},
	}
	err := r.Validate()
	if err == nil {
		t.Fatal("expected error for ingredient with whitespace-only name")
	}
	if !strings.Contains(err.Error(), "ingredient 1: name is required") {
		t.Fatalf("expected ingredient name error, got: %v", err)
	}
}

func TestValidate_IngredientMissingQuantity(t *testing.T) {
	r := validRecipe()
	r.Ingredients = []Ingredient{
		{Name: "Flour", Quantity: ""},
	}
	err := r.Validate()
	if err == nil {
		t.Fatal("expected error for ingredient with missing quantity")
	}
	if !strings.Contains(err.Error(), "quantity is required") {
		t.Fatalf("expected ingredient quantity error, got: %v", err)
	}
}

func TestValidate_IngredientWhitespaceOnlyQuantity(t *testing.T) {
	r := validRecipe()
	r.Ingredients = []Ingredient{
		{Name: "Flour", Quantity: "  \t "},
	}
	err := r.Validate()
	if err == nil {
		t.Fatal("expected error for ingredient with whitespace-only quantity")
	}
	if !strings.Contains(err.Error(), "quantity is required") {
		t.Fatalf("expected ingredient quantity error, got: %v", err)
	}
}

func TestValidate_EmptyPreparationStep(t *testing.T) {
	r := validRecipe()
	r.Preparation = []string{"Step one.", "", "Step three."}
	err := r.Validate()
	if err == nil {
		t.Fatal("expected error for empty preparation step")
	}
	if !strings.Contains(err.Error(), "preparation step 2 is empty") {
		t.Fatalf("expected step 2 empty error, got: %v", err)
	}
}

func TestValidate_WhitespaceOnlyPreparationStep(t *testing.T) {
	r := validRecipe()
	r.Preparation = []string{"Step one.", "  \t  "}
	err := r.Validate()
	if err == nil {
		t.Fatal("expected error for whitespace-only preparation step")
	}
	if !strings.Contains(err.Error(), "preparation step 2 is empty") {
		t.Fatalf("expected step 2 empty error, got: %v", err)
	}
}

func TestValidate_NegativeServings(t *testing.T) {
	r := validRecipe()
	r.Servings = -1
	err := r.Validate()
	if err == nil {
		t.Fatal("expected error for negative servings")
	}
	if !strings.Contains(err.Error(), "servings cannot be negative") {
		t.Fatalf("expected servings error, got: %v", err)
	}
}

func TestValidate_ZeroServingsIsValid(t *testing.T) {
	r := validRecipe()
	r.Servings = 0
	if err := r.Validate(); err != nil {
		t.Fatalf("expected no error for zero servings (omitted), got: %v", err)
	}
}

func TestValidate_MultipleErrors(t *testing.T) {
	r := &Recipe{
		Title:       "",
		Ingredients: nil,
		Preparation: nil,
		Servings:    -5,
	}
	err := r.Validate()
	if err == nil {
		t.Fatal("expected error for multiple issues")
	}
	msg := err.Error()
	if !strings.Contains(msg, "title is required") {
		t.Error("missing 'title is required' in error")
	}
	if !strings.Contains(msg, "at least one ingredient is required") {
		t.Error("missing ingredient error")
	}
	if !strings.Contains(msg, "at least one preparation step is required") {
		t.Error("missing preparation error")
	}
	if !strings.Contains(msg, "servings cannot be negative") {
		t.Error("missing servings error")
	}
}

func TestValidate_MultipleIngredientErrors(t *testing.T) {
	r := validRecipe()
	r.Ingredients = []Ingredient{
		{Name: "Flour", Quantity: "200", Unit: "g"},
		{Name: "", Quantity: ""},
		{Name: "Sugar", Quantity: ""},
	}
	err := r.Validate()
	if err == nil {
		t.Fatal("expected error for multiple ingredient issues")
	}
	msg := err.Error()
	if !strings.Contains(msg, "ingredient 2: name is required") {
		t.Errorf("expected ingredient 2 name error, got: %v", msg)
	}
	if !strings.Contains(msg, "ingredient 2 (): quantity is required") {
		t.Errorf("expected ingredient 2 quantity error, got: %v", msg)
	}
	if !strings.Contains(msg, "ingredient 3 (Sugar): quantity is required") {
		t.Errorf("expected ingredient 3 quantity error, got: %v", msg)
	}
}

func TestValidate_OptionalFieldsCanBeEmpty(t *testing.T) {
	r := validRecipe()
	r.PrepTime = ""
	r.CookTime = ""
	r.TotalTime = ""
	r.Difficulty = ""
	r.Category = ""
	r.Cuisine = ""
	r.SourceBook = ""
	r.SourcePage = 0
	if err := r.Validate(); err != nil {
		t.Fatalf("expected no error when optional fields are empty, got: %v", err)
	}
}

func TestValidate_ErrorFormat(t *testing.T) {
	r := &Recipe{
		Title:       "",
		Ingredients: []Ingredient{{Name: "a", Quantity: "1"}},
		Preparation: []string{"step"},
	}
	err := r.Validate()
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.HasPrefix(err.Error(), "validation failed:\n  - ") {
		t.Fatalf("unexpected error format: %v", err)
	}
}
