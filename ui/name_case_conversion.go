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

// nameChange represents a proposed name change
type nameChange struct {
	person    store.Person
	oldName   string
	newName   string
	fieldType string
}

// showNameCaseConversionDialog shows the name case conversion dialog
func showNameCaseConversionDialog(w fyne.Window, s *store.Store) {
	// Create a new window for the dialog
	convertWin := fyne.CurrentApp().NewWindow("Global Name Case Conversion")
	convertWin.Resize(fyne.NewSize(800, 600))

	// Conversion options
	conversionSelect := widget.NewSelect([]string{
		"Convert to Proper Case",
		"Convert to UPPERCASE",
		"Convert to lowercase",
	}, nil)
	conversionSelect.SetSelected("Convert to Proper Case")

	// Field selector
	fieldSelect := widget.NewRadioGroup([]string{
		"Given Names only",
		"Surnames only",
		"Both Given Names and Surnames",
		"Preferred Names only",
	}, nil)
	fieldSelect.SetSelected("Both Given Names and Surnames")

	// Preview list
	var previewList *widget.List
	var affectedPeople []store.Person
	var previewChanges []nameChange
	var updatePreview func()

	previewList = widget.NewList(
		func() int { return len(previewChanges) },
		func() fyne.CanvasObject {
			return container.NewVBox(
				widget.NewLabel("Template Name"),
				widget.NewLabel("→ New Name"),
			)
		},
		func(id widget.ListItemID, obj fyne.CanvasObject) {
			vbox := obj.(*fyne.Container)
			oldLabel := vbox.Objects[0].(*widget.Label)
			newLabel := vbox.Objects[1].(*widget.Label)

			change := previewChanges[id]
			oldLabel.SetText(fmt.Sprintf("%s: %s", formatPersonName(change.person), change.oldName))
			newLabel.SetText(fmt.Sprintf("  → %s", change.newName))
		},
	)

	updatePreview = func() {
		previewChanges = []nameChange{}
		people, err := s.GetPeople()
		if err != nil {
			return
		}

		for _, person := range people {
			changes := getNameChangesForPerson(person, conversionSelect.Selected, fieldSelect.Selected)
			previewChanges = append(previewChanges, changes...)
		}

		affectedPeople = people
		previewList.Refresh()
	}

	// Preview button
	previewBtn := widget.NewButton("Preview Changes", func() {
		updatePreview()
	})

	// Apply button
	applyBtn := widget.NewButton("Apply Changes", func() {
		if len(previewChanges) == 0 {
			dialog.ShowInformation("No Changes", "No changes to apply. Click 'Preview Changes' first.", convertWin)
			return
		}

		dialog.ShowConfirm("Confirm Conversion",
			fmt.Sprintf("Convert names for %d records?\n\nThis operation cannot be undone.",
				len(previewChanges)),
			func(confirmed bool) {
				if !confirmed {
					return
				}

				// Perform conversion
				count, err := performNameConversion(s, affectedPeople, conversionSelect.Selected, fieldSelect.Selected)

				if err != nil {
					dialog.ShowError(fmt.Errorf("Conversion failed: %v", err), convertWin)
					return
				}

				dialog.ShowInformation("Conversion Complete",
					fmt.Sprintf("Successfully converted names for %d records.", count),
					convertWin)

				// Clear preview
				previewChanges = []nameChange{}
				previewList.Refresh()
			}, convertWin)
	})

	// Layout
	header := widget.NewLabelWithStyle("Global Name Case Conversion", fyne.TextAlignCenter, fyne.TextStyle{Bold: true})
	description := widget.NewLabel("Convert names to proper case, UPPERCASE, or lowercase across all records.\n\nProper Case handles special cases like McDonald, O'Brien, and van der Berg.")
	description.Wrapping = fyne.TextWrapWord

	options := container.NewVBox(
		widget.NewLabel("Conversion Type:"),
		conversionSelect,
		widget.NewLabel("Apply to:"),
		fieldSelect,
	)

	buttons := container.NewHBox(previewBtn, applyBtn)

	previewLabel := widget.NewLabelWithStyle(
		"Preview (showing name changes that will be applied):",
		fyne.TextAlignLeading,
		fyne.TextStyle{Bold: true},
	)
	previewContainer := container.NewBorder(previewLabel, nil, nil, nil, container.NewVScroll(previewList))

	content := container.NewBorder(
		container.NewVBox(header, description, options, buttons, widget.NewSeparator()),
		nil, nil, nil,
		previewContainer,
	)

	convertWin.SetContent(content)
	convertWin.Show()
}

// getNameChangesForPerson returns the name changes that would be applied to a person
func getNameChangesForPerson(person store.Person, conversionType, field string) []nameChange {
	changes := []nameChange{}

	switch field {
	case "Given Names only":
		if person.GivenName != "" {
			newName := convertName(person.GivenName, conversionType)
			if newName != person.GivenName {
				changes = append(changes, nameChange{
					person:    person,
					oldName:   person.GivenName,
					newName:   newName,
					fieldType: "Given Name",
				})
			}
		}
	case "Surnames only":
		if person.Surname != "" {
			newName := convertName(person.Surname, conversionType)
			if newName != person.Surname {
				changes = append(changes, nameChange{
					person:    person,
					oldName:   person.Surname,
					newName:   newName,
					fieldType: "Surname",
				})
			}
		}
	case "Both Given Names and Surnames":
		if person.GivenName != "" {
			newGiven := convertName(person.GivenName, conversionType)
			if newGiven != person.GivenName {
				changes = append(changes, nameChange{
					person:    person,
					oldName:   fmt.Sprintf("Given: %s", person.GivenName),
					newName:   newGiven,
					fieldType: "Given Name",
				})
			}
		}
		if person.Surname != "" {
			newSurname := convertName(person.Surname, conversionType)
			if newSurname != person.Surname {
				changes = append(changes, nameChange{
					person:    person,
					oldName:   fmt.Sprintf("Surname: %s", person.Surname),
					newName:   newSurname,
					fieldType: "Surname",
				})
			}
		}
	case "Preferred Names only":
		if person.PreferredName != "" {
			newName := convertName(person.PreferredName, conversionType)
			if newName != person.PreferredName {
				changes = append(changes, nameChange{
					person:    person,
					oldName:   person.PreferredName,
					newName:   newName,
					fieldType: "Preferred Name",
				})
			}
		}
	}

	return changes
}

