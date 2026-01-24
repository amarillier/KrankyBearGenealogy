package ui

import (
	"fmt"
	"genealogy/store"
	"sort"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
)

var relationshipCalculatorDialog fyne.Window

// showRelationshipCalculator displays a dialog to calculate relationships between any two people
func showRelationshipCalculator(w fyne.Window, s *store.Store, navigateFunc func(int64)) {
	// Check if already open
	if relationshipCalculatorDialog != nil {
		relationshipCalculatorDialog.RequestFocus()
		relationshipCalculatorDialog.Show()
		return
	}

	// Get all people
	people, err := s.GetPeople()
	if err != nil {
		dialog.ShowError(fmt.Errorf("Failed to load people: %w", err), w)
		return
	}

	if len(people) < 2 {
		dialog.ShowInformation("Relationship Calculator", "Need at least 2 people in the database", w)
		return
	}

	// Sort people alphabetically
	sort.Slice(people, func(i, j int) bool {
		if people[i].Surname != people[j].Surname {
			return strings.ToLower(people[i].Surname) < strings.ToLower(people[j].Surname)
		}
		return strings.ToLower(people[i].GivenName) < strings.ToLower(people[j].GivenName)
	})

	// Create person options
	personOptions := make([]string, len(people))
	personMap := make(map[string]int64)
	for i, p := range people {
		label := fmt.Sprintf("%s %s", p.GivenName, p.Surname)
		if p.BirthDate != "" {
			label += fmt.Sprintf(" (b. %s)", p.BirthDate)
		}
		personOptions[i] = label
		personMap[label] = p.ID
	}

	content := container.NewVBox()

	title := widget.NewLabelWithStyle("Relationship Calculator", fyne.TextAlignCenter, fyne.TextStyle{Bold: true})
	content.Add(title)
	content.Add(widget.NewLabel("Calculate the relationship between any two people"))
	content.Add(widget.NewSeparator())

	// Person 1 selection
	var person1ID int64
	person1Label := widget.NewLabel("Person 1: (none selected)")
	person1Select := widget.NewSelect(personOptions, func(selected string) {
		if selected != "" && !strings.HasPrefix(selected, "Found ") {
			person1ID = personMap[selected]
			person1Label.SetText("Person 1: " + selected)
		}
	})
	person1Select.PlaceHolder = "Select first person..."

	// Search for Person 1
	person1SearchEntry := widget.NewEntry()
	person1SearchEntry.SetPlaceHolder("Type to search, then click dropdown below...")
	person1SearchEntry.OnChanged = func(query string) {
		// Clear any previous selection when user starts typing
		person1ID = 0
		person1Label.SetText("Person 1: (none selected)")
		
		if query == "" {
			person1Select.Options = personOptions
			person1Select.SetSelected("")
			person1Select.PlaceHolder = "Select first person..."
		} else {
			filtered := []string{}
			queryLower := strings.ToLower(query)
			for _, opt := range personOptions {
				if strings.Contains(strings.ToLower(opt), queryLower) {
					filtered = append(filtered, opt)
				}
			}
			person1Select.Options = filtered
			// Auto-select if only one match
			if len(filtered) == 1 {
				person1Select.SetSelected(filtered[0])
			} else if len(filtered) > 1 {
				// Clear selection and show helpful placeholder
				person1Select.ClearSelected()
				person1Select.PlaceHolder = fmt.Sprintf("📋 Found %d matches - click here to select", len(filtered))
			} else {
				person1Select.ClearSelected()
				person1Select.PlaceHolder = "❌ No matches found"
			}
		}
		person1Select.Refresh()
	}

	content.Add(widget.NewLabel("First Person:"))
	content.Add(person1SearchEntry)
	content.Add(person1Select)
	content.Add(person1Label)
	content.Add(widget.NewSeparator())

	// Person 2 selection
	var person2ID int64
	person2Label := widget.NewLabel("Person 2: (none selected)")
	person2Select := widget.NewSelect(personOptions, func(selected string) {
		if selected != "" && !strings.HasPrefix(selected, "Found ") {
			person2ID = personMap[selected]
			person2Label.SetText("Person 2: " + selected)
		}
	})
	person2Select.PlaceHolder = "Select second person..."

	// Search for Person 2
	person2SearchEntry := widget.NewEntry()
	person2SearchEntry.SetPlaceHolder("Type to search, then click dropdown below...")
	person2SearchEntry.OnChanged = func(query string) {
		// Clear any previous selection when user starts typing
		person2ID = 0
		person2Label.SetText("Person 2: (none selected)")
		
		if query == "" {
			person2Select.Options = personOptions
			person2Select.SetSelected("")
			person2Select.PlaceHolder = "Select second person..."
		} else {
			filtered := []string{}
			queryLower := strings.ToLower(query)
			for _, opt := range personOptions {
				if strings.Contains(strings.ToLower(opt), queryLower) {
					filtered = append(filtered, opt)
				}
			}
			person2Select.Options = filtered
			// Auto-select if only one match
			if len(filtered) == 1 {
				person2Select.SetSelected(filtered[0])
			} else if len(filtered) > 1 {
				// Clear selection and show helpful placeholder
				person2Select.ClearSelected()
				person2Select.PlaceHolder = fmt.Sprintf("📋 Found %d matches - click here to select", len(filtered))
			} else {
				person2Select.ClearSelected()
				person2Select.PlaceHolder = "❌ No matches found"
			}
		}
		person2Select.Refresh()
	}
	
	// Set up Enter key handler for person1 search
	person1SearchEntry.OnSubmitted = func(query string) {
		if query != "" && len(person1Select.Options) > 0 {
			// If only one match, it's already auto-selected
			// If multiple matches, user needs to click the dropdown
			// This just provides feedback
			if len(person1Select.Options) == 1 {
				// Already selected by OnChanged
			} else if len(person1Select.Options) > 1 {
				// Multiple matches - user should click dropdown to select
				person1Label.SetText(fmt.Sprintf("Person 1: Found %d matches - click dropdown to select", len(person1Select.Options)))
			}
		}
	}

	content.Add(widget.NewLabel("Second Person:"))
	content.Add(person2SearchEntry)
	content.Add(person2Select)
	content.Add(person2Label)
	content.Add(widget.NewSeparator())

	// Result area
	resultContainer := container.NewVBox()

	// Calculate button (forward declare for person2SearchEntry.OnSubmitted)
	var calculateBtn *widget.Button
	calculateBtn = widget.NewButton("Calculate Relationship", func() {
		if person1ID == 0 || person2ID == 0 {
			dialog.ShowInformation("Selection Required", "Please select both people", w)
			return
		}

		if person1ID == person2ID {
			dialog.ShowInformation("Same Person", "Please select two different people", w)
			return
		}

		// Get person details
		p1, err1 := s.GetPersonByID(person1ID)
		p2, err2 := s.GetPersonByID(person2ID)
		if err1 != nil || err2 != nil {
			dialog.ShowError(fmt.Errorf("Failed to load person details"), w)
			return
		}

		// Calculate relationship from person1 to person2
		rel1to2 := calculateRelationship(s, person1ID, person2ID)
		// Calculate reverse relationship
		rel2to1 := calculateRelationship(s, person2ID, person1ID)

		resultContainer.Objects = nil

		// Display results
		resultTitle := widget.NewLabelWithStyle("Relationship Results", fyne.TextAlignCenter, fyne.TextStyle{Bold: true})
		resultContainer.Add(resultTitle)

		if rel1to2 != "" {
			// Show relationship
			relationshipText := fmt.Sprintf("📊 %s %s is the %s of %s %s",
				p2.GivenName, p2.Surname, rel1to2, p1.GivenName, p1.Surname)
			relationshipLabel := widget.NewLabelWithStyle(relationshipText, fyne.TextAlignCenter, fyne.TextStyle{Bold: true})
			resultContainer.Add(relationshipLabel)

			if rel2to1 != "" && rel2to1 != rel1to2 {
				reverseText := fmt.Sprintf("(Inverse: %s %s is the %s of %s %s)",
					p1.GivenName, p1.Surname, rel2to1, p2.GivenName, p2.Surname)
				resultContainer.Add(widget.NewLabel(reverseText))
			}

			resultContainer.Add(widget.NewSeparator())

			// Find common ancestors
			commonAncestors := findCommonAncestors(s, person1ID, person2ID)
			if len(commonAncestors) > 0 {
				resultContainer.Add(widget.NewLabelWithStyle("Common Ancestors:", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}))
				for _, ancestor := range commonAncestors {
					ancestorCopy := ancestor
					ancestorBtn := widget.NewButton(
						fmt.Sprintf("• %s %s (b. %s)",
							ancestor.GivenName, ancestor.Surname, ancestor.BirthDate),
						func() {
							navigateFunc(ancestorCopy.ID)
							relationshipCalculatorDialog.Hide()
						})
					resultContainer.Add(ancestorBtn)
				}
				resultContainer.Add(widget.NewSeparator())
			}

			// Show lines of descent
			linesOfDescent := getLineOfDescent(s, person1ID, person2ID, commonAncestors)
			if linesOfDescent != "" {
				resultContainer.Add(widget.NewLabelWithStyle("Line of Descent:", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}))
				resultContainer.Add(widget.NewLabel(linesOfDescent))
			}

		} else {
			// No direct relationship found
			resultContainer.Add(widget.NewLabelWithStyle("❌ No direct relationship found", fyne.TextAlignCenter, fyne.TextStyle{Bold: true}))
			resultContainer.Add(widget.NewLabel(fmt.Sprintf("%s %s and %s %s do not appear to be related by blood or marriage.",
				p1.GivenName, p1.Surname, p2.GivenName, p2.Surname)))

			// Check for similar names (possible duplicates)
			similarToP1 := findSimilarNames(s, p1, person1ID)
			similarToP2 := findSimilarNames(s, p2, person2ID)
			
			if len(similarToP1) > 0 || len(similarToP2) > 0 {
				resultContainer.Add(widget.NewSeparator())
				resultContainer.Add(widget.NewLabelWithStyle("💡 Did you mean a different person?", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}))
				resultContainer.Add(widget.NewLabel("Found people with similar names who might be who you're looking for:"))
				
				if len(similarToP1) > 0 {
					resultContainer.Add(widget.NewLabel(fmt.Sprintf("\nInstead of %s %s:", p1.GivenName, p1.Surname)))
					for _, similar := range similarToP1 {
						similarCopy := similar
						birthInfo := ""
						if similar.BirthDate != "" {
							birthInfo = fmt.Sprintf(" (b. %s)", similar.BirthDate)
						}
						btn := widget.NewButton(
							fmt.Sprintf("  → %s %s%s", similar.GivenName, similar.Surname, birthInfo),
							func() {
								// Update person1 selection
								person1ID = similarCopy.ID
								person1Label.SetText(fmt.Sprintf("Person 1: %s %s (b. %s)", 
									similarCopy.GivenName, similarCopy.Surname, similarCopy.BirthDate))
								// Auto-calculate with new selection
								calculateBtn.OnTapped()
							})
						resultContainer.Add(btn)
					}
				}
				
				if len(similarToP2) > 0 {
					resultContainer.Add(widget.NewLabel(fmt.Sprintf("\nInstead of %s %s:", p2.GivenName, p2.Surname)))
					for _, similar := range similarToP2 {
						similarCopy := similar
						birthInfo := ""
						if similar.BirthDate != "" {
							birthInfo = fmt.Sprintf(" (b. %s)", similar.BirthDate)
						}
						btn := widget.NewButton(
							fmt.Sprintf("  → %s %s%s", similar.GivenName, similar.Surname, birthInfo),
							func() {
								// Update person2 selection
								person2ID = similarCopy.ID
								person2Label.SetText(fmt.Sprintf("Person 2: %s %s (b. %s)", 
									similarCopy.GivenName, similarCopy.Surname, similarCopy.BirthDate))
								// Auto-calculate with new selection
								calculateBtn.OnTapped()
							})
						resultContainer.Add(btn)
					}
				}
			}

			// Still show common ancestors if any
			commonAncestors := findCommonAncestors(s, person1ID, person2ID)
			if len(commonAncestors) > 0 {
				resultContainer.Add(widget.NewSeparator())
				resultContainer.Add(widget.NewLabelWithStyle("Note: Common Ancestors Found:", fyne.TextAlignLeading, fyne.TextStyle{Italic: true}))
				resultContainer.Add(widget.NewLabel("(The relationship calculation may be incomplete for distant relatives)"))
				for _, ancestor := range commonAncestors {
					ancestorCopy := ancestor
					ancestorBtn := widget.NewButton(
						fmt.Sprintf("• %s %s (b. %s)",
							ancestor.GivenName, ancestor.Surname, ancestor.BirthDate),
						func() {
							navigateFunc(ancestorCopy.ID)
							relationshipCalculatorDialog.Hide()
						})
					resultContainer.Add(ancestorBtn)
				}
			}
		}

		resultContainer.Refresh()
	})
	
	// Now set up person2SearchEntry.OnSubmitted after calculateBtn is defined
	person2SearchEntry.OnSubmitted = func(query string) {
		if query != "" && len(person2Select.Options) > 0 {
			// If only one match, it's already auto-selected, so calculate
			if len(person2Select.Options) == 1 && person1ID != 0 && person2ID != 0 {
				calculateBtn.OnTapped()
			} else if len(person2Select.Options) > 1 {
				// Multiple matches - user should click dropdown to select
				person2Label.SetText(fmt.Sprintf("Person 2: Found %d matches - click dropdown to select", len(person2Select.Options)))
			}
		}
	}

	content.Add(calculateBtn)
	content.Add(widget.NewSeparator())
	content.Add(resultContainer)

	scroll := container.NewVScroll(content)
	scroll.SetMinSize(fyne.NewSize(600, 500))

	relationshipCalculatorDialog = fyne.CurrentApp().NewWindow("Relationship Calculator")
	relationshipCalculatorDialog.SetContent(scroll)
	relationshipCalculatorDialog.Resize(fyne.NewSize(700, 650))
	relationshipCalculatorDialog.SetOnClosed(func() {
		relationshipCalculatorDialog = nil
	})
	relationshipCalculatorDialog.Show()
}

