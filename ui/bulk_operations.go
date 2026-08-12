package ui

import (
	"fmt"
	"genealogy/store"
	"sort"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
)

// Bulk Privacy Settings
var bulkPrivacyDialog fyne.Window

func showBulkPrivacyDialog(w fyne.Window, s *store.Store, refreshFunc func()) {
	// Check if already open
	if bulkPrivacyDialog != nil {
		bulkPrivacyDialog.RequestFocus()
		bulkPrivacyDialog.Show()
		return
	}

	people, err := s.GetPeople()
	if err != nil {
		dialog.ShowError(fmt.Errorf("Failed to load people: %w", err), w)
		return
	}

	// State variables
	filterText := ""
	sortAlphabetically := true
	showLivingOnly := false
	showDeceasedOnly := false

	// Track selections
	selections := make(map[int64]bool)
	checkboxes := make(map[int64]*widget.Check)

	// Build content container and scroll
	var content *fyne.Container
	var scroll *container.Scroll
	var rebuildContent func()

	rebuildContent = func() {
		// Filter candidates
		var candidates []store.Person
		currentYear := time.Now().Year()

		for _, p := range people {
			// Apply living/deceased filter
			if showLivingOnly && !p.IsLiving {
				continue
			}
			if showDeceasedOnly && (p.IsLiving || p.DeathDate == "") {
				continue
			}

			// Apply name filter
			personName := strings.ToLower(formatPersonName(p))
			if filterText == "" || strings.Contains(personName, strings.ToLower(filterText)) {
				candidates = append(candidates, p)
			}
		}

		// Sort candidates
		if sortAlphabetically {
			sort.Slice(candidates, func(i, j int) bool {
				nameI := formatPersonName(candidates[i])
				nameJ := formatPersonName(candidates[j])
				return strings.ToLower(nameI) < strings.ToLower(nameJ)
			})
		} else {
			sort.Slice(candidates, func(i, j int) bool {
				return candidates[i].BirthDate < candidates[j].BirthDate
			})
		}

		// Clear and rebuild content
		content.Objects = nil
		checkboxes = make(map[int64]*widget.Check)

		// Title
		title := widget.NewLabelWithStyle("Bulk Privacy Settings",
			fyne.TextAlignCenter, fyne.TextStyle{Bold: true})
		content.Add(title)
		content.Add(widget.NewSeparator())

		if len(candidates) == 0 {
			content.Add(widget.NewLabel(fmt.Sprintf("No matches for current filters")))
		} else {
			// Summary
			summary := widget.NewLabel(fmt.Sprintf("Showing %d of %d people:",
				len(candidates), len(people)))
			content.Add(summary)
			content.Add(widget.NewSeparator())

			// Action buttons
			selectAllBtn := widget.NewButton("Select All Visible", func() {
				for _, cand := range candidates {
					if check, ok := checkboxes[cand.ID]; ok {
						check.SetChecked(true)
						selections[cand.ID] = true
					}
				}
			})

			deselectAllBtn := widget.NewButton("Deselect All", func() {
				for id, check := range checkboxes {
					check.SetChecked(false)
					selections[id] = false
				}
			})

			// Multiple action buttons
			markLivingBtn := widget.NewButton("✓ Mark as Living", func() {
				applyBulkPrivacyAction(s, selections, "mark_living", bulkPrivacyDialog, refreshFunc)
			})

			markDeceasedBtn := widget.NewButton("† Mark as Deceased", func() {
				applyBulkPrivacyAction(s, selections, "mark_deceased", bulkPrivacyDialog, refreshFunc)
			})

			clearContactBtn := widget.NewButton("🔒 Clear Contact Info", func() {
				applyBulkPrivacyAction(s, selections, "clear_contact", bulkPrivacyDialog, refreshFunc)
			})

			clearDeathDateBtn := widget.NewButton("Clear Death Dates", func() {
				applyBulkPrivacyAction(s, selections, "clear_death", bulkPrivacyDialog, refreshFunc)
			})

			selectionBox := container.NewHBox(selectAllBtn, deselectAllBtn)
			content.Add(selectionBox)

			actionBox1 := container.NewHBox(markLivingBtn, markDeceasedBtn)
			actionBox2 := container.NewHBox(clearContactBtn, clearDeathDateBtn)
			content.Add(actionBox1)
			content.Add(actionBox2)
			content.Add(widget.NewSeparator())

			// Add each candidate
			for _, cand := range candidates {
				candCopy := cand // Capture for closure

				// Restore previous selection state
				previouslySelected := selections[cand.ID]

				check := widget.NewCheck("", func(checked bool) {
					selections[candCopy.ID] = checked
				})
				check.SetChecked(previouslySelected)
				checkboxes[cand.ID] = check

				// Build info string
				infoStr := formatPersonName(cand)
				if cand.BirthDate != "" {
					infoStr += fmt.Sprintf(" (born %s", cand.BirthDate)
					if cand.BirthDate != "" {
						birthYear := parseBirthYear(cand.BirthDate)
						if birthYear > 0 {
							age := currentYear - birthYear
							infoStr += fmt.Sprintf(", age ~%d", age)
						}
					}
					infoStr += ")"
				}

				if cand.IsLiving {
					infoStr += " [LIVING]"
				} else if cand.DeathDate != "" {
					infoStr += fmt.Sprintf(" [Deceased %s]", cand.DeathDate)
				}

				// Show if has contact info
				hasContact := cand.Address != "" || cand.City != "" || cand.Phone != "" || cand.Email != ""
				if hasContact {
					infoStr += " 📧"
				}

				nameLabel := widget.NewLabel(infoStr)
				personBox := container.NewHBox(check, nameLabel)
				content.Add(personBox)
			}
		}

		content.Refresh()
		if scroll != nil {
			scroll.Refresh()
		}
	}

	// Initialize content
	content = container.NewVBox()

	// Filter input
	filterEntry := widget.NewEntry()
	filterEntry.SetPlaceHolder("Filter by name...")
	filterEntry.OnChanged = func(text string) {
		filterText = text
		rebuildContent()
	}

	// Filter checkboxes (declare first to avoid forward reference issues)
	var showLivingCheck, showDeceasedCheck *widget.Check

	showLivingCheck = widget.NewCheck("Living people only", func(checked bool) {
		showLivingOnly = checked
		if checked {
			showDeceasedOnly = false
			showDeceasedCheck.SetChecked(false)
		}
		rebuildContent()
	})

	showDeceasedCheck = widget.NewCheck("Deceased people only", func(checked bool) {
		showDeceasedOnly = checked
		if checked {
			showLivingOnly = false
			showLivingCheck.SetChecked(false)
		}
		rebuildContent()
	})

	// Sort options
	sortRadio := widget.NewRadioGroup([]string{"Sort Alphabetically (A-Z)", "Sort by Birth Date"}, func(value string) {
		sortAlphabetically = (value == "Sort Alphabetically (A-Z)")
		rebuildContent()
	})
	sortRadio.SetSelected("Sort Alphabetically (A-Z)")

	// Initial build
	rebuildContent()

	scroll = container.NewVScroll(content)
	scroll.SetMinSize(fyne.NewSize(700, 400))

	// Main layout with controls at top
	mainContent := container.NewBorder(
		container.NewVBox(
			filterEntry,
			container.NewHBox(showLivingCheck, showDeceasedCheck),
			sortRadio,
			widget.NewSeparator(),
		),
		nil, nil, nil,
		scroll,
	)

	bulkPrivacyDialog = fyne.CurrentApp().NewWindow("Bulk Privacy Settings")
	bulkPrivacyDialog.SetContent(mainContent)
	bulkPrivacyDialog.Resize(fyne.NewSize(900, 700))
	bulkPrivacyDialog.SetOnClosed(func() {
		bulkPrivacyDialog = nil
	})
	bulkPrivacyDialog.Show()
}

