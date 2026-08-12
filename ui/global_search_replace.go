package ui

import (
	"fmt"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"

	"genealogy/store"
)

var globalSearchReplaceWindow fyne.Window

// showGlobalSearchReplaceDialog shows the global search and replace dialog
func showGlobalSearchReplaceDialog(w fyne.Window, s *store.Store) {
	// If window already exists and is visible, just bring it to front
	if globalSearchReplaceWindow != nil && globalSearchReplaceWindow.Content().Visible() {
		globalSearchReplaceWindow.Show()
		globalSearchReplaceWindow.RequestFocus()
		return
	}

	// Create a new window for the dialog
	searchWin := fyne.CurrentApp().NewWindow("Global Search and Replace")
	searchWin.Resize(fyne.NewSize(800, 600))
	globalSearchReplaceWindow = searchWin
	
	// Register for window lifecycle management
	RegisterSecondaryWindow(searchWin)

	// Search and replace fields
	searchEntry := widget.NewEntry()
	searchEntry.SetPlaceHolder("Enter text to search for...")

	replaceEntry := widget.NewEntry()
	replaceEntry.SetPlaceHolder("Enter replacement text...")

	// Field selector
	fieldSelect := widget.NewSelect([]string{
		"All Place Fields",
		"Birth Place",
		"Death Place",
		"Marriage Place",
		"Address",
		"City",
		"State",
		"Country",
		"Notes",
	}, nil)
	fieldSelect.SetSelected("All Place Fields")

	// Case sensitive checkbox
	caseSensitive := widget.NewCheck("Case sensitive", nil)

	// Results list
	var resultsList *widget.List
	var matchedPeople []store.Person
	var resultsLabel *widget.Label
	var updateList func()

	resultsList = widget.NewList(
		func() int { return len(matchedPeople) },
		func() fyne.CanvasObject {
			return container.NewHBox(
				widget.NewCheck("", nil),
				widget.NewLabel("Template"),
			)
		},
		func(id widget.ListItemID, obj fyne.CanvasObject) {
			hbox := obj.(*fyne.Container)
			checkbox := hbox.Objects[0].(*widget.Check)
			label := hbox.Objects[1].(*widget.Label)

			person := matchedPeople[id]
			checkbox.SetChecked(true)

			matchText := getMatchInfo(s, person, fieldSelect.Selected, searchEntry.Text)
			label.SetText(fmt.Sprintf("%s - %s", formatPersonName(person), matchText))
		},
	)

	updateList = func() {
		matchedPeople = findMatches(s, searchEntry.Text, fieldSelect.Selected, caseSensitive.Checked)
		resultsList.Refresh()
		resultsLabel.SetText(fmt.Sprintf("Results: %d matches found", len(matchedPeople)))
	}

	// Search button
	searchBtn := widget.NewButton("Search", func() {
		if searchEntry.Text == "" {
			dialog.ShowInformation("Search Required", "Please enter text to search for.", searchWin)
			return
		}
		updateList()
	})

	// Replace button
	replaceBtn := widget.NewButton("Replace Selected", func() {
		if searchEntry.Text == "" || replaceEntry.Text == "" {
			dialog.ShowInformation("Input Required", "Please enter both search and replace text.", searchWin)
			return
		}

		// Count selected items
		selectedCount := len(matchedPeople) // All are selected by default in this simple version

		dialog.ShowConfirm("Confirm Replace",
			fmt.Sprintf("Replace '%s' with '%s' in %d records?\n\nThis operation cannot be undone.",
				searchEntry.Text, replaceEntry.Text, selectedCount),
			func(confirmed bool) {
				if !confirmed {
					return
				}

				// Perform replacement
				count, err := performReplacement(s, matchedPeople, searchEntry.Text, replaceEntry.Text,
					fieldSelect.Selected, caseSensitive.Checked)

				if err != nil {
					dialog.ShowError(fmt.Errorf("Replacement failed: %v", err), searchWin)
					return
				}

				dialog.ShowInformation("Replacement Complete",
					fmt.Sprintf("Successfully replaced text in %d records.", count),
					searchWin)

				// Refresh list
				updateList()
			}, searchWin)
	})

	// Layout
	header := widget.NewLabelWithStyle("Global Search and Replace", fyne.TextAlignCenter, fyne.TextStyle{Bold: true})
	description := widget.NewLabel("Search for text across all records and replace it with new text.")
	description.Wrapping = fyne.TextWrapWord

	searchForm := container.NewVBox(
		widget.NewLabel("Search for:"),
		searchEntry,
		widget.NewLabel("Replace with:"),
		replaceEntry,
		widget.NewLabel("In field:"),
		fieldSelect,
		caseSensitive,
	)

	buttons := container.NewHBox(searchBtn, replaceBtn)

	resultsLabel = widget.NewLabel("Results: (click Search to find matches)")
	resultsContainer := container.NewBorder(resultsLabel, nil, nil, nil, container.NewVScroll(resultsList))

	content := container.NewBorder(
		container.NewVBox(header, description, searchForm, buttons, widget.NewSeparator()),
		nil, nil, nil,
		resultsContainer,
	)

	searchWin.SetContent(content)
	
	// Hide window instead of closing it, so we can reuse it
	searchWin.SetCloseIntercept(func() {
		searchWin.Hide()
	})
	
	searchWin.Show()
	searchWin.RequestFocus() // Bring window to front
}