// findSimilarNames finds people with similar names (possible duplicates)
func findSimilarNames(s *store.Store, person *store.Person, excludeID int64) []store.Person {
	allPeople, err := s.GetPeople()
	if err != nil {
		return nil
	}
	
	similar := []store.Person{}
	
	for _, p := range allPeople {
		if p.ID == excludeID {
			continue // Skip the person we're comparing against
		}
		
		// Check for exact surname match with same or similar given name
		if strings.EqualFold(p.Surname, person.Surname) {
			// Same surname - check given name similarity
			if strings.EqualFold(p.GivenName, person.GivenName) {
				// Exact name match - definitely a duplicate candidate
				similar = append(similar, p)
			} else if strings.Contains(strings.ToLower(p.GivenName), strings.ToLower(person.GivenName)) ||
				      strings.Contains(strings.ToLower(person.GivenName), strings.ToLower(p.GivenName)) {
				// Partial given name match (e.g., "Daniel Lourens" vs "Daniel")
				similar = append(similar, p)
			}
		}
	}
	
	// Sort by birth date (helpful to distinguish duplicates)
	sort.Slice(similar, func(i, j int) bool {
		return similar[i].BirthDate < similar[j].BirthDate
	})
	
	// Limit to 5 suggestions to avoid overwhelming
	if len(similar) > 5 {
		similar = similar[:5]
	}
	
	return similar
}