func applyBulkPrivacyAction(s *store.Store, selections map[int64]bool, action string, parentWindow fyne.Window, refreshFunc func()) {
	selectedCount := 0
	var selectedIDs []int64
	for id, selected := range selections {
		if selected {
			selectedCount++
			selectedIDs = append(selectedIDs, id)
		}
	}

	if selectedCount == 0 {
		dialog.ShowInformation("No Selection", "Please select at least one person.", parentWindow)
		return
	}

	var actionDesc string
	switch action {
	case "mark_living":
		actionDesc = fmt.Sprintf("mark %d people as LIVING", selectedCount)
	case "mark_deceased":
		actionDesc = fmt.Sprintf("mark %d people as DECEASED (remove living flag)", selectedCount)
	case "clear_contact":
		actionDesc = fmt.Sprintf("clear contact information for %d people", selectedCount)
	case "clear_death":
		actionDesc = fmt.Sprintf("clear death dates for %d people", selectedCount)
	default:
		return
	}

	dialog.ShowConfirm("Confirm Bulk Action",
		fmt.Sprintf("Are you sure you want to %s?", actionDesc),
		func(confirmed bool) {
			if !confirmed {
				return
			}

			updatedCount := 0
			var errors []string

			for _, id := range selectedIDs {
				person, err := s.GetPersonByID(id)
				if err != nil {
					errors = append(errors, fmt.Sprintf("Failed to get person ID %d: %v", id, err))
					continue
				}

				switch action {
				case "mark_living":
					person.IsLiving = true
					person.DeathDate = "" // Clear death date when marking as living
				case "mark_deceased":
					person.IsLiving = false
				case "clear_contact":
					person.Address = ""
					person.City = ""
					person.State = ""
					person.PostalCode = ""
					person.Country = ""
					person.Phone = ""
					person.Email = ""
				case "clear_death":
					person.DeathDate = ""
					person.DeathPlace = ""
				}

				if err := s.UpdatePerson(person); err == nil {
					updatedCount++
				} else {
					errors = append(errors, fmt.Sprintf("%s: %v", formatPersonName(*person), err))
				}
			}

			if len(errors) > 0 {
				maxErrors := 5
				if len(errors) < maxErrors {
					maxErrors = len(errors)
				}
				errorMsg := fmt.Sprintf("Updated %d people.\n\nErrors occurred:\n%s",
					updatedCount, strings.Join(errors[:maxErrors], "\n"))
				if len(errors) > 5 {
					errorMsg += fmt.Sprintf("\n... and %d more errors", len(errors)-5)
				}
				dialog.ShowError(fmt.Errorf("%s", errorMsg), parentWindow)
			} else {
				dialog.ShowInformation("Success",
					fmt.Sprintf("Successfully updated %d people.", updatedCount),
					parentWindow)
			}

			// Refresh and close
			refreshFunc()
			parentWindow.Close()
		}, parentWindow)
}
