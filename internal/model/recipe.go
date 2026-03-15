package model

import (
	"fmt"
	"strings"
	"time"
)

type Ingredient struct {
	Name     string `json:"name"`
	Quantity string `json:"quantity"`
	Unit     string `json:"unit,omitempty"`
	Notes    string `json:"notes,omitempty"`
}

type Recipe struct {
	ID           int64        `json:"id"`
	Title        string       `json:"title"`
	Ingredients  []Ingredient `json:"ingredients"`
	Preparation  []string     `json:"preparation"` // ordered steps
	PrepTime     string       `json:"prep_time,omitempty"`
	CookTime     string       `json:"cook_time,omitempty"`
	TotalTime    string       `json:"total_time,omitempty"`
	Servings     int          `json:"servings,omitempty"`
	Difficulty   string       `json:"difficulty,omitempty"`
	Category     string       `json:"category,omitempty"`
	Cuisine      string       `json:"cuisine,omitempty"`
	SourceBook   string       `json:"source_book"`
	SourcePage   int          `json:"source_page,omitempty"`
	ExtractedAt  time.Time    `json:"extracted_at"`
}

func (r *Recipe) Validate() error {
	var errs []string

	if strings.TrimSpace(r.Title) == "" {
		errs = append(errs, "title is required")
	}
	if len(r.Ingredients) == 0 {
		errs = append(errs, "at least one ingredient is required")
	}
	if len(r.Preparation) == 0 {
		errs = append(errs, "at least one preparation step is required")
	}
	for i, ing := range r.Ingredients {
		if strings.TrimSpace(ing.Name) == "" {
			errs = append(errs, fmt.Sprintf("ingredient %d: name is required", i+1))
		}
		if strings.TrimSpace(ing.Quantity) == "" {
			errs = append(errs, fmt.Sprintf("ingredient %d (%s): quantity is required", i+1, ing.Name))
		}
	}
	for i, step := range r.Preparation {
		if strings.TrimSpace(step) == "" {
			errs = append(errs, fmt.Sprintf("preparation step %d is empty", i+1))
		}
	}
	if r.Servings < 0 {
		errs = append(errs, "servings cannot be negative")
	}

	if len(errs) > 0 {
		return fmt.Errorf("validation failed:\n  - %s", strings.Join(errs, "\n  - "))
	}
	return nil
}
