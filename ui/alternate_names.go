package ui

import (
	"fmt"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
	"genealogy/store"
)

// showAlternateNamesManager shows a dialog to manage alternate names for a person
func showAlternateNamesManager(w fyne.Window, s *store.Store, personID int64, personName string) {
	// Create a new window for alternate names
	altNamesWindow := fyne.CurrentApp().NewWindow("Alternate Names - " + personName)
	altNamesWindow.Resize(fyne.NewSize(600, 500))

	// Load existing alternate names
	alternateNames, err := s.GetAlternateNames(personID)
	if err != nil {
		dialog.ShowError(err, altNamesWindow)
		return
	}

	// Create list of alternate names
	namesList := container.NewVBox()
	
	var refreshList func()
	refreshList = func() {
		alternateNames, err = s.GetAlternateNames(personID)
		if err != nil {
			dialog.ShowError(err, altNamesWindow)
			return
		}

		namesList.Objects = nil

		if len(alternateNames) == 0 {
			emptyLabel := widget.NewLabel("No alternate names recorded")
			emptyLabel.TextStyle = fyne.TextStyle{Italic: true}
			namesList.Add(emptyLabel)
		} else {
			for _, alt := range alternateNames {
				altCopy := alt // Capture for closure
				nameCard := makeAlternateNameCard(altCopy, s, refreshList, altNamesWindow)
				namesList.Add(nameCard)
			}
		}
		namesList.Refresh()
	}

	refreshList()

	// Scroll container
	scroll := container.NewVScroll(namesList)

	// Add button
	addBtn := widget.NewButton("+ Add Alternate Name", func() {
		showAddAlternateNameDialog(altNamesWindow, s, personID, refreshList)
	})

	// Instructions
	instructions := widget.NewLabel(
		"Alternate names help track name variations, spellings, nicknames, and maiden names. " +
		"These will be included in searches and displayed in person details.")
	instructions.Wrapping = fyne.TextWrapWord

	// Layout
	content := container.NewBorder(
		container.NewVBox(instructions, widget.NewSeparator(), addBtn, widget.NewSeparator()),
		nil, nil, nil,
		scroll,
	)

	altNamesWindow.SetContent(content)
	altNamesWindow.Show()
}

// makeAlternateNameCard creates a card for displaying an alternate name
func makeAlternateNameCard(alt store.AlternateName, s *store.Store, onUpdate func(), w fyne.Window) *fyne.Container {
	// Format the name
	fullName := fmt.Sprintf("%s %s", alt.GivenName, alt.Surname)
	if alt.GivenName == "" {
		fullName = alt.Surname
	} else if alt.Surname == "" {
		fullName = alt.GivenName
	}

	// Name type badge
	typeLabel := widget.NewLabel(fmt.Sprintf("[%s]", alt.NameType))
	typeLabel.TextStyle = fyne.TextStyle{Bold: true}

	nameLabel := widget.NewLabel(fullName)
	nameLabel.TextStyle = fyne.TextStyle{Bold: true}

	headerLine := container.NewHBox(typeLabel, nameLabel)

	// Notes (if any)
	var notesLine *widget.Label
	if alt.Notes != "" {
		notesLine = widget.NewLabel(fmt.Sprintf("Note: %s", alt.Notes))
		notesLine.TextStyle = fyne.TextStyle{Italic: true}
	}

	// Edit button
	editBtn := widget.NewButton("Edit", func() {
		showEditAlternateNameDialog(w, s, &alt, onUpdate)
	})

	// Delete button
	deleteBtn := widget.NewButton("Delete", func() {
		dialog.ShowConfirm("Delete Alternate Name",
			fmt.Sprintf("Delete alternate name '%s'?", fullName),
			func(confirmed bool) {
				if !confirmed {
					return
				}
				if err := s.DeleteAlternateName(alt.ID); err != nil {
					dialog.ShowError(err, w)
					return
				}
				onUpdate()
			}, w)
	})

	buttons := container.NewHBox(editBtn, deleteBtn)

	// Assemble card
	cardContent := container.NewVBox(headerLine)
	if notesLine != nil {
		cardContent.Add(notesLine)
	}
	cardContent.Add(buttons)

	return container.NewVBox(
		cardContent,
		widget.NewSeparator(),
	)
}

