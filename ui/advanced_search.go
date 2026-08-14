package ui

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"

	"genealogy/store"
)

// SearchCriteria holds all search parameters
type SearchCriteria struct {
	// Name fields
	GivenName   string
	Surname     string
	PreferredName string
	
	// Date fields
	BirthYearFrom int
	BirthYearTo   int
	DeathYearFrom int
	DeathYearTo   int
	
	// Place fields
	BirthPlace string
	DeathPlace string
	
	// Boolean filters
	LivingOnly    bool
	DeceasedOnly  bool
	HasMedia      bool
	HasSources    bool
	HasTodos      bool
	IsBookmarked  bool
	
	// Gender filter
	Gender string // "", "M", "F"
}

// showAdvancedSearchDialog displays the advanced search window
func showAdvancedSearchDialog(w fyne.Window, s *store.Store, navigateToPerson func(int64)) {
	// Create a new window for advanced search
	searchWindow := fyne.CurrentApp().NewWindow("Advanced Search")

	// Search criteria inputs with autocomplete
	givenNameAutocomplete := NewGivenNameAutocomplete(func() *store.Store { return s }, "")
	givenNameAutocomplete.Entry.SetPlaceHolder("Given name...")
	
	surnameAutocomplete := NewSurnameAutocomplete(func() *store.Store { return s }, "")
	surnameAutocomplete.Entry.SetPlaceHolder("Surname...")
	
	preferredNameAutocomplete := NewGivenNameAutocomplete(func() *store.Store { return s }, "")
	preferredNameAutocomplete.Entry.SetPlaceHolder("Preferred/nickname...")
	
	// Date range inputs
	birthYearFromEntry := widget.NewEntry()
	birthYearFromEntry.SetPlaceHolder("From year")
	
	birthYearToEntry := widget.NewEntry()
	birthYearToEntry.SetPlaceHolder("To year")
	
	deathYearFromEntry := widget.NewEntry()
	deathYearFromEntry.SetPlaceHolder("From year")
	
	deathYearToEntry := widget.NewEntry()
	deathYearToEntry.SetPlaceHolder("To year")
	
	// Place inputs
	birthPlaceEntry := widget.NewEntry()
	birthPlaceEntry.SetPlaceHolder("Birth place (contains)...")
	
	deathPlaceEntry := widget.NewEntry()
	deathPlaceEntry.SetPlaceHolder("Death place (contains)...")
	
	// Gender filter
	genderSelect := widget.NewSelect([]string{"Any", "Male", "Female"}, func(string) {})
	genderSelect.SetSelected("Any")
	
	// Boolean filters
	livingOnlyCheck := widget.NewCheck("Living only", func(bool) {})
	deceasedOnlyCheck := widget.NewCheck("Deceased only", func(bool) {})
	hasMediaCheck := widget.NewCheck("Has media", func(bool) {})
	hasSourcesCheck := widget.NewCheck("Has sources", func(bool) {})
	hasTodosCheck := widget.NewCheck("Has pending to-dos", func(bool) {})
	isBookmarkedCheck := widget.NewCheck("Bookmarked", func(bool) {})
	
	// Mutual exclusion for living/deceased
	livingOnlyCheck.OnChanged = func(checked bool) {
		if checked {
			deceasedOnlyCheck.SetChecked(false)
		}
	}
	deceasedOnlyCheck.OnChanged = func(checked bool) {
		if checked {
			livingOnlyCheck.SetChecked(false)
		}
	}
	
	// Results display
	resultsLabel := widget.NewLabel("Enter search criteria and click Search")
	resultsLabel.Wrapping = fyne.TextWrapWord
	
	var resultsList *widget.List
	var results []store.Person
	
	resultsList = widget.NewList(
		func() int { return len(results) },
		func() fyne.CanvasObject {
			return widget.NewLabel("template")
		},
		func(i widget.ListItemID, o fyne.CanvasObject) {
			if i >= len(results) {
				return
			}
			p := results[i]
			label := o.(*widget.Label)
			
			// Build display text
			name := formatPersonName(p)
			dates := ""
			if p.BirthDate != "" {
				dates = fmt.Sprintf("b. %s", p.BirthDate)
			}
			if p.DeathDate != "" {
				if dates != "" {
					dates += " - "
				}
				dates += fmt.Sprintf("d. %s", p.DeathDate)
			} else if p.IsLiving {
				if dates != "" {
					dates += " - "
				}
				dates += "Living"
			}
			
			displayText := name
			if dates != "" {
				displayText += fmt.Sprintf(" (%s)", dates)
			}
			
			label.SetText(displayText)
		},
	)
	
	resultsList.OnSelected = func(id widget.ListItemID) {
		if int(id) < len(results) {
			navigateToPerson(results[id].ID)
			searchWindow.Close()
		}
	}
	
	// Search button
	var searchBtn *widget.Button
	searchBtn = widget.NewButton("Search", func() {
		searchBtn.Disable()
		defer searchBtn.Enable()
		
		// Build search criteria
		criteria := SearchCriteria{
			GivenName:     strings.TrimSpace(givenNameAutocomplete.GetText()),
			Surname:       strings.TrimSpace(surnameAutocomplete.GetText()),
			PreferredName: strings.TrimSpace(preferredNameAutocomplete.GetText()),
			BirthPlace:    strings.TrimSpace(birthPlaceEntry.Text),
			DeathPlace:    strings.TrimSpace(deathPlaceEntry.Text),
			LivingOnly:    livingOnlyCheck.Checked,
			DeceasedOnly:  deceasedOnlyCheck.Checked,
			HasMedia:      hasMediaCheck.Checked,
			HasSources:    hasSourcesCheck.Checked,
			HasTodos:      hasTodosCheck.Checked,
			IsBookmarked:  isBookmarkedCheck.Checked,
		}
		
		// Parse year ranges
		if birthYearFromEntry.Text != "" {
			if year, err := strconv.Atoi(strings.TrimSpace(birthYearFromEntry.Text)); err == nil {
				criteria.BirthYearFrom = year
			}
		}
		if birthYearToEntry.Text != "" {
			if year, err := strconv.Atoi(strings.TrimSpace(birthYearToEntry.Text)); err == nil {
				criteria.BirthYearTo = year
			}
		}
		if deathYearFromEntry.Text != "" {
			if year, err := strconv.Atoi(strings.TrimSpace(deathYearFromEntry.Text)); err == nil {
				criteria.DeathYearFrom = year
			}
		}
		if deathYearToEntry.Text != "" {
			if year, err := strconv.Atoi(strings.TrimSpace(deathYearToEntry.Text)); err == nil {
				criteria.DeathYearTo = year
			}
		}
		
		// Gender filter
		switch genderSelect.Selected {
		case "Male":
			criteria.Gender = "M"
		case "Female":
			criteria.Gender = "F"
		default:
			criteria.Gender = ""
		}
		
		// Perform search
		results = performAdvancedSearch(s, criteria)
		
		// Update results display
		resultsLabel.SetText(fmt.Sprintf("Found %d matching people", len(results)))
		resultsList.Refresh()
	})
	searchBtn.Importance = widget.HighImportance
	
	// Clear button
	clearBtn := widget.NewButton("Clear", func() {
		givenNameAutocomplete.SetText("")
		surnameAutocomplete.SetText("")
		preferredNameAutocomplete.SetText("")
		birthYearFromEntry.SetText("")
		birthYearToEntry.SetText("")
		deathYearFromEntry.SetText("")
		deathYearToEntry.SetText("")
		birthPlaceEntry.SetText("")
		deathPlaceEntry.SetText("")
		genderSelect.SetSelected("Any")
		livingOnlyCheck.SetChecked(false)
		deceasedOnlyCheck.SetChecked(false)
		hasMediaCheck.SetChecked(false)
		hasSourcesCheck.SetChecked(false)
		hasTodosCheck.SetChecked(false)
		isBookmarkedCheck.SetChecked(false)
		results = nil
		resultsLabel.SetText("Enter search criteria and click Search")
		resultsList.Refresh()
	})
	
	// Export button with format options
	exportBtn := widget.NewButton("Export Results...", func() {
		if len(results) == 0 {
			dialog.ShowInformation("No Results", "No search results to export", searchWindow)
			return
		}
		
		showExportDialog(searchWindow, results)
	})
	
	// Saved searches functionality
	savedSearchSelect := widget.NewSelect([]string{}, func(selected string) {
		if selected == "" {
			return
		}
		// Load the selected saved search
		savedSearches, err := s.GetSavedSearches()
		if err != nil {
			dialog.ShowError(fmt.Errorf("Failed to load saved searches: %v", err), searchWindow)
			return
		}
		
		for _, ss := range savedSearches {
			if ss.Name == selected {
				loadSearchCriteria(ss.CriteriaJSON, givenNameAutocomplete, surnameAutocomplete, preferredNameAutocomplete,
					birthYearFromEntry, birthYearToEntry, deathYearFromEntry, deathYearToEntry,
					birthPlaceEntry, deathPlaceEntry, genderSelect,
					livingOnlyCheck, deceasedOnlyCheck, hasMediaCheck, hasSourcesCheck,
					hasTodosCheck, isBookmarkedCheck)
				break
			}
		}
	})
	
	// Function to refresh saved searches dropdown
	refreshSavedSearches := func() {
		savedSearches, err := s.GetSavedSearches()
		if err != nil {
			return
		}
		options := []string{""} // Empty option for "Select saved search..."
		for _, ss := range savedSearches {
			options = append(options, ss.Name)
		}
		savedSearchSelect.SetOptions(options)
		if len(options) > 1 {
			savedSearchSelect.SetSelected("")
		}
	}
	refreshSavedSearches()
	
	// Save search button
	saveSearchBtn := widget.NewButton("Save Search...", func() {
		// Get current criteria
		criteria := getCurrentSearchCriteria(givenNameAutocomplete, surnameAutocomplete, preferredNameAutocomplete,
			birthYearFromEntry, birthYearToEntry, deathYearFromEntry, deathYearToEntry,
			birthPlaceEntry, deathPlaceEntry, genderSelect,
			livingOnlyCheck, deceasedOnlyCheck, hasMediaCheck, hasSourcesCheck,
			hasTodosCheck, isBookmarkedCheck)
		
		// Convert to JSON
		criteriaJSON, err := json.Marshal(criteria)
		if err != nil {
			dialog.ShowError(fmt.Errorf("Failed to encode search criteria: %v", err), searchWindow)
			return
		}
		
		// Show dialog to enter name
		nameEntry := widget.NewEntry()
		nameEntry.SetPlaceHolder("Enter a name for this search...")
		
		content := container.NewVBox(
			widget.NewLabel("Save this search for reuse:"),
			nameEntry,
		)
		
		dialog.ShowCustomConfirm("Save Search", "Save", "Cancel", content, func(save bool) {
			if !save {
				return
			}
			name := strings.TrimSpace(nameEntry.Text)
			if name == "" {
				dialog.ShowInformation("Name Required", "Please enter a name for this search.", searchWindow)
				return
			}
			
			// Check if name already exists
			savedSearches, _ := s.GetSavedSearches()
			for _, ss := range savedSearches {
				if ss.Name == name {
					dialog.ShowConfirm("Overwrite?", fmt.Sprintf("A search named '%s' already exists. Overwrite it?", name),
						func(overwrite bool) {
							if overwrite {
								if err := s.UpdateSavedSearch(ss.ID, name, string(criteriaJSON)); err != nil {
									dialog.ShowError(fmt.Errorf("Failed to update search: %v", err), searchWindow)
									return
								}
								refreshSavedSearches()
								dialog.ShowInformation("Saved", fmt.Sprintf("Search '%s' updated successfully.", name), searchWindow)
							}
						}, searchWindow)
					return
				}
			}
			
			// Create new saved search
			if err := s.CreateSavedSearch(name, string(criteriaJSON)); err != nil {
				dialog.ShowError(fmt.Errorf("Failed to save search: %v", err), searchWindow)
				return
			}
			refreshSavedSearches()
			dialog.ShowInformation("Saved", fmt.Sprintf("Search '%s' saved successfully.", name), searchWindow)
		}, searchWindow)
	})
	
	// Delete search button
	deleteSearchBtn := widget.NewButton("Delete Search...", func() {
		selected := savedSearchSelect.Selected
		if selected == "" {
			dialog.ShowInformation("No Selection", "Please select a saved search to delete.", searchWindow)
			return
		}
		
		dialog.ShowConfirm("Delete Search", fmt.Sprintf("Delete saved search '%s'?", selected),
			func(delete bool) {
				if !delete {
					return
				}
				
				savedSearches, err := s.GetSavedSearches()
				if err != nil {
					dialog.ShowError(fmt.Errorf("Failed to load saved searches: %v", err), searchWindow)
					return
				}
				
				for _, ss := range savedSearches {
					if ss.Name == selected {
						if err := s.DeleteSavedSearch(ss.ID); err != nil {
							dialog.ShowError(fmt.Errorf("Failed to delete search: %v", err), searchWindow)
							return
						}
						refreshSavedSearches()
						dialog.ShowInformation("Deleted", fmt.Sprintf("Search '%s' deleted successfully.", selected), searchWindow)
						return
					}
				}
			}, searchWindow)
	})
	
	// Saved searches section
	savedSearchesSection := container.NewHBox(
		widget.NewLabel("Saved Searches:"),
		savedSearchSelect,
		saveSearchBtn,
		deleteSearchBtn,
	)
	
	// Build search form - buttons at TOP for easy access
	buttonBar := container.NewGridWithColumns(3, searchBtn, clearBtn, exportBtn)
	
	nameSection := container.NewVBox(
		widget.NewLabel("Name Criteria:"),
		widget.NewLabel("Given Name:"), givenNameAutocomplete.Container,
		widget.NewLabel("Surname:"), surnameAutocomplete.Container,
		widget.NewLabel("Preferred/Nickname:"), preferredNameAutocomplete.Container,
	)
	
	birthDateSection := container.NewVBox(
		widget.NewLabel("Birth Year Range:"),
		container.NewGridWithColumns(2,
			birthYearFromEntry,
			birthYearToEntry,
		),
	)
	
	deathDateSection := container.NewVBox(
		widget.NewLabel("Death Year Range:"),
		container.NewGridWithColumns(2,
			deathYearFromEntry,
			deathYearToEntry,
		),
	)
	
	placeSection := container.NewVBox(
		widget.NewLabel("Place Criteria (contains):"),
		widget.NewLabel("Birth Place:"), birthPlaceEntry,
		widget.NewLabel("Death Place:"), deathPlaceEntry,
	)
	
	genderSection := container.NewVBox(
		widget.NewLabel("Gender:"), genderSelect,
	)
	
	filtersSection := container.NewVBox(
		widget.NewLabel("Filters:"),
		livingOnlyCheck,
		deceasedOnlyCheck,
		hasMediaCheck,
		hasSourcesCheck,
		hasTodosCheck,
		isBookmarkedCheck,
	)
	
	searchForm := container.NewBorder(
		container.NewVBox(savedSearchesSection, buttonBar),
		nil, nil, nil,
		container.NewVScroll(container.NewVBox(
			nameSection,
			widget.NewSeparator(),
			birthDateSection,
			deathDateSection,
			widget.NewSeparator(),
			placeSection,
			widget.NewSeparator(),
			genderSection,
			widget.NewSeparator(),
			filtersSection,
		)),
	)
	
	// Results section
	resultsSection := container.NewBorder(
		resultsLabel,
		nil, nil, nil,
		container.NewScroll(resultsList),
	)
	
	// Use split for form and results
	split := container.NewHSplit(
		searchForm,
		resultsSection,
	)
	split.SetOffset(0.4) // 40% form, 60% results
	
	searchWindow.SetContent(split)
	searchWindow.Resize(fyne.NewSize(1000, 700))
	searchWindow.Show()
}

