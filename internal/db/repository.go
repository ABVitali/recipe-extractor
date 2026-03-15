package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	_ "github.com/lib/pq"

	"github.com/ABVitali/recipe-extractor/internal/model"
)

type Repository struct {
	db *sql.DB
}

func New(databaseURL string) (*Repository, error) {
	db, err := sql.Open("postgres", databaseURL)
	if err != nil {
		return nil, fmt.Errorf("opening database: %w", err)
	}
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("connecting to database: %w", err)
	}
	return &Repository{db: db}, nil
}

func (r *Repository) Close() error {
	return r.db.Close()
}

func (r *Repository) InsertRecipe(ctx context.Context, recipe *model.Recipe) (int64, error) {
	ingredients, err := json.Marshal(recipe.Ingredients)
	if err != nil {
		return 0, fmt.Errorf("marshaling ingredients: %w", err)
	}
	preparation, err := json.Marshal(recipe.Preparation)
	if err != nil {
		return 0, fmt.Errorf("marshaling preparation: %w", err)
	}

	var id int64
	err = r.db.QueryRowContext(ctx, `
		INSERT INTO recipes (
			title, ingredients, preparation,
			prep_time, cook_time, total_time,
			servings, difficulty, category, cuisine,
			source_book, source_page, extracted_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)
		RETURNING id`,
		recipe.Title, ingredients, preparation,
		recipe.PrepTime, recipe.CookTime, recipe.TotalTime,
		recipe.Servings, recipe.Difficulty, recipe.Category, recipe.Cuisine,
		recipe.SourceBook, recipe.SourcePage, recipe.ExtractedAt,
	).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("inserting recipe: %w", err)
	}
	return id, nil
}

func (r *Repository) DB() *sql.DB {
	return r.db
}
