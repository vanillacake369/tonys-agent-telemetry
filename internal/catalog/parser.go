package catalog

import (
	"encoding/json"
	"fmt"
	"log"
)

// wireItem is the JSON deserialization shape for a catalog entry.
// It maps snake_case JSON keys to Go struct fields before conversion to Item.
// The wire format is JSON array (not markdown) for v0 because reliable markdown
// parsing would require a third-party dependency. A future markdown adapter can
// live in parser_markdown.go without changing this file.
//
// TODO: evaluate upstream markdown catalog at
// https://github.com/FlorianBruniaux/claude-code-ultimate-guide for a Phase 2
// markdown adapter once the catalog shape stabilizes.
type wireItem struct {
	ID            string   `json:"id"`
	Title         string   `json:"title"`
	Type          ItemType `json:"type"`
	Description   string   `json:"description"`
	Tags          []string `json:"tags"`
	MaturityLevel int      `json:"maturity_level"`
	SourceURL     string   `json:"source_url"`
}

// Parse deserializes a JSON byte slice into a []Item.
// The expected wire format is a JSON array of objects matching wireItem.
// Invalid entries (those that fail Item.IsValid) are logged and skipped rather
// than returned as errors — graceful degrade ensures a partially-valid corpus
// is still usable. A JSON syntax error does return an error because the entire
// payload is corrupt in that case.
func Parse(raw []byte) ([]Item, error) {
	var wires []wireItem
	if err := json.Unmarshal(raw, &wires); err != nil {
		return nil, fmt.Errorf("catalog: JSON parse error: %w", err)
	}

	items := make([]Item, 0, len(wires))
	for _, w := range wires {
		item := Item{
			ID:            w.ID,
			Title:         w.Title,
			Type:          w.Type,
			Description:   w.Description,
			Tags:          w.Tags,
			MaturityLevel: w.MaturityLevel,
			SourceURL:     w.SourceURL,
		}
		if !item.IsValid() {
			log.Printf("catalog: skipping invalid entry id=%q title=%q type=%q", item.ID, item.Title, item.Type)
			continue
		}
		items = append(items, item)
	}
	return items, nil
}
