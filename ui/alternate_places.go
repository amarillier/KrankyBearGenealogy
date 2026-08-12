package ui

import (
	"fmt"
	"strconv"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"

	"genealogy/store"
)

var alternatePlacesWindow fyne.Window

// showAlternatePlacesManager shows a dialog to manage historical place name mappings
func showAlternatePlacesManager(w fyne.Window, s *store.Store) {
	// If window already exists and is visible, just bring it to front
	if alternatePlacesWindow != nil && alternatePlacesWindow.Content().Visible() {
		alternatePlacesWindow.Show()
		alternatePlacesWindow.RequestFocus()
		return
	}

	// Create a new window for alternate places
	altPlacesWindow := fyne.CurrentApp().NewWindow("Historical Place Names")
	altPlacesWindow.Resize(fyne.NewSize(800, 600))
	alternatePlacesWindow = altPlacesWindow
	
	// Register for window lifecycle management
	RegisterSecondaryWindow(altPlacesWindow)

	// Load existing alternate places
	alternatePlaces, err := s.GetAllAlternatePlaces()
	if err != nil {
		dialog.ShowError(err, altPlacesWindow)
		return
	}

	// Create list of alternate places
	var list *fyne.Container
	var refreshList func()

	refreshList = func() {
		alternatePlaces, err = s.GetAllAlternatePlaces()
		if err != nil {
			dialog.ShowError(err, altPlacesWindow)
			return
		}

		list.Objects = nil
		if len(alternatePlaces) == 0 {
			emptyLabel := widget.NewLabel("No historical place name mappings recorded")
			emptyLabel.Wrapping = fyne.TextWrapWord
			list.Add(emptyLabel)
		} else {
			for _, alt := range alternatePlaces {
				altCopy := alt // Capture for closure
				placeCard := makeAlternatePlaceCard(altCopy, s, refreshList, altPlacesWindow)
				list.Add(placeCard)
			}
		}
		list.Refresh()
	}

	list = container.NewVBox()
	refreshList()

	scroll := container.NewVScroll(list)
	scroll.SetMinSize(fyne.NewSize(750, 400))

	addBtn := widget.NewButton("+ Add Historical Place Name", func() {
		showAddAlternatePlaceDialog(altPlacesWindow, s, refreshList)
	})

	// Instructions
	instructions := widget.NewLabel(
		"Map historical place names to their modern equivalents.\n" +
		"Examples: \"Salisbury, Rhodesia\" → \"Harare, Zimbabwe\"\n" +
		"          \"Leningrad, USSR\" → \"St. Petersburg, Russia\"\n\n" +
		"• Used for geocoding fallback when historical name not found\n" +
		"• Displayed as context: \"Salisbury, Rhodesia (now Harare, Zimbabwe)\"")
	instructions.Wrapping = fyne.TextWrapWord

	// Layout
	content := container.NewBorder(
		container.NewVBox(instructions, widget.NewSeparator(), addBtn, widget.NewSeparator()),
		nil, nil, nil,
		scroll,
	)

	altPlacesWindow.SetContent(content)
	
	// Hide window instead of closing it, so we can reuse it
	altPlacesWindow.SetCloseIntercept(func() {
		altPlacesWindow.Hide()
	})
	
	altPlacesWindow.Show()
	altPlacesWindow.RequestFocus() // Bring window to front
}

// makeAlternatePlaceCard creates a card for displaying an alternate place mapping
func makeAlternatePlaceCard(alt store.AlternatePlace, s *store.Store, onUpdate func(), w fyne.Window) *fyne.Container {
	// Format display text
	displayText := fmt.Sprintf("%s → %s", alt.HistoricalName, alt.CurrentName)
	if alt.YearChanged > 0 {
		displayText += fmt.Sprintf(" (changed %d)", alt.YearChanged)
	}

	nameLabel := widget.NewLabel(displayText)
	nameLabel.TextStyle.Bold = true
	nameLabel.Wrapping = fyne.TextWrapWord

	// Type and notes
	detailsText := ""
	if alt.Notes != "" {
		detailsText = "Notes: " + alt.Notes
	}

	var cardObjects []fyne.CanvasObject
	cardObjects = append(cardObjects, nameLabel)

	if detailsText != "" {
		detailLabel := widget.NewLabel(detailsText)
		detailLabel.Wrapping = fyne.TextWrapWord
		cardObjects = append(cardObjects, detailLabel)
	}

	// Edit button
	editBtn := widget.NewButton("Edit", func() {
		showEditAlternatePlaceDialog(w, s, &alt, onUpdate)
	})

	// Delete button
	deleteBtn := widget.NewButton("Delete", func() {
		dialog.ShowConfirm("Delete Historical Place Name",
			fmt.Sprintf("Delete mapping '%s → %s'?", alt.HistoricalName, alt.CurrentName),
			func(confirmed bool) {
				if confirmed {
					if err := s.DeleteAlternatePlace(alt.ID); err != nil {
						dialog.ShowError(err, w)
					} else {
						onUpdate()
					}
				}
			}, w)
	})

	buttons := container.NewHBox(editBtn, deleteBtn)
	cardObjects = append(cardObjects, buttons)

	card := container.NewVBox(cardObjects...)

	// Add separator
	return container.NewVBox(card, widget.NewSeparator())
}