// findCommonAncestors finds all ancestors shared by two people
func findCommonAncestors(s *store.Store, person1ID, person2ID int64) []store.Person {
	// Get all ancestors of person 1
	ancestors1 := getAllAncestors(s, person1ID)
	ancestors1IDs := make(map[int64]bool)
	for _, a := range ancestors1 {
		ancestors1IDs[a.ID] = true
	}

	// Get all ancestors of person 2
	ancestors2 := getAllAncestors(s, person2ID)

	// Find common ones
	common := []store.Person{}
	seenIDs := make(map[int64]bool)
	for _, a := range ancestors2 {
		if ancestors1IDs[a.ID] && !seenIDs[a.ID] {
			common = append(common, a)
			seenIDs[a.ID] = true
		}
	}

	// Sort by birth date (oldest first)
	sort.Slice(common, func(i, j int) bool {
		return common[i].BirthDate < common[j].BirthDate
	})

	return common
}

// getAllAncestors recursively gets all ancestors of a person
func getAllAncestors(s *store.Store, personID int64) []store.Person {
	ancestors := []store.Person{}
	visited := make(map[int64]bool)

	var collectAncestorsRecursive func(int64)
	collectAncestorsRecursive = func(id int64) {
		if visited[id] {
			return
		}
		visited[id] = true

		parents, err := s.GetRelatedPeople(id, "parent")
		if err != nil {
			return
		}

		for _, parent := range parents {
			ancestors = append(ancestors, parent)
			collectAncestorsRecursive(parent.ID)
		}
	}

	collectAncestorsRecursive(personID)
	return ancestors
}