// showAddAlternateNameDialog shows a dialog to add a new alternate name
func showAddAlternateNameDialog(parentWindow fyne.Window, s *store.Store, personID int64, onSave func()) {
	// Create form fields
	givenNameEntry := widget.NewEntry()
	givenNameEntry.SetPlaceHolder("Alternate given name(s)")

	surnameEntry := widget.NewEntry()
	surnameEntry.SetPlaceHolder("Alternate surname")

	typeSelect := widget.NewSelect([]string{"nickname", "maiden", "married", "spelling", "other"}, nil)
	typeSelect.SetSelected("spelling") // Default

	notesEntry := widget.NewMultiLineEntry()
	notesEntry.SetPlaceHolder("Optional: Why this name is different (e.g., 'Used in 1900 census')")
	notesEntry.SetMinRowsVisible(2)

	validationLabel := widget.NewLabel("")
	validationLabel.Hide()

	// Form items
	form := container.NewVBox(
		widget.NewLabel("Alternate Given Name:"), givenNameEntry,
		widget.NewLabel("Alternate Surname:"), surnameEntry,
		widget.NewLabel("Name Type:"), typeSelect,
		widget.NewLabel("Notes (optional):"), notesEntry,
		validationLabel,
	)

	formScroll := container.NewVScroll(form)
	formScroll.SetMinSize(fyne.NewSize(400, 300))

	// Create dialog
	d := dialog.NewCustomConfirm("Add Alternate Name", "Save", "Cancel", formScroll, func(save bool) {
		if !save {
			return
		}

		// Validate
		givenName := givenNameEntry.Text
		surname := surnameEntry.Text

		if givenName == "" && surname == "" {
			validationLabel.SetText("❌ Please enter at least a given name or surname")
			validationLabel.Show()
			return
		}

		// Create alternate name
		alt := &store.AlternateName{
			PersonID:  personID,
			NameType:  typeSelect.Selected,
			GivenName: givenName,
			Surname:   surname,
			Notes:     notesEntry.Text,
		}

		if err := s.CreateAlternateName(alt); err != nil {
			dialog.ShowError(err, parentWindow)
			return
		}

		onSave()
	}, parentWindow)

	d.Resize(fyne.NewSize(500, 400))
	d.Show()
}

// showEditAlternateNameDialog shows a dialog to edit an existing alternate name
func showEditAlternateNameDialog(parentWindow fyne.Window, s *store.Store, alt *store.AlternateName, onSave func()) {
	// Create form fields with existing values
	givenNameEntry := widget.NewEntry()
	givenNameEntry.SetText(alt.GivenName)
	givenNameEntry.SetPlaceHolder("Alternate given name(s)")

	surnameEntry := widget.NewEntry()
	surnameEntry.SetText(alt.Surname)
	surnameEntry.SetPlaceHolder("Alternate surname")

	typeSelect := widget.NewSelect([]string{"nickname", "maiden", "married", "spelling", "other"}, nil)
	typeSelect.SetSelected(alt.NameType)

	notesEntry := widget.NewMultiLineEntry()
	notesEntry.SetText(alt.Notes)
	notesEntry.SetPlaceHolder("Optional: Why this name is different")
	notesEntry.SetMinRowsVisible(2)

	validationLabel := widget.NewLabel("")
	validationLabel.Hide()

	// Form items
	form := container.NewVBox(
		widget.NewLabel("Alternate Given Name:"), givenNameEntry,
		widget.NewLabel("Alternate Surname:"), surnameEntry,
		widget.NewLabel("Name Type:"), typeSelect,
		widget.NewLabel("Notes (optional):"), notesEntry,
		validationLabel,
	)

	formScroll := container.NewVScroll(form)
	formScroll.SetMinSize(fyne.NewSize(400, 300))

	// Create dialog
	d := dialog.NewCustomConfirm("Edit Alternate Name", "Save", "Cancel", formScroll, func(save bool) {
		if !save {
			return
		}

		// Validate
		givenName := givenNameEntry.Text
		surname := surnameEntry.Text

		if givenName == "" && surname == "" {
			validationLabel.SetText("❌ Please enter at least a given name or surname")
			validationLabel.Show()
			return
		}

		// Update alternate name
		alt.NameType = typeSelect.Selected
		alt.GivenName = givenName
		alt.Surname = surname
		alt.Notes = notesEntry.Text

		if err := s.UpdateAlternateName(alt); err != nil {
			dialog.ShowError(err, parentWindow)
			return
		}

		onSave()
	}, parentWindow)

	d.Resize(fyne.NewSize(500, 400))
	d.Show()
}
