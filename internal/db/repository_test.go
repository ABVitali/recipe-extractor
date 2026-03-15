package db

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/ABVitali/recipe-extractor/internal/model"
)

func skipIfNoDatabase(t *testing.T) string {
	t.Helper()
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		t.Skip("DATABASE_URL not set, skipping database tests")
	}
	return dbURL
}

func TestNew_InvalidURL(t *testing.T) {
	_, err := New("postgres://invalid:invalid@localhost:1/nonexistent?sslmode=disable&connect_timeout=1")
	if err == nil {
		t.Fatal("expected error for invalid database URL")
	}
}

func TestNew_Success(t *testing.T) {
	dbURL := skipIfNoDatabase(t)
	repo, err := New(dbURL)
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer repo.Close()
}

func TestClose(t *testing.T) {
	dbURL := skipIfNoDatabase(t)
	repo, err := New(dbURL)
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	if err := repo.Close(); err != nil {
		t.Fatalf("failed to close: %v", err)
	}
	// After closing, the DB should not be usable
	if err := repo.db.Ping(); err == nil {
		t.Fatal("expected error after closing, but ping succeeded")
	}
}

func TestDB_ReturnsUnderlyingDB(t *testing.T) {
	dbURL := skipIfNoDatabase(t)
	repo, err := New(dbURL)
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer repo.Close()

	db := repo.DB()
	if db == nil {
		t.Fatal("expected non-nil *sql.DB")
	}
	if err := db.Ping(); err != nil {
		t.Fatalf("ping via DB() failed: %v", err)
	}
}

func TestInsertRecipe_Success(t *testing.T) {
	dbURL := skipIfNoDatabase(t)
	repo, err := New(dbURL)
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer repo.Close()

	// Run migrations to ensure table exists
	migrationsPath := "../../migrations"
	if _, err := os.Stat(migrationsPath); os.IsNotExist(err) {
		t.Skip("migrations directory not found, skipping insert test")
	}
	if err := repo.MigrateUp(migrationsPath); err != nil {
		t.Fatalf("failed to run migrations: %v", err)
	}

	recipe := &model.Recipe{
		Title: "Test Recipe " + time.Now().Format(time.RFC3339Nano),
		Ingredients: []model.Ingredient{
			{Name: "Flour", Quantity: "200", Unit: "g"},
			{Name: "Sugar", Quantity: "100", Unit: "g"},
		},
		Preparation: []string{"Mix ingredients", "Bake at 180C"},
		PrepTime:    "10 minutes",
		CookTime:    "30 minutes",
		TotalTime:   "40 minutes",
		Servings:    4,
		Difficulty:  "easy",
		Category:    "dessert",
		Cuisine:     "Italian",
		SourceBook:  "Test Cookbook",
		SourcePage:  42,
		ExtractedAt: time.Now(),
	}

	id, err := repo.InsertRecipe(context.Background(), recipe)
	if err != nil {
		t.Fatalf("failed to insert recipe: %v", err)
	}
	if id <= 0 {
		t.Fatalf("expected positive ID, got %d", id)
	}

	// Clean up: delete the test recipe
	_, _ = repo.db.Exec("DELETE FROM recipes WHERE id = $1", id)
}

func TestInsertRecipe_MinimalFields(t *testing.T) {
	dbURL := skipIfNoDatabase(t)
	repo, err := New(dbURL)
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer repo.Close()

	migrationsPath := "../../migrations"
	if _, err := os.Stat(migrationsPath); os.IsNotExist(err) {
		t.Skip("migrations directory not found, skipping insert test")
	}
	if err := repo.MigrateUp(migrationsPath); err != nil {
		t.Fatalf("failed to run migrations: %v", err)
	}

	recipe := &model.Recipe{
		Title:       "Minimal Test Recipe " + time.Now().Format(time.RFC3339Nano),
		Ingredients: []model.Ingredient{{Name: "Water", Quantity: "1", Unit: "cup"}},
		Preparation: []string{"Boil"},
		SourceBook:  "Test",
		ExtractedAt: time.Now(),
	}

	id, err := repo.InsertRecipe(context.Background(), recipe)
	if err != nil {
		t.Fatalf("failed to insert minimal recipe: %v", err)
	}
	if id <= 0 {
		t.Fatalf("expected positive ID, got %d", id)
	}

	// Clean up
	_, _ = repo.db.Exec("DELETE FROM recipes WHERE id = $1", id)
}
