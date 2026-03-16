#!/bin/bash
# Compare recipes extracted by two different models
# Usage: ./scripts/compare.sh

DB_USER="alessandro"
DB_PASS="postgres"
DB_HOST="localhost"
DB_PORT="5432"
DB1="recipes"
DB2="recipes_sonnet"

OUTDIR="output/comparison"
mkdir -p "$OUTDIR"

QUERY="SELECT json_agg(row_to_json(r) ORDER BY r.title) FROM (
  SELECT id, title, ingredients, preparation, prep_time, cook_time, total_time,
         servings, difficulty, category, cuisine, source_book, source_page
  FROM recipes ORDER BY title
) r"

echo "Exporting $DB1..."
podman exec local-postgres psql -U $DB_USER -d $DB1 -t -A -c "$QUERY" | jq '.' > "$OUTDIR/opus.json"

echo "Exporting $DB2..."
podman exec local-postgres psql -U $DB_USER -d $DB2 -t -A -c "$QUERY" | jq '.' > "$OUTDIR/sonnet.json"

echo ""
echo "=== Recipe count ==="
echo "Opus:   $(jq length "$OUTDIR/opus.json")"
echo "Sonnet: $(jq length "$OUTDIR/sonnet.json")"

echo ""
echo "=== Titles only in Opus ==="
diff <(jq -r '.[].title' "$OUTDIR/opus.json" | sort) <(jq -r '.[].title' "$OUTDIR/sonnet.json" | sort) | grep "^< " | sed 's/^< /  /'

echo ""
echo "=== Titles only in Sonnet ==="
diff <(jq -r '.[].title' "$OUTDIR/opus.json" | sort) <(jq -r '.[].title' "$OUTDIR/sonnet.json" | sort) | grep "^> " | sed 's/^> /  /'

echo ""
echo "=== Field-by-field comparison (matching titles) ==="
jq -r '.[].title' "$OUTDIR/opus.json" | sort > "$OUTDIR/titles_opus.txt"
jq -r '.[].title' "$OUTDIR/sonnet.json" | sort > "$OUTDIR/titles_sonnet.txt"
comm -12 "$OUTDIR/titles_opus.txt" "$OUTDIR/titles_sonnet.txt" | while IFS= read -r title; do
  opus=$(jq --arg t "$title" '[.[] | select(.title == $t)][0]' "$OUTDIR/opus.json")
  sonnet=$(jq --arg t "$title" '[.[] | select(.title == $t)][0]' "$OUTDIR/sonnet.json")

  diffs=""
  for field in servings difficulty category cuisine prep_time cook_time total_time; do
    v1=$(echo "$opus" | jq -r ".$field // \"\"")
    v2=$(echo "$sonnet" | jq -r ".$field // \"\"")
    if [ "$v1" != "$v2" ]; then
      diffs="$diffs\n    $field: opus=\"$v1\" sonnet=\"$v2\""
    fi
  done

  ing1=$(echo "$opus" | jq '[.ingredients[].name] | sort | length')
  ing2=$(echo "$sonnet" | jq '[.ingredients[].name] | sort | length')
  if [ "$ing1" != "$ing2" ]; then
    diffs="$diffs\n    ingredients count: opus=$ing1 sonnet=$ing2"
  fi

  steps1=$(echo "$opus" | jq '.preparation | length')
  steps2=$(echo "$sonnet" | jq '.preparation | length')
  if [ "$steps1" != "$steps2" ]; then
    diffs="$diffs\n    preparation steps: opus=$steps1 sonnet=$steps2"
  fi

  if [ -n "$diffs" ]; then
    echo "  $title"
    echo -e "$diffs"
  fi
done

echo ""
echo "Full JSON exports saved to $OUTDIR/opus.json and $OUTDIR/sonnet.json"
echo "For detailed diff: diff <(jq -S . $OUTDIR/opus.json) <(jq -S . $OUTDIR/sonnet.json)"