// convertName converts a name according to the specified type
func convertName(name, conversionType string) string {
	switch conversionType {
	case "Convert to Proper Case":
		return toProperCase(name)
	case "Convert to UPPERCASE":
		return strings.ToUpper(name)
	case "Convert to lowercase":
		return strings.ToLower(name)
	default:
		return name
	}
}

// toProperCase converts a name to proper case, handling special cases
func toProperCase(name string) string {
	if name == "" {
		return name
	}

	// Split by spaces
	words := strings.Fields(name)
	for i, word := range words {
		words[i] = capitalizeWord(word)
	}

	return strings.Join(words, " ")
}

// capitalizeWord capitalizes a single word, handling special cases
func capitalizeWord(word string) string {
	if word == "" {
		return word
	}

	lower := strings.ToLower(word)
	
	// Handle Roman numerals (I, II, III, IV, V, VI, VII, VIII, IX, X, etc.)
	// These should stay uppercase
	romanNumerals := []string{"i", "ii", "iii", "iv", "v", "vi", "vii", "viii", "ix", "x", 
		"xi", "xii", "xiii", "xiv", "xv", "xvi", "xvii", "xviii", "xix", "xx"}
	for _, roman := range romanNumerals {
		if lower == roman {
			return strings.ToUpper(word)
		}
	}
	
	// Handle common suffixes (Jr., Sr., Jr, Sr, Esq., Esq, etc.)
	// These should be capitalized with proper format
	suffixes := map[string]string{
		"jr":   "Jr.",
		"jr.":  "Jr.",
		"sr":   "Sr.",
		"sr.":  "Sr.",
		"esq":  "Esq.",
		"esq.": "Esq.",
		"phd":  "PhD",
		"phd.": "PhD",
		"md":   "MD",
		"md.":  "MD",
		"dds":  "DDS",
		"dds.": "DDS",
	}
	if replacement, exists := suffixes[lower]; exists {
		return replacement
	}

	// Handle special prefixes
	specialPrefixes := map[string]string{
		"mc":  "Mc",
		"mac": "Mac",
		"o'":  "O'",
		"van": "van",
		"von": "von",
		"de":  "de",
		"der": "der",
		"la":  "la",
		"le":  "le",
	}

	// Check for special prefixes
	for prefix, replacement := range specialPrefixes {
		if strings.HasPrefix(lower, prefix) {
			rest := word[len(prefix):]
			if len(rest) > 0 {
				// Special handling for Mc/Mac - capitalize next letter
				if prefix == "mc" || prefix == "mac" {
					return replacement + strings.ToUpper(rest[:1]) + strings.ToLower(rest[1:])
				}
				// For van, von, de, etc. - keep lowercase prefix, capitalize rest
				if prefix == "van" || prefix == "von" || prefix == "de" || prefix == "der" || prefix == "la" || prefix == "le" {
					return replacement + " " + strings.ToUpper(rest[:1]) + strings.ToLower(rest[1:])
				}
				// For O' - capitalize after apostrophe
				if prefix == "o'" {
					return replacement + strings.ToUpper(rest[:1]) + strings.ToLower(rest[1:])
				}
			}
		}
	}

	// Default: capitalize first letter, lowercase rest
	return strings.ToUpper(word[:1]) + strings.ToLower(word[1:])
}

// performNameConversion performs the actual name conversion
func performNameConversion(s *store.Store, people []store.Person, conversionType, field string) (int, error) {
	count := 0

	for _, person := range people {
		personCopy := person
		modified := false

		switch field {
		case "Given Names only":
			if person.GivenName != "" {
				newName := convertName(person.GivenName, conversionType)
				if newName != person.GivenName {
					personCopy.GivenName = newName
					modified = true
				}
			}
		case "Surnames only":
			if person.Surname != "" {
				newName := convertName(person.Surname, conversionType)
				if newName != person.Surname {
					personCopy.Surname = newName
					modified = true
				}
			}
		case "Both Given Names and Surnames":
			if person.GivenName != "" {
				newGiven := convertName(person.GivenName, conversionType)
				if newGiven != person.GivenName {
					personCopy.GivenName = newGiven
					modified = true
				}
			}
			if person.Surname != "" {
				newSurname := convertName(person.Surname, conversionType)
				if newSurname != person.Surname {
					personCopy.Surname = newSurname
					modified = true
				}
			}
		case "Preferred Names only":
			if person.PreferredName != "" {
				newName := convertName(person.PreferredName, conversionType)
				if newName != person.PreferredName {
					personCopy.PreferredName = newName
					modified = true
				}
			}
		}

		if modified {
			err := s.UpdatePerson(&personCopy)
			if err != nil {
				return count, err
			}
			count++
		}
	}

	return count, nil
}