// performAdvancedSearch executes the search with given criteria
func performAdvancedSearch(s *store.Store, criteria SearchCriteria) []store.Person {
	// Get all people
	allPeople, err := s.GetPeople()
	if err != nil {
		return nil
	}
	
	var results []store.Person
	
	for _, p := range allPeople {
		// Name filters (check primary name and alternate names)
		if criteria.GivenName != "" {
			matchFound := strings.Contains(strings.ToLower(p.GivenName), strings.ToLower(criteria.GivenName))
			
			// If primary name doesn't match, check alternate names
			if !matchFound {
				altNames, err := s.GetAlternateNames(p.ID)
				if err == nil {
					for _, alt := range altNames {
						if strings.Contains(strings.ToLower(alt.GivenName), strings.ToLower(criteria.GivenName)) {
							matchFound = true
							break
						}
					}
				}
			}
			
			if !matchFound {
				continue
			}
		}
		
		if criteria.Surname != "" {
			matchFound := strings.Contains(strings.ToLower(p.Surname), strings.ToLower(criteria.Surname))
			
			// If primary surname doesn't match, check alternate names
			if !matchFound {
				altNames, err := s.GetAlternateNames(p.ID)
				if err == nil {
					for _, alt := range altNames {
						if strings.Contains(strings.ToLower(alt.Surname), strings.ToLower(criteria.Surname)) {
							matchFound = true
							break
						}
					}
				}
			}
			
			if !matchFound {
				continue
			}
		}
		
		if criteria.PreferredName != "" {
			if !strings.Contains(strings.ToLower(p.PreferredName), strings.ToLower(criteria.PreferredName)) {
				continue
			}
		}
		
		// Gender filter
		if criteria.Gender != "" && p.Gender != criteria.Gender {
			continue
		}
		
		// Living status filters
		if criteria.LivingOnly && !p.IsLiving {
			continue
		}
		if criteria.DeceasedOnly && p.IsLiving {
			continue
		}
		
		// Birth year range - extract year from birth_date string
		if criteria.BirthYearFrom > 0 || criteria.BirthYearTo > 0 {
			birthYear := extractYearFromDate(p.BirthDate)
			if birthYear == 0 {
				continue // Skip if no birth year
			}
			if criteria.BirthYearFrom > 0 && birthYear < criteria.BirthYearFrom {
				continue
			}
			if criteria.BirthYearTo > 0 && birthYear > criteria.BirthYearTo {
				continue
			}
		}
		
		// Death year range - extract year from death_date string
		if criteria.DeathYearFrom > 0 || criteria.DeathYearTo > 0 {
			deathYear := extractYearFromDate(p.DeathDate)
			if deathYear == 0 {
				continue // Skip if no death year
			}
			if criteria.DeathYearFrom > 0 && deathYear < criteria.DeathYearFrom {
				continue
			}
			if criteria.DeathYearTo > 0 && deathYear > criteria.DeathYearTo {
				continue
			}
		}
		
		// Place filters
		if criteria.BirthPlace != "" {
			if !strings.Contains(strings.ToLower(p.BirthPlace), strings.ToLower(criteria.BirthPlace)) {
				continue
			}
		}
		
		if criteria.DeathPlace != "" {
			if !strings.Contains(strings.ToLower(p.DeathPlace), strings.ToLower(criteria.DeathPlace)) {
				continue
			}
		}
		
		// Boolean filters - these require database queries
		if criteria.HasMedia {
			media, _ := s.GetMediaForPerson(p.ID)
			if len(media) == 0 {
				continue
			}
		}
		
		if criteria.HasSources {
			count, _ := s.CountCitationsForPerson(p.ID)
			if count == 0 {
				continue
			}
		}
		
		if criteria.HasTodos {
			count, _ := s.CountPendingTodosForPerson(p.ID)
			if count == 0 {
				continue
			}
		}
		
		if criteria.IsBookmarked && !p.Bookmarked {
			continue
		}
		
		// Person matches all criteria
		results = append(results, p)
	}
	
	// Sort results by surname, then given name
	sort.Slice(results, func(i, j int) bool {
		if results[i].Surname != results[j].Surname {
			return results[i].Surname < results[j].Surname
		}
		return results[i].GivenName < results[j].GivenName
	})
	
	return results
}