// getLineOfDescent creates a text representation of the line from common ancestor to both people
func getLineOfDescent(s *store.Store, person1ID, person2ID int64, commonAncestors []store.Person) string {
	if len(commonAncestors) == 0 {
		return ""
	}

	// Use the most recent common ancestor (last in the sorted list)
	mrca := commonAncestors[len(commonAncestors)-1]

	p1, _ := s.GetPersonByID(person1ID)
	p2, _ := s.GetPersonByID(person2ID)

	// Get path from MRCA to person1
	path1 := getPathFromAncestor(s, mrca.ID, person1ID)
	// Get path from MRCA to person2
	path2 := getPathFromAncestor(s, mrca.ID, person2ID)

	if len(path1) == 0 || len(path2) == 0 {
		return ""
	}

	result := fmt.Sprintf("Most Recent Common Ancestor: %s %s\n\n", mrca.GivenName, mrca.Surname)
	result += fmt.Sprintf("Path to %s %s:\n", p1.GivenName, p1.Surname)
	result += "  " + strings.Join(path1, " → ") + "\n\n"
	result += fmt.Sprintf("Path to %s %s:\n", p2.GivenName, p2.Surname)
	result += "  " + strings.Join(path2, " → ")

	return result
}

// getPathFromAncestor finds the path from an ancestor to a descendant
func getPathFromAncestor(s *store.Store, ancestorID, descendantID int64) []string {
	if ancestorID == descendantID {
		ancestor, _ := s.GetPersonByID(ancestorID)
		return []string{fmt.Sprintf("%s %s", ancestor.GivenName, ancestor.Surname)}
	}

	// BFS to find path
	type pathNode struct {
		personID int64
		path     []string
	}

	queue := []pathNode{{ancestorID, nil}}
	visited := make(map[int64]bool)

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		if visited[current.personID] {
			continue
		}
		visited[current.personID] = true

		person, err := s.GetPersonByID(current.personID)
		if err != nil {
			continue
		}

		currentPath := append(current.path, fmt.Sprintf("%s %s", person.GivenName, person.Surname))

		if current.personID == descendantID {
			return currentPath
		}

		// Get children
		children, err := s.GetRelatedPeople(current.personID, "child")
		if err != nil {
			continue
		}

		for _, child := range children {
			if !visited[child.ID] {
				queue = append(queue, pathNode{child.ID, currentPath})
			}
		}
	}

	return nil
}
