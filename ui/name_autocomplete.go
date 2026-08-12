package ui

import (
	"fmt"
	"genealogy/store"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

// NameFieldType specifies which name field to autocomplete
type NameFieldType int

const (
	GivenNameField NameFieldType = iota
	SurnameField
)

// NameAutocompleteContainer creates an entry with inline autocomplete suggestions for names
type NameAutocompleteContainer struct {
	Entry             *widget.Entry
	SuggestionsSelect *widget.Select
	Container         *fyne.Container
	getStore          func() *store.Store
	fieldType         NameFieldType
}

// NewGivenNameAutocomplete creates a new given name autocomplete widget
func NewGivenNameAutocomplete(getStore func() *store.Store, initialText string) *NameAutocompleteContainer {
	return newNameAutocomplete(getStore, initialText, GivenNameField, "Given name(s)")
}

// NewSurnameAutocomplete creates a new surname autocomplete widget
func NewSurnameAutocomplete(getStore func() *store.Store, initialText string) *NameAutocompleteContainer {
	return newNameAutocomplete(getStore, initialText, SurnameField, "Surname")
}

// newNameAutocomplete creates a new name autocomplete widget
func newNameAutocomplete(getStore func() *store.Store, initialText string, fieldType NameFieldType, placeholder string) *NameAutocompleteContainer {
	nac := &NameAutocompleteContainer{
		Entry:     widget.NewEntry(),
		getStore:  getStore,
		fieldType: fieldType,
	}

	nac.Entry.SetPlaceHolder(placeholder)
	nac.Entry.SetText(initialText)

	// Create suggestions dropdown (hidden by default)
	nac.SuggestionsSelect = widget.NewSelect([]string{}, func(selected string) {
		if selected != "" {
			// Extract just the name (remove frequency count if present)
			name := extractNameFromSuggestion(selected)
			nac.Entry.SetText(name)
			nac.SuggestionsSelect.Hide()
		}
	})
	nac.SuggestionsSelect.PlaceHolder = "↓ Select from matches"
	nac.SuggestionsSelect.Hide()

	// Update suggestions as user types
	nac.Entry.OnChanged = func(text string) {
		text = strings.TrimSpace(text)

		// Only show suggestions if typing (2+ chars)
		if len(text) < 2 {
			nac.SuggestionsSelect.Hide()
			return
		}

		matches := nac.queryNames(text)
		if len(matches) > 0 {
			nac.SuggestionsSelect.Options = matches
			nac.SuggestionsSelect.ClearSelected()
			nac.SuggestionsSelect.Show()
			nac.SuggestionsSelect.Refresh()
		} else {
			nac.SuggestionsSelect.Hide()
		}
	}

	// Create container
	nac.Container = container.NewVBox(
		nac.Entry,
		nac.SuggestionsSelect,
	)

	return nac
}

// queryNames gets matching names from database with frequency counts
func (nac *NameAutocompleteContainer) queryNames(query string) []string {
	s := nac.getStore()
	if s == nil {
		return nil
	}

	lowerQuery := strings.ToLower(query)
	var results []string

	if nac.fieldType == GivenNameField {
		// Query given names and preferred names
		rows, err := s.DB.Query(`
			SELECT name, COUNT(*) as count FROM (
				SELECT given_name AS name FROM persons WHERE given_name != '' AND given_name IS NOT NULL
				UNION ALL
				SELECT preferred_name AS name FROM persons WHERE preferred_name != '' AND preferred_name IS NOT NULL
			)
			WHERE LOWER(name) LIKE ?
			GROUP BY LOWER(name)
			ORDER BY count DESC, name
			LIMIT 15
		`, "%"+lowerQuery+"%")

		if err == nil {
			defer rows.Close()
			for rows.Next() {
				var name string
				var count int
				if err := rows.Scan(&name, &count); err == nil {
					name = strings.TrimSpace(name)
					if name != "" {
						if count > 1 {
							results = append(results, fmt.Sprintf("%s (%d)", name, count))
						} else {
							results = append(results, name)
						}
					}
				}
			}
		}
	} else {
		// Query surnames
		rows, err := s.DB.Query(`
			SELECT surname, COUNT(*) as count
			FROM persons
			WHERE surname != '' AND surname IS NOT NULL AND LOWER(surname) LIKE ?
			GROUP BY LOWER(surname)
			ORDER BY count DESC, surname
			LIMIT 15
		`, "%"+lowerQuery+"%")

		if err == nil {
			defer rows.Close()
			for rows.Next() {
				var name string
				var count int
				if err := rows.Scan(&name, &count); err == nil {
					name = strings.TrimSpace(name)
					if name != "" {
						if count > 1 {
							results = append(results, fmt.Sprintf("%s (%d)", name, count))
						} else {
							results = append(results, name)
						}
					}
				}
			}
		}
	}

	return results
}

// extractNameFromSuggestion removes the frequency count from a suggestion
func extractNameFromSuggestion(suggestion string) string {
	// If the suggestion has " (count)" at the end, remove it
	if idx := strings.LastIndex(suggestion, " ("); idx != -1 {
		return strings.TrimSpace(suggestion[:idx])
	}
	return suggestion
}

// GetText returns the current text in the entry
func (nac *NameAutocompleteContainer) GetText() string {
	return nac.Entry.Text
}

// SetText sets the text in the entry
func (nac *NameAutocompleteContainer) SetText(text string) {
	nac.Entry.SetText(text)
}