// extractYearFromDate extracts the year from a date string (handles various formats)
func extractYearFromDate(dateStr string) int {
	dateStr = strings.TrimSpace(dateStr)
	if dateStr == "" {
		return 0
	}
	
	// Try to find a 4-digit year
	for i := 0; i <= len(dateStr)-4; i++ {
		yearStr := dateStr[i : i+4]
		if year, err := strconv.Atoi(yearStr); err == nil {
			// Check if it's a reasonable year (1000-2100)
			if year >= 1000 && year <= 2100 {
				return year
			}
		}
	}
	
	return 0
}

// showExportDialog displays export options (CSV, JSON, XML)
func showExportDialog(w fyne.Window, results []store.Person) {
	formatSelect := widget.NewSelect([]string{"CSV (Comma Separated)", "JSON (JavaScript Object Notation)", "XML (Extensible Markup Language)"}, func(string) {})
	formatSelect.SetSelected("CSV (Comma Separated)")
	
	content := container.NewVBox(
		widget.NewLabel(fmt.Sprintf("Export %d search results", len(results))),
		widget.NewSeparator(),
		widget.NewLabel("Select export format:"),
		formatSelect,
		widget.NewLabel("\nThe file will be saved with the appropriate extension (.csv, .json, or .xml)"),
	)
	
	dialog.ShowCustomConfirm("Export Search Results", "Export", "Cancel", content, func(ok bool) {
		if !ok {
			return
		}
		
		var defaultName string
		var exportFunc func([]store.Person) (string, string, error)
		
		switch formatSelect.Selected {
		case "JSON (JavaScript Object Notation)":
			defaultName = "search_results.json"
			exportFunc = exportToJSON
		case "XML (Extensible Markup Language)":
			defaultName = "search_results.xml"
			exportFunc = exportToXML
		default: // CSV
			defaultName = "search_results.csv"
			exportFunc = exportToCSV
		}
		
		// Show file save dialog
		fd := dialog.NewFileSave(func(uc fyne.URIWriteCloser, err error) {
			if err != nil || uc == nil {
				return
			}
			defer uc.Close()
			
			content, ext, err := exportFunc(results)
			if err != nil {
				dialog.ShowError(fmt.Errorf("Failed to generate export: %w", err), w)
				return
			}
			
			if _, err := uc.Write([]byte(content)); err != nil {
				dialog.ShowError(fmt.Errorf("Failed to write file: %w", err), w)
				return
			}
			
			dialog.ShowInformation("Export Complete",
				fmt.Sprintf("Exported %d results to %s", len(results), ext), w)
		}, w)
		
		fd.SetFileName(defaultName)
		fd.Show()
	}, w)
}

