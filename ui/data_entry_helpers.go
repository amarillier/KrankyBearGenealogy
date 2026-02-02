package ui

import (
	"strings"
	"unicode"
)

// SmartCapitalizePlace attempts to properly capitalize place names
// Examples: "austin, texas" → "Austin, Texas"
//           "NEW YORK" → "New York"
//           "united states" → "United States"
func SmartCapitalizePlace(place string) string {
	if place == "" {
		return place
	}

	// Split by comma for city, state, country format
	parts := strings.Split(place, ",")
	for i, part := range parts {
		parts[i] = SmartCapitalizePart(strings.TrimSpace(part))
	}
	
	return strings.Join(parts, ", ")
}

// SmartCapitalizePart capitalizes a single part (city, state, country)
func SmartCapitalizePart(s string) string {
	if s == "" {
		return s
	}

	// Common abbreviations that should stay uppercase
	upperAbbrev := map[string]bool{
		// Countries
		"usa":  true,
		"uk":   true,
		"ussr": true,
		// US States (2-letter codes)
		"al": true, "ak": true, "az": true, "ar": true, "ca": true, "co": true,
		"ct": true, "de": true, "fl": true, "ga": true, "hi": true, "id": true,
		"il": true, "in": true, "ia": true, "ks": true, "ky": true, "la": true,
		"me": true, "md": true, "ma": true, "mi": true, "mn": true, "ms": true,
		"mo": true, "mt": true, "ne": true, "nv": true, "nh": true, "nj": true,
		"nm": true, "ny": true, "nc": true, "nd": true, "oh": true, "ok": true,
		"or": true, "pa": true, "ri": true, "sc": true, "sd": true, "tn": true,
		"tx": true, "ut": true, "vt": true, "va": true, "wa": true, "wv": true,
		"wi": true, "wy": true,
		// Territories
		"dc": true, "pr": true, "vi": true, "gu": true, "as": true, "mp": true,
		// Canadian Provinces
		"ab": true, "bc": true, "mb": true, "nb": true, "nl": true, "nt": true,
		"ns": true, "nu": true, "on": true, "pe": true, "qc": true, "sk": true, "yt": true,
	}

	// Check if it's a known abbreviation (must be exact match, not part of word)
	lower := strings.ToLower(s)
	trimmed := strings.TrimSpace(lower)
	if upperAbbrev[trimmed] {
		return strings.ToUpper(s)
	}

	// Split by spaces and capitalize each word
	words := strings.Fields(s)
	for i, word := range words {
		words[i] = capitalizePlaceWord(word)
	}
	
	return strings.Join(words, " ")
}

// capitalizePlaceWord capitalizes a single word with special handling for place names
func capitalizePlaceWord(word string) string {
	if word == "" {
		return word
	}

	lower := strings.ToLower(word)

	// Special cases for common place name patterns
	specialCases := map[string]string{
		"st":    "St",
		"st.":   "St.",
		"mt":    "Mt",
		"mt.":   "Mt.",
		"ft":    "Ft",
		"ft.":   "Ft.",
		"d.c.":  "D.C.",
		"la":    "La",
		"los":   "Los",
		"las":   "Las",
		"san":   "San",
		"santa": "Santa",
		"del":   "del",
		"de":    "de",
		"von":   "von",
		"van":   "van",
	}

	if special, ok := specialCases[lower]; ok {
		return special
	}

	// Handle hyphenated words (e.g., "Winston-Salem")
	if strings.Contains(word, "-") {
		parts := strings.Split(word, "-")
		for i, part := range parts {
			parts[i] = strings.Title(strings.ToLower(part))
		}
		return strings.Join(parts, "-")
	}

	// Handle apostrophes (e.g., "O'Fallon")
	if strings.Contains(word, "'") {
		parts := strings.Split(word, "'")
		for i, part := range parts {
			if part != "" {
				parts[i] = strings.Title(strings.ToLower(part))
			}
		}
		return strings.Join(parts, "'")
	}

	// Default: Title case
	return strings.Title(lower)
}

// SmartTrimPlace removes extra whitespace and normalizes place format
func SmartTrimPlace(place string) string {
	if place == "" {
		return place
	}

	// Remove leading/trailing spaces
	place = strings.TrimSpace(place)

	// Normalize spaces around commas
	place = strings.ReplaceAll(place, " ,", ",")
	place = strings.ReplaceAll(place, ",", ", ")
	
	// Remove double spaces
	for strings.Contains(place, "  ") {
		place = strings.ReplaceAll(place, "  ", " ")
	}

	return place
}

// IsAllUppercase returns true if the string is all uppercase letters
func IsAllUppercase(s string) bool {
	if s == "" {
		return false
	}
	
	hasLetter := false
	for _, r := range s {
		if unicode.IsLetter(r) {
			hasLetter = true
			if !unicode.IsUpper(r) {
				return false
			}
		}
	}
	
	return hasLetter
}

// IsAllLowercase returns true if the string is all lowercase letters
func IsAllLowercase(s string) bool {
	if s == "" {
		return false
	}
	
	hasLetter := false
	for _, r := range s {
		if unicode.IsLetter(r) {
			hasLetter = true
			if !unicode.IsLower(r) {
				return false
			}
		}
	}
	
	return hasLetter
}

// AutoCapitalizePlace automatically capitalizes a single place field when appropriate
func AutoCapitalizePlace(place *string) {
	if place == nil || *place == "" {
		return
	}

	// Split by comma and check each part independently
	parts := strings.Split(*place, ",")
	anyChanged := false
	
	for i, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed == "" {
			continue
		}
		
		// Auto-capitalize if this part is all uppercase or all lowercase
		if IsAllUppercase(trimmed) || IsAllLowercase(trimmed) {
			parts[i] = SmartCapitalizePart(trimmed)
			anyChanged = true
		} else {
			// Keep as-is but trim spaces
			parts[i] = trimmed
		}
	}
	
	// Rejoin with proper comma-space formatting
	if anyChanged {
		*place = strings.Join(parts, ", ")
	} else {
		// Just normalize spacing
		*place = SmartTrimPlace(*place)
	}
}

// AutoCapitalizePlaces automatically capitalizes place fields when appropriate
// Called when user finishes editing place fields
func AutoCapitalizePlaces(birthPlace, deathPlace *string) {
	AutoCapitalizePlace(birthPlace)
	AutoCapitalizePlace(deathPlace)
}

// AutoCapitalizeContactPlaces auto-capitalizes contact/address place fields
func AutoCapitalizeContactPlaces(city, state, country *string) {
	AutoCapitalizePlace(city)
	AutoCapitalizePlace(state)
	AutoCapitalizePlace(country)
}