// findMatches finds all people matching the search criteria
func findMatches(s *store.Store, searchText, field string, caseSensitive bool) []store.Person {
	if searchText == "" {
		return []store.Person{}
	}

	people, err := s.GetPeople()
	if err != nil {
		return []store.Person{}
	}

	var matches []store.Person
	for _, person := range people {
		if personMatchesSearch(s, person, searchText, field, caseSensitive) {
			matches = append(matches, person)
		}
	}

	return matches
}

// personMatchesSearch checks if a person matches the search criteria
func personMatchesSearch(s *store.Store, person store.Person, searchText, field string, caseSensitive bool) bool {
	search := searchText
	if !caseSensitive {
		search = strings.ToLower(search)
	}

	var fieldsToSearch []string

	switch field {
	case "Birth Place":
		fieldsToSearch = []string{person.BirthPlace}
	case "Death Place":
		fieldsToSearch = []string{person.DeathPlace}
	case "Marriage Place":
		// Get all relationships for this person and check marriage places
		rels, err := s.GetRelationshipsForPerson(person.ID)
		if err == nil {
			for _, rel := range rels {
				if rel.MarriagePlace != "" {
					fieldsToSearch = append(fieldsToSearch, rel.MarriagePlace)
				}
			}
		}
	case "Address":
		fieldsToSearch = []string{person.Address}
	case "City":
		fieldsToSearch = []string{person.City}
	case "State":
		fieldsToSearch = []string{person.State}
	case "Country":
		fieldsToSearch = []string{person.Country}
	case "Notes":
		fieldsToSearch = []string{person.Notes}
	case "All Place Fields":
		fieldsToSearch = []string{person.BirthPlace, person.DeathPlace, person.Address, person.City, person.State, person.Country}
		// Also include marriage places
		rels, err := s.GetRelationshipsForPerson(person.ID)
		if err == nil {
			for _, rel := range rels {
				if rel.MarriagePlace != "" {
					fieldsToSearch = append(fieldsToSearch, rel.MarriagePlace)
				}
			}
		}
	}

	for _, fieldValue := range fieldsToSearch {
		compareValue := fieldValue
		if !caseSensitive {
			compareValue = strings.ToLower(compareValue)
		}
		if strings.Contains(compareValue, search) {
			return true
		}
	}

	return false
}

// getMatchInfo returns information about what matched
func getMatchInfo(s *store.Store, person store.Person, field, searchText string) string {
	switch field {
	case "Birth Place":
		return fmt.Sprintf("Birth Place: %s", person.BirthPlace)
	case "Death Place":
		return fmt.Sprintf("Death Place: %s", person.DeathPlace)
	case "Marriage Place":
		// Find matching marriage places
		rels, err := s.GetRelationshipsForPerson(person.ID)
		if err == nil {
			for _, rel := range rels {
				if rel.MarriagePlace != "" && strings.Contains(strings.ToLower(rel.MarriagePlace), strings.ToLower(searchText)) {
					// Get spouse name
					spouseID := rel.ObjectID
					if spouseID == person.ID {
						spouseID = rel.SubjectID
					}
					spouse, err := s.GetPersonByID(spouseID)
					spouseName := "Unknown"
					if err == nil && spouse != nil {
						spouseName = formatPersonName(*spouse)
					}
					return fmt.Sprintf("Marriage to %s: %s", spouseName, rel.MarriagePlace)
				}
			}
		}
	case "Address":
		return fmt.Sprintf("Address: %s", person.Address)
	case "City":
		return fmt.Sprintf("City: %s", person.City)
	case "State":
		return fmt.Sprintf("State: %s", person.State)
	case "Country":
		return fmt.Sprintf("Country: %s", person.Country)
	case "Notes":
		// Truncate long notes
		notes := person.Notes
		if len(notes) > 50 {
			notes = notes[:50] + "..."
		}
		return fmt.Sprintf("Notes: %s", notes)
	case "All Place Fields":
		// Show which field matched
		fields := []struct {
			name  string
			value string
		}{
			{"Birth Place", person.BirthPlace},
			{"Death Place", person.DeathPlace},
			{"Address", person.Address},
			{"City", person.City},
			{"State", person.State},
			{"Country", person.Country},
		}
		for _, f := range fields {
			if strings.Contains(strings.ToLower(f.value), strings.ToLower(searchText)) {
				return fmt.Sprintf("%s: %s", f.name, f.value)
			}
		}
		// Check marriage places
		rels, err := s.GetRelationshipsForPerson(person.ID)
		if err == nil {
			for _, rel := range rels {
				if rel.MarriagePlace != "" && strings.Contains(strings.ToLower(rel.MarriagePlace), strings.ToLower(searchText)) {
					spouseID := rel.ObjectID
					if spouseID == person.ID {
						spouseID = rel.SubjectID
					}
					spouse, err := s.GetPersonByID(spouseID)
					spouseName := "Unknown"
					if err == nil && spouse != nil {
						spouseName = formatPersonName(*spouse)
					}
					return fmt.Sprintf("Marriage to %s: %s", spouseName, rel.MarriagePlace)
				}
			}
		}
	}
	return ""
}