// exportToCSV generates CSV content
func exportToCSV(results []store.Person) (string, string, error) {
	var csv strings.Builder
	csv.WriteString("Given Name,Surname,Preferred Name,Gender,Birth Date,Birth Place,Death Date,Death Place,Living\n")
	
	for _, p := range results {
		csv.WriteString(fmt.Sprintf("%q,%q,%q,%q,%q,%q,%q,%q,%t\n",
			p.GivenName, p.Surname, p.PreferredName, p.Gender,
			p.BirthDate, p.BirthPlace, p.DeathDate, p.DeathPlace, p.IsLiving))
	}
	
	return csv.String(), "CSV", nil
}

// exportToJSON generates JSON content
func exportToJSON(results []store.Person) (string, string, error) {
	type ExportPerson struct {
		ID            int64  `json:"id"`
		GivenName     string `json:"given_name"`
		Surname       string `json:"surname"`
		PreferredName string `json:"preferred_name,omitempty"`
		Gender        string `json:"gender,omitempty"`
		BirthDate     string `json:"birth_date,omitempty"`
		BirthPlace    string `json:"birth_place,omitempty"`
		DeathDate     string `json:"death_date,omitempty"`
		DeathPlace    string `json:"death_place,omitempty"`
		IsLiving      bool   `json:"is_living"`
	}
	
	exportData := make([]ExportPerson, len(results))
	for i, p := range results {
		exportData[i] = ExportPerson{
			ID:            p.ID,
			GivenName:     p.GivenName,
			Surname:       p.Surname,
			PreferredName: p.PreferredName,
			Gender:        p.Gender,
			BirthDate:     p.BirthDate,
			BirthPlace:    p.BirthPlace,
			DeathDate:     p.DeathDate,
			DeathPlace:    p.DeathPlace,
			IsLiving:      p.IsLiving,
		}
	}
	
	jsonBytes, err := json.MarshalIndent(exportData, "", "  ")
	if err != nil {
		return "", "JSON", err
	}
	
	return string(jsonBytes), "JSON", nil
}

