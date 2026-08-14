package ui

import (
	"fmt"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/dialog"

	"genealogy/store"
)

// icsEscape escapes text per RFC 5545 (commas, semicolons, backslashes,
// newlines) for use in an iCalendar SUMMARY/DESCRIPTION field.
func icsEscape(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, ";", "\\;")
	s = strings.ReplaceAll(s, ",", "\\,")
	s = strings.ReplaceAll(s, "\n", "\\n")
	return s
}

// icsEvent writes a single yearly-recurring VEVENT to sb.
func icsEvent(sb *strings.Builder, uid, dtstamp string, year, month, day int, summary, description string) {
	if year == 0 {
		year = 1900 // placeholder anchor so the event still has a valid DTSTART
	}
	sb.WriteString("BEGIN:VEVENT\r\n")
	sb.WriteString(fmt.Sprintf("UID:%s\r\n", uid))
	sb.WriteString(fmt.Sprintf("DTSTAMP:%s\r\n", dtstamp))
	sb.WriteString(fmt.Sprintf("DTSTART;VALUE=DATE:%04d%02d%02d\r\n", year, month, day))
	sb.WriteString("RRULE:FREQ=YEARLY\r\n")
	sb.WriteString(fmt.Sprintf("SUMMARY:%s\r\n", icsEscape(summary)))
	if description != "" {
		sb.WriteString(fmt.Sprintf("DESCRIPTION:%s\r\n", icsEscape(description)))
	}
	sb.WriteString("END:VEVENT\r\n")
}

// exportCalendarICS exports birthdays and wedding anniversaries as a
// standard iCalendar (.ics) file with yearly recurring events, importable
// into Calendar, Outlook, or Google Calendar.
func exportCalendarICS(w fyne.Window, s *store.Store) {
	people, err := s.GetAllPeople()
	if err != nil {
		dialog.ShowError(fmt.Errorf("Failed to get people: %w", err), w)
		return
	}

	relationships, err := s.GetRelationships()
	if err != nil {
		dialog.ShowError(fmt.Errorf("Failed to get relationships: %w", err), w)
		return
	}

	fd := dialog.NewFileSave(func(uc fyne.URIWriteCloser, err error) {
		if err != nil || uc == nil {
			return
		}
		defer uc.Close()

		dtstamp := time.Now().UTC().Format("20060102T150405Z")

		var sb strings.Builder
		sb.WriteString("BEGIN:VCALENDAR\r\n")
		sb.WriteString("VERSION:2.0\r\n")
		sb.WriteString("PRODID:-//KrankyBear Genealogy//Calendar Export//EN\r\n")
		sb.WriteString("CALSCALE:GREGORIAN\r\n")

		eventCount := 0

		// Birthdays
		for _, p := range people {
			result := ParseGenealogyDate(p.BirthDate)
			// Skip "year only" results - ParseGenealogyDate defaults
			// Month/Day to January 1 via time.Parse's zero-fill when only
			// a year is known, which would otherwise silently produce a
			// bogus "Jan 1" birthday/anniversary.
			if !result.Valid || result.Format == "Year only" {
				continue
			}
			name := fmt.Sprintf("%s %s", p.GivenName, p.Surname)
			description := ""
			if p.BirthPlace != "" {
				description = fmt.Sprintf("Born in %s", p.BirthPlace)
			}
			icsEvent(&sb, fmt.Sprintf("person-%d-birthday@krankybear-genealogy", p.ID), dtstamp,
				result.Year, result.Month, result.Day,
				fmt.Sprintf("%s's Birthday", name), description)
			eventCount++
		}

		// Wedding anniversaries - relationships are stored in both
		// directions, so only process one direction per pair to avoid
		// emitting the same marriage twice.
		for _, rel := range relationships {
			if rel.Type != "spouse" || rel.MarriageDate == "" || rel.SubjectID >= rel.ObjectID {
				continue
			}
			result := ParseGenealogyDate(rel.MarriageDate)
			// Skip "year only" results - ParseGenealogyDate defaults
			// Month/Day to January 1 via time.Parse's zero-fill when only
			// a year is known, which would otherwise silently produce a
			// bogus "Jan 1" birthday/anniversary.
			if !result.Valid || result.Format == "Year only" {
				continue
			}
			person1, err1 := s.GetPersonByID(rel.SubjectID)
			person2, err2 := s.GetPersonByID(rel.ObjectID)
			if err1 != nil || err2 != nil {
				continue
			}
			description := ""
			if rel.MarriagePlace != "" {
				description = fmt.Sprintf("Married in %s", rel.MarriagePlace)
			}
			icsEvent(&sb, fmt.Sprintf("marriage-%d-%d@krankybear-genealogy", rel.SubjectID, rel.ObjectID), dtstamp,
				result.Year, result.Month, result.Day,
				fmt.Sprintf("%s & %s's Anniversary", formatPersonName(*person1), formatPersonName(*person2)), description)
			eventCount++
		}

		sb.WriteString("END:VCALENDAR\r\n")

		if _, err := uc.Write([]byte(sb.String())); err != nil {
			dialog.ShowError(fmt.Errorf("Failed to write calendar file: %w", err), w)
			return
		}

		dialog.ShowInformation("Export Successful",
			fmt.Sprintf("Exported %d birthdays and anniversaries to:\n%s", eventCount, uc.URI().Path()), w)
	}, w)

	fd.SetFileName("genealogy_calendar.ics")
	fd.Show()
}
