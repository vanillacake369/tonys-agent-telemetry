package catalog

import (
	"bufio"
	"bytes"
	"log"
	"strings"
)

// ParseMarkdown parses the upstream CATALOG.md format into a []Item.
//
// Contract: produces []catalog.Item satisfying the same IsValid contract as
// Parse (the JSON parser). The two parsers are equivalent at the contract
// level — any Item returned by ParseMarkdown would also pass IsValid if it
// were serialised through the JSON wire format.
//
// Wire-format decision rationale: the upstream repository
// (FlorianBruniaux/claude-code-ultimate-guide) publishes only a Markdown
// catalog (examples/CATALOG.md). There is no JSON catalog file. The Markdown
// format is auto-generated with a highly regular structure, making stdlib
// regex parsing preferable to an external markdown AST library (goldmark was
// evaluated and rejected: the format does not require full HTML rendering, and
// a regex parser avoids a new dependency).
//
// Parsing rules:
//   - Only the "By Category" section (### Heading lines) is parsed.
//   - The "By Domain" section uses a different format (no links) and is skipped
//     automatically because its entries don't match reEntryHeader.
//   - Entries not matching reEntryHeader are silently skipped (headers, blank lines,
//     the "General" section, etc.).
//   - Description is taken from the indented line immediately following the entry
//     header line. If no indented line follows, Description is "".
//   - Invalid Items (those failing IsValid) are logged and dropped — same behaviour
//     as the JSON parser's graceful degrade policy.
//
// ID convention: "<type>/<slug>" where slug is derived from the file path via
// slugFromPath (see parser_markdown_helpers.go).
func ParseMarkdown(raw []byte) ([]Item, error) {
	if len(raw) == 0 {
		return nil, nil
	}

	items := make([]Item, 0, 200)

	var currentType ItemType
	var inByCategory bool

	scanner := bufio.NewScanner(bytes.NewReader(raw))

	// pendingItem holds a partially constructed item awaiting its description line.
	// nil means no item is pending.
	var pendingItem *Item

	for scanner.Scan() {
		line := scanner.Text()

		// Track whether we're inside the "By Category" block.
		// The section starts at "## By Category" and ends at "## By Domain" or EOF.
		if strings.TrimSpace(line) == "## By Category" {
			inByCategory = true
			continue
		}
		if strings.HasPrefix(strings.TrimSpace(line), "## ") && inByCategory {
			// Leaving "By Category" — flush pending item and stop.
			if pendingItem != nil {
				items = flushPending(items, pendingItem)
				pendingItem = nil
			}
			inByCategory = false
			continue
		}

		if !inByCategory {
			continue
		}

		// Category section header: "### Skills (64)"
		if m := reSectionHeader.FindStringSubmatch(strings.TrimSpace(line)); m != nil {
			if pendingItem != nil {
				items = flushPending(items, pendingItem)
				pendingItem = nil
			}
			t, ok := sectionNameToType(m[1])
			if !ok {
				currentType = "" // unknown section — skip entries until next section
			} else {
				currentType = t
			}
			continue
		}

		// Skip if current section type is unknown.
		if currentType == "" {
			continue
		}

		// Entry header line: "- **[title](path)** *intermediate* • 30 min"
		if m := reEntryHeader.FindStringSubmatch(line); m != nil {
			// Flush the previous pending item (it had no indented description line).
			if pendingItem != nil {
				items = flushPending(items, pendingItem)
			}

			title := strings.TrimSpace(m[1])
			relPath := strings.TrimSpace(m[2])
			slug := slugFromPath(relPath)

			pendingItem = &Item{
				ID:            string(currentType) + "/" + slug,
				Title:         title,
				Type:          currentType,
				Description:   "",
				Tags:          []string{},
				MaturityLevel: 0, // upstream CATALOG.md only records "intermediate"; treat as unknown
				SourceURL:     buildSourceURL(relPath),
			}
			continue
		}

		// Description line (two-space indent following an entry header).
		if pendingItem != nil {
			if m := reDescriptionLine.FindStringSubmatch(line); m != nil {
				pendingItem.Description = strings.TrimSpace(m[1])
				items = flushPending(items, pendingItem)
				pendingItem = nil
				continue
			}
			// Blank or non-indented line — flush with no description.
			if strings.TrimSpace(line) == "" || !strings.HasPrefix(line, " ") {
				items = flushPending(items, pendingItem)
				pendingItem = nil
			}
		}
	}

	// Flush any trailing pending item at EOF.
	if pendingItem != nil {
		items = flushPending(items, pendingItem)
	}

	return items, scanner.Err()
}

// flushPending validates item and appends it to items if valid.
// Invalid items are logged and discarded, matching the JSON parser behaviour.
func flushPending(items []Item, item *Item) []Item {
	if item == nil {
		return items
	}
	if !item.IsValid() {
		log.Printf("catalog(md): skipping invalid entry id=%q title=%q type=%q",
			item.ID, item.Title, item.Type)
		return items
	}
	return append(items, *item)
}
