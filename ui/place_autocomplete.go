package ui

import (
	"genealogy/store"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

// PlaceAutocompleteContainer creates an entry with inline autocomplete suggestions
type PlaceAutocompleteContainer struct {
	Entry             *widget.Entry
	SuggestionsSelect *widget.Select
	Container         *fyne.Container
	getStore          func() *store.Store
}

// NewPlaceAutocompleteContainer creates a new place autocomplete widget
func NewPlaceAutocompleteContainer(getStore func() *store.Store, initialText string) *PlaceAutocompleteContainer {
	pac := &PlaceAutocompleteContainer{
		Entry:    widget.NewEntry(),
		getStore: getStore,
	}

	pac.Entry.SetPlaceHolder("City, State/Province, Country")
	pac.Entry.SetText(initialText)

	// Create suggestions dropdown (hidden by default)
	pac.SuggestionsSelect = widget.NewSelect([]string{}, func(selected string) {
		if selected != "" {
			pac.Entry.SetText(selected)
			pac.SuggestionsSelect.Hide()
		}
	})
	pac.SuggestionsSelect.PlaceHolder = "↓ Select from matches"
	pac.SuggestionsSelect.Hide()

	// Update suggestions as user types
	pac.Entry.OnChanged = func(text string) {
		text = strings.TrimSpace(text)

		// Only show suggestions if typing (2+ chars)
		if len(text) < 2 {
			pac.SuggestionsSelect.Hide()
			return
		}

		matches := pac.queryPlaces(text)
		if len(matches) > 0 {
			pac.SuggestionsSelect.Options = matches
			pac.SuggestionsSelect.ClearSelected()
			pac.SuggestionsSelect.Show()
			pac.SuggestionsSelect.Refresh()
		} else {
			pac.SuggestionsSelect.Hide()
		}
	}

	// Create container
	pac.Container = container.NewVBox(
		pac.Entry,
		pac.SuggestionsSelect,
	)

	return pac
}

// queryPlaces gets matching place names from database
func (pac *PlaceAutocompleteContainer) queryPlaces(query string) []string {
	s := pac.getStore()
	if s == nil {
		return nil
	}

	lowerQuery := strings.ToLower(query)
	placeMap := make(map[string]bool)
	var places []string

	// Query all places from database
	rows, err := s.DB.Query(`
		SELECT DISTINCT place FROM (
			SELECT birth_place AS place FROM persons WHERE birth_place != '' AND birth_place IS NOT NULL
			UNION
			SELECT death_place AS place FROM persons WHERE death_place != '' AND death_place IS NOT NULL
			UNION
			SELECT marriage_place AS place FROM relationships WHERE marriage_place != '' AND marriage_place IS NOT NULL
		)
		WHERE LOWER(place) LIKE ?
		ORDER BY place
		LIMIT 10
	`, "%"+lowerQuery+"%")

	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var place string
			if err := rows.Scan(&place); err == nil {
				place = strings.TrimSpace(place)
				if place != "" && !placeMap[place] {
					placeMap[place] = true
					places = append(places, place)
				}
			}
		}
	}

	return places
}

// GetText returns the current text in the entry
func (pac *PlaceAutocompleteContainer) GetText() string {
	return pac.Entry.Text
}

// SetText sets the text in the entry
func (pac *PlaceAutocompleteContainer) SetText(text string) {
	pac.Entry.SetText(text)
}