// exportToXML generates XML content
func exportToXML(results []store.Person) (string, string, error) {
	var xml strings.Builder
	xml.WriteString("<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n")
	xml.WriteString("<people>\n")
	
	for _, p := range results {
		xml.WriteString("  <person>\n")
		xml.WriteString(fmt.Sprintf("    <id>%d</id>\n", p.ID))
		if p.GivenName != "" {
			xml.WriteString(fmt.Sprintf("    <given_name>%s</given_name>\n", escapeXML(p.GivenName)))
		}
		if p.Surname != "" {
			xml.WriteString(fmt.Sprintf("    <surname>%s</surname>\n", escapeXML(p.Surname)))
		}
		if p.PreferredName != "" {
			xml.WriteString(fmt.Sprintf("    <preferred_name>%s</preferred_name>\n", escapeXML(p.PreferredName)))
		}
		if p.Gender != "" {
			xml.WriteString(fmt.Sprintf("    <gender>%s</gender>\n", escapeXML(p.Gender)))
		}
		if p.BirthDate != "" {
			xml.WriteString(fmt.Sprintf("    <birth_date>%s</birth_date>\n", escapeXML(p.BirthDate)))
		}
		if p.BirthPlace != "" {
			xml.WriteString(fmt.Sprintf("    <birth_place>%s</birth_place>\n", escapeXML(p.BirthPlace)))
		}
		if p.DeathDate != "" {
			xml.WriteString(fmt.Sprintf("    <death_date>%s</death_date>\n", escapeXML(p.DeathDate)))
		}
		if p.DeathPlace != "" {
			xml.WriteString(fmt.Sprintf("    <death_place>%s</death_place>\n", escapeXML(p.DeathPlace)))
		}
		xml.WriteString(fmt.Sprintf("    <is_living>%t</is_living>\n", p.IsLiving))
		xml.WriteString("  </person>\n")
	}
	
	xml.WriteString("</people>\n")
	return xml.String(), "XML", nil
}