// performReplacement performs the actual replacement operation
func performReplacement(s *store.Store, people []store.Person, searchText, replaceText, field string, caseSensitive bool) (int, error) {
	count := 0

	for _, person := range people {
		modified := false
		personCopy := person

		switch field {
		case "Birth Place":
			personCopy.BirthPlace = replaceInString(person.BirthPlace, searchText, replaceText, caseSensitive)
			modified = personCopy.BirthPlace != person.BirthPlace
		case "Death Place":
			personCopy.DeathPlace = replaceInString(person.DeathPlace, searchText, replaceText, caseSensitive)
			modified = personCopy.DeathPlace != person.DeathPlace
		case "Marriage Place":
			// Update marriage places in relationships
			rels, err := s.GetRelationshipsForPerson(person.ID)
			if err != nil {
				return count, err
			}
			for _, rel := range rels {
				if rel.MarriagePlace != "" && strings.Contains(strings.ToLower(rel.MarriagePlace), strings.ToLower(searchText)) {
					originalPlace := rel.MarriagePlace
					rel.MarriagePlace = replaceInString(rel.MarriagePlace, searchText, replaceText, caseSensitive)
					if rel.MarriagePlace != originalPlace {
						err := s.UpdateRelationship(&rel)
						if err != nil {
							return count, err
						}
						modified = true
					}
				}
			}
		case "Address":
			personCopy.Address = replaceInString(person.Address, searchText, replaceText, caseSensitive)
			modified = personCopy.Address != person.Address
		case "City":
			personCopy.City = replaceInString(person.City, searchText, replaceText, caseSensitive)
			modified = personCopy.City != person.City
		case "State":
			personCopy.State = replaceInString(person.State, searchText, replaceText, caseSensitive)
			modified = personCopy.State != person.State
		case "Country":
			personCopy.Country = replaceInString(person.Country, searchText, replaceText, caseSensitive)
			modified = personCopy.Country != person.Country
		case "Notes":
			personCopy.Notes = replaceInString(person.Notes, searchText, replaceText, caseSensitive)
			modified = personCopy.Notes != person.Notes
		case "All Place Fields":
			personCopy.BirthPlace = replaceInString(person.BirthPlace, searchText, replaceText, caseSensitive)
			personCopy.DeathPlace = replaceInString(person.DeathPlace, searchText, replaceText, caseSensitive)
			personCopy.Address = replaceInString(person.Address, searchText, replaceText, caseSensitive)
			personCopy.City = replaceInString(person.City, searchText, replaceText, caseSensitive)
			personCopy.State = replaceInString(person.State, searchText, replaceText, caseSensitive)
			personCopy.Country = replaceInString(person.Country, searchText, replaceText, caseSensitive)
			modified = personCopy.BirthPlace != person.BirthPlace ||
				personCopy.DeathPlace != person.DeathPlace ||
				personCopy.Address != person.Address ||
				personCopy.City != person.City ||
				personCopy.State != person.State ||
				personCopy.Country != person.Country
			
			// Also update marriage places
			rels, err := s.GetRelationshipsForPerson(person.ID)
			if err != nil {
				return count, err
			}
			for _, rel := range rels {
				if rel.MarriagePlace != "" && strings.Contains(strings.ToLower(rel.MarriagePlace), strings.ToLower(searchText)) {
					originalPlace := rel.MarriagePlace
					rel.MarriagePlace = replaceInString(rel.MarriagePlace, searchText, replaceText, caseSensitive)
					if rel.MarriagePlace != originalPlace {
						err := s.UpdateRelationship(&rel)
						if err != nil {
							return count, err
						}
						modified = true
					}
				}
			}
		}

		if modified && field != "Marriage Place" {
			// For Marriage Place, we already updated the relationships directly
			err := s.UpdatePerson(&personCopy)
			if err != nil {
				return count, err
			}
			count++
		} else if modified && field == "Marriage Place" {
			count++
		}
	}

	return count, nil
}

// replaceInString performs case-sensitive or case-insensitive replacement
func replaceInString(original, search, replace string, caseSensitive bool) string {
	if original == "" || search == "" {
		return original
	}

	if caseSensitive {
		return strings.ReplaceAll(original, search, replace)
	}

	// Case-insensitive replacement
	result := original
	lowerOriginal := strings.ToLower(original)
	lowerSearch := strings.ToLower(search)

	for {
		index := strings.Index(lowerOriginal, lowerSearch)
		if index == -1 {
			break
		}

		// Replace preserving original case structure
		result = result[:index] + replace + result[index+len(search):]
		lowerOriginal = lowerOriginal[:index] + strings.Repeat(" ", len(replace)) + lowerOriginal[index+len(search):]
	}

	return result
}
