CREATE TABLE IF NOT EXISTS recipes (
    id           BIGSERIAL PRIMARY KEY,
    title        TEXT NOT NULL,
    ingredients  JSONB NOT NULL,
    preparation  JSONB NOT NULL,
    prep_time    TEXT,
    cook_time    TEXT,
    total_time   TEXT,
    servings     INT,
    difficulty   TEXT,
    category     TEXT,
    cuisine      TEXT,
    source_book  TEXT NOT NULL,
    source_page  INT,
    extracted_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_recipes_title ON recipes (title);
CREATE INDEX idx_recipes_category ON recipes (category);
CREATE INDEX idx_recipes_cuisine ON recipes (cuisine);
CREATE INDEX idx_recipes_source_book ON recipes (source_book);
