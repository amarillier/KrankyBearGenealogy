package ui

import (
	"fmt"
	"genealogy/store"
)

// formatPlaceWithHistoricalContext formats a place name with modern equivalent if available
// Example: "Salisbury, Rhodesia" → "Salisbury, Rhodesia (now Harare, Zimbabwe)"
func formatPlaceWithHistoricalContext(placeName string, s *store.Store) string {
	if placeName == "" {
		return ""
	}
	
	// Check if there's an alternate place mapping
	if altPlace, err := s.GetAlternatePlaceFor(placeName); err == nil && altPlace != nil {
		return fmt.Sprintf("%s (now %s)", placeName, altPlace.CurrentName)
	}
	
	// No alternate found, return as-is
	return placeName
}