// showAddAlternatePlaceDialog shows a dialog to add a new historical place name mapping
func showAddAlternatePlaceDialog(parentWindow fyne.Window, s *store.Store, onSave func()) {
	// Create form fields
	historicalNameEntry := widget.NewEntry()
	historicalNameEntry.SetPlaceHolder("Historical name (e.g., Salisbury, Rhodesia)")

	currentNameEntry := widget.NewEntry()
	currentNameEntry.SetPlaceHolder("Current name (e.g., Harare, Zimbabwe)")

	yearEntry := widget.NewEntry()
	yearEntry.SetPlaceHolder("Year changed (optional, e.g., 1982)")

	notesEntry := widget.NewMultiLineEntry()
	notesEntry.SetPlaceHolder("Optional notes (e.g., 'Capital city renamed after independence')")
	notesEntry.SetMinRowsVisible(2)

	validationLabel := widget.NewLabel("")
	validationLabel.Hide()

	// Form items
	form := container.NewVBox(
		widget.NewLabel("Historical Place Name:"), historicalNameEntry,
		widget.NewLabel("Current Place Name:"), currentNameEntry,
		widget.NewLabel("Year Changed:"), yearEntry,
		widget.NewLabel("Notes (optional):"), notesEntry,
		validationLabel,
	)

	formScroll := container.NewVScroll(form)
	formScroll.SetMinSize(fyne.NewSize(500, 350))

	// Create dialog
	var d dialog.Dialog
	d = dialog.NewCustomConfirm("Add Historical Place Name", "Save", "Cancel", formScroll, func(save bool) {
		if !save {
			return
		}

		// Validate
		historicalName := strings.TrimSpace(historicalNameEntry.Text)
		currentName := strings.TrimSpace(currentNameEntry.Text)

		if historicalName == "" || currentName == "" {
			validationLabel.SetText("❌ Both historical and current place names are required")
			validationLabel.Show()
			return
		}

		// Parse year (optional)
		var yearChanged int
		if yearEntry.Text != "" {
			var err error
			yearChanged, err = strconv.Atoi(strings.TrimSpace(yearEntry.Text))
			if err != nil || yearChanged < 0 || yearChanged > 9999 {
				validationLabel.SetText("❌ Invalid year (must be a number between 0 and 9999)")
				validationLabel.Show()
				return
			}
		}

		// Create alternate place
		alt := &store.AlternatePlace{
			HistoricalName: historicalName,
			CurrentName:    currentName,
			YearChanged:    yearChanged,
			Notes:          strings.TrimSpace(notesEntry.Text),
		}

		if err := s.CreateAlternatePlace(alt); err != nil {
			dialog.ShowError(err, parentWindow)
		} else {
			d.Hide()
			if onSave != nil {
				onSave()
			}
		}
	}, parentWindow)

	d.Resize(fyne.NewSize(550, 450))
	d.Show()
}

// showEditAlternatePlaceDialog shows a dialog to edit an existing historical place name mapping
func showEditAlternatePlaceDialog(parentWindow fyne.Window, s *store.Store, alt *store.AlternatePlace, onSave func()) {
	// Create form fields with existing values
	historicalNameEntry := widget.NewEntry()
	historicalNameEntry.SetText(alt.HistoricalName)
	historicalNameEntry.SetPlaceHolder("Historical name (e.g., Salisbury, Rhodesia)")

	currentNameEntry := widget.NewEntry()
	currentNameEntry.SetText(alt.CurrentName)
	currentNameEntry.SetPlaceHolder("Current name (e.g., Harare, Zimbabwe)")

	yearEntry := widget.NewEntry()
	if alt.YearChanged > 0 {
		yearEntry.SetText(strconv.Itoa(alt.YearChanged))
	}
	yearEntry.SetPlaceHolder("Year changed (optional, e.g., 1982)")

	notesEntry := widget.NewMultiLineEntry()
	notesEntry.SetText(alt.Notes)
	notesEntry.SetPlaceHolder("Optional notes")
	notesEntry.SetMinRowsVisible(2)

	validationLabel := widget.NewLabel("")
	validationLabel.Hide()

	// Form items
	form := container.NewVBox(
		widget.NewLabel("Historical Place Name:"), historicalNameEntry,
		widget.NewLabel("Current Place Name:"), currentNameEntry,
		widget.NewLabel("Year Changed:"), yearEntry,
		widget.NewLabel("Notes (optional):"), notesEntry,
		validationLabel,
	)

	formScroll := container.NewVScroll(form)
	formScroll.SetMinSize(fyne.NewSize(500, 350))

	// Create dialog
	var d dialog.Dialog
	d = dialog.NewCustomConfirm("Edit Historical Place Name", "Save", "Cancel", formScroll, func(save bool) {
		if !save {
			return
		}

		// Validate
		historicalName := strings.TrimSpace(historicalNameEntry.Text)
		currentName := strings.TrimSpace(currentNameEntry.Text)

		if historicalName == "" || currentName == "" {
			validationLabel.SetText("❌ Both historical and current place names are required")
			validationLabel.Show()
			return
		}

		// Parse year (optional)
		var yearChanged int
		if yearEntry.Text != "" {
			var err error
			yearChanged, err = strconv.Atoi(strings.TrimSpace(yearEntry.Text))
			if err != nil || yearChanged < 0 || yearChanged > 9999 {
				validationLabel.SetText("❌ Invalid year (must be a number between 0 and 9999)")
				validationLabel.Show()
				return
			}
		}

		// Update alternate place
		alt.HistoricalName = historicalName
		alt.CurrentName = currentName
		alt.YearChanged = yearChanged
		alt.Notes = strings.TrimSpace(notesEntry.Text)

		if err := s.UpdateAlternatePlace(alt); err != nil {
			dialog.ShowError(err, parentWindow)
		} else {
			d.Hide()
			if onSave != nil {
				onSave()
			}
		}
	}, parentWindow)

	d.Resize(fyne.NewSize(550, 450))
	d.Show()
}