// escapeXML escapes special XML characters
func escapeXML(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, "\"", "&quot;")
	s = strings.ReplaceAll(s, "'", "&apos;")
	return s
}

// loadSearchCriteria loads search criteria from JSON into form fields
func loadSearchCriteria(criteriaJSON string,
	givenNameAutocomplete, surnameAutocomplete, preferredNameAutocomplete *NameAutocompleteContainer,
	birthYearFromEntry, birthYearToEntry, deathYearFromEntry, deathYearToEntry *widget.Entry,
	birthPlaceEntry, deathPlaceEntry *widget.Entry,
	genderSelect *widget.Select,
	livingOnlyCheck, deceasedOnlyCheck, hasMediaCheck, hasSourcesCheck, hasTodosCheck, isBookmarkedCheck *widget.Check) {
	
	var criteria SearchCriteria
	if err := json.Unmarshal([]byte(criteriaJSON), &criteria); err != nil {
		return
	}
	
	givenNameAutocomplete.SetText(criteria.GivenName)
	surnameAutocomplete.SetText(criteria.Surname)
	preferredNameAutocomplete.SetText(criteria.PreferredName)
	
	if criteria.BirthYearFrom > 0 {
		birthYearFromEntry.SetText(strconv.Itoa(criteria.BirthYearFrom))
	} else {
		birthYearFromEntry.SetText("")
	}
	if criteria.BirthYearTo > 0 {
		birthYearToEntry.SetText(strconv.Itoa(criteria.BirthYearTo))
	} else {
		birthYearToEntry.SetText("")
	}
	if criteria.DeathYearFrom > 0 {
		deathYearFromEntry.SetText(strconv.Itoa(criteria.DeathYearFrom))
	} else {
		deathYearFromEntry.SetText("")
	}
	if criteria.DeathYearTo > 0 {
		deathYearToEntry.SetText(strconv.Itoa(criteria.DeathYearTo))
	} else {
		deathYearToEntry.SetText("")
	}
	
	birthPlaceEntry.SetText(criteria.BirthPlace)
	deathPlaceEntry.SetText(criteria.DeathPlace)
	
	switch criteria.Gender {
	case "M":
		genderSelect.SetSelected("Male")
	case "F":
		genderSelect.SetSelected("Female")
	default:
		genderSelect.SetSelected("Any")
	}
	
	livingOnlyCheck.SetChecked(criteria.LivingOnly)
	deceasedOnlyCheck.SetChecked(criteria.DeceasedOnly)
	hasMediaCheck.SetChecked(criteria.HasMedia)
	hasSourcesCheck.SetChecked(criteria.HasSources)
	hasTodosCheck.SetChecked(criteria.HasTodos)
	isBookmarkedCheck.SetChecked(criteria.IsBookmarked)
}

