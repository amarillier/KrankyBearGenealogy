package ui

import (
	"regexp"
	"strconv"
	"strings"
	"time"
)

// DateParseResult represents the result of parsing a date string
type DateParseResult struct {
	Valid        bool      // Whether the date could be parsed
	Date         time.Time // Parsed date (if valid)
	OriginalText string    // Original input text
	Format       string    // Detected format description
	IsEstimated  bool      // True for "abt", "est", "circa", etc.
	IsRange      bool      // True for "between", "from...to", etc.
	IsBefore     bool      // True for "bef", "before"
	IsAfter      bool      // True for "aft", "after"
	Year         int       // Extracted year (if any)
	Month        int       // Extracted month (if any)
	Day          int       // Extracted day (if any)
}

// ParseGenealogy date attempts to parse various genealogy date formats
func ParseGenealogyDate(dateStr string) DateParseResult {
	result := DateParseResult{
		OriginalText: dateStr,
		Valid:        false,
	}

	dateStr = strings.TrimSpace(dateStr)
	if dateStr == "" {
		return result
	}

	lowerDate := strings.ToLower(dateStr)

	// Check for modifiers
	result.IsEstimated = containsAny(lowerDate, []string{"abt", "about", "circa", "ca", "est", "estimated", "~"})
	result.IsBefore = containsAny(lowerDate, []string{"bef", "before", "<"})
	result.IsAfter = containsAny(lowerDate, []string{"aft", "after", ">"})
	result.IsRange = containsAny(lowerDate, []string{"between", "from", "to", "-"})

	// Remove common modifiers to extract the actual date
	cleanDate := dateStr
	modifiers := []string{"abt ", "about ", "circa ", "ca ", "ca. ", "est ", "estimated ", "bef ", "before ", "aft ", "after ", "~"}
	for _, mod := range modifiers {
		cleanDate = strings.Replace(strings.ToLower(cleanDate), mod, "", 1)
		cleanDate = strings.TrimSpace(cleanDate)
	}

	// Try various date formats
	formats := []struct {
		layout      string
		description string
	}{
		{"2006-01-02", "ISO format (YYYY-MM-DD)"},
		{"2 Jan 2006", "Day Month Year"},
		{"02 Jan 2006", "DD Mon YYYY"},
		{"Jan 2006", "Month Year"},
		{"January 2006", "Month Year (full)"},
		{"2006", "Year only"},
		{"2/1/2006", "M/D/YYYY"},
		{"02/01/2006", "MM/DD/YYYY"},
		{"2-1-2006", "M-D-YYYY"},
		{"02-01-2006", "MM-DD-YYYY"},
	}

	// Try parsing with standard formats
	for _, fmt := range formats {
		if t, err := time.Parse(fmt.layout, cleanDate); err == nil {
			result.Valid = true
			result.Date = t
			result.Format = fmt.description
			result.Year = t.Year()
			result.Month = int(t.Month())
			result.Day = t.Day()
			return result
		}
	}

	// Try to extract just a year (only if it's essentially JUST a year, not part of a malformed date)
	// Only match if the string is primarily the year (e.g., "1945", " 1945 ", but not "01 Jans 1945")
	yearOnlyRegex := regexp.MustCompile(`^\s*(1[0-9]{3}|20[0-9]{2})\s*$`)
	if matches := yearOnlyRegex.FindStringSubmatch(cleanDate); len(matches) > 1 {
		if year, err := strconv.Atoi(matches[1]); err == nil {
			result.Valid = true
			result.Year = year
			result.Date = time.Date(year, 1, 1, 0, 0, 0, 0, time.UTC)
			result.Format = "Year only"
			return result
		}
	}

	return result
}

// FormatGenealogyDate formats a date with modifiers for display
func FormatGenealogyDate(result DateParseResult) string {
	if !result.Valid {
		return result.OriginalText
	}

	dateStr := ""
	
	// Add year, month, day if available
	if result.Year > 0 {
		if result.Month > 0 {
			if result.Day > 0 {
				dateStr = result.Date.Format("2 Jan 2006")
			} else {
				dateStr = result.Date.Format("Jan 2006")
			}
		} else {
			dateStr = strconv.Itoa(result.Year)
		}
	}

	// Add modifiers
	if result.IsEstimated {
		dateStr = "abt " + dateStr
	} else if result.IsBefore {
		dateStr = "bef " + dateStr
	} else if result.IsAfter {
		dateStr = "aft " + dateStr
	}

	return dateStr
}

// ValidateDateString returns true if the date string can be parsed
func ValidateDateString(dateStr string) bool {
	return ParseGenealogyDate(dateStr).Valid
}

// GetDateValidationMessage returns a user-friendly message about date validity
func GetDateValidationMessage(dateStr string) string {
	if dateStr == "" {
		return ""
	}

	result := ParseGenealogyDate(dateStr)
	if !result.Valid {
		return "⚠ Unable to parse date. Supported formats: YYYY-MM-DD, DD Mon YYYY, Mon YYYY, YYYY, or use 'abt', 'bef', 'aft' prefixes"
	}

	msg := "✓ Recognized as " + result.Format
	
	if result.IsEstimated {
		msg += " (estimated)"
	} else if result.IsBefore {
		msg += " (before)"
	} else if result.IsAfter {
		msg += " (after)"
	}

	return msg
}

// containsAny returns true if the string contains any of the substrings
func containsAny(s string, substrs []string) bool {
	for _, substr := range substrs {
		if strings.Contains(s, substr) {
			return true
		}
	}
	return false
}

// NormalizeDateForStorage converts various date formats to YYYY-MM-DD when possible
func NormalizeDateForStorage(dateStr string) string {
	result := ParseGenealogyDate(dateStr)
	if !result.Valid {
		return dateStr // Keep original if can't parse
	}

	// For precise dates, use ISO format
	if result.Day > 0 && result.Month > 0 && result.Year > 0 {
		baseDate := result.Date.Format("2006-01-02")
		
		// Add modifiers back
		if result.IsEstimated {
			return "abt " + baseDate
		} else if result.IsBefore {
			return "bef " + baseDate
		} else if result.IsAfter {
			return "aft " + baseDate
		}
		return baseDate
	}

	// For partial dates, use formatted version
	return FormatGenealogyDate(result)
}
