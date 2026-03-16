package db

import (
	"context"

	"github.com/ABVitali/recipe-extractor/internal/model"
)

// RecipeRepository defines the interface for persisting recipes.
type RecipeRepository interface {
	InsertRecipe(ctx context.Context, recipe *model.Recipe) (int64, error)
	Close() error
}