// getCurrentSearchCriteria extracts current search criteria from form fields
func getCurrentSearchCriteria(
	givenNameAutocomplete, surnameAutocomplete, preferredNameAutocomplete *NameAutocompleteContainer,
	birthYearFromEntry, birthYearToEntry, deathYearFromEntry, deathYearToEntry *widget.Entry,
	birthPlaceEntry, deathPlaceEntry *widget.Entry,
	genderSelect *widget.Select,
	livingOnlyCheck, deceasedOnlyCheck, hasMediaCheck, hasSourcesCheck, hasTodosCheck, isBookmarkedCheck *widget.Check) SearchCriteria {
	
	criteria := SearchCriteria{
		GivenName:     strings.TrimSpace(givenNameAutocomplete.GetText()),
		Surname:       strings.TrimSpace(surnameAutocomplete.GetText()),
		PreferredName: strings.TrimSpace(preferredNameAutocomplete.GetText()),
		BirthPlace:    strings.TrimSpace(birthPlaceEntry.Text),
		DeathPlace:    strings.TrimSpace(deathPlaceEntry.Text),
		LivingOnly:    livingOnlyCheck.Checked,
		DeceasedOnly:  deceasedOnlyCheck.Checked,
		HasMedia:      hasMediaCheck.Checked,
		HasSources:    hasSourcesCheck.Checked,
		HasTodos:      hasTodosCheck.Checked,
		IsBookmarked:  isBookmarkedCheck.Checked,
	}
	
	// Parse year ranges
	if birthYearFromEntry.Text != "" {
		if year, err := strconv.Atoi(strings.TrimSpace(birthYearFromEntry.Text)); err == nil {
			criteria.BirthYearFrom = year
		}
	}
	if birthYearToEntry.Text != "" {
		if year, err := strconv.Atoi(strings.TrimSpace(birthYearToEntry.Text)); err == nil {
			criteria.BirthYearTo = year
		}
	}
	if deathYearFromEntry.Text != "" {
		if year, err := strconv.Atoi(strings.TrimSpace(deathYearFromEntry.Text)); err == nil {
			criteria.DeathYearFrom = year
		}
	}
	if deathYearToEntry.Text != "" {
		if year, err := strconv.Atoi(strings.TrimSpace(deathYearToEntry.Text)); err == nil {
			criteria.DeathYearTo = year
		}
	}
	
	// Gender filter
	switch genderSelect.Selected {
	case "Male":
		criteria.Gender = "M"
	case "Female":
		criteria.Gender = "F"
	default:
		criteria.Gender = ""
	}
	
	return criteria
}
