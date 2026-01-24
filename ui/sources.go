package ui

import (
	"fmt"
	"sort"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"

	"genealogy/store"
)

// showSourcesLibrary shows the sources library window.
var sourcesLibraryWindow fyne.Window

func showSourcesLibrary(parentWindow fyne.Window, s *store.Store) {
	// Check if already open
	if sourcesLibraryWindow != nil {
		sourcesLibraryWindow.RequestFocus()
		sourcesLibraryWindow.Show()
		return
	}

	// Create new window
	sourcesLibraryWindow = fyne.CurrentApp().NewWindow("Sources Library")

	// Function to rebuild content
	var rebuildContent func()
	rebuildContent = func() {
		// Load all sources
		sources, err := s.GetAllSources()
		if err != nil {
			dialog.ShowError(fmt.Errorf("Failed to load sources: %w", err), sourcesLibraryWindow)
			return
		}

		content := container.NewVBox()

		// Header
		header := widget.NewLabelWithStyle(
			fmt.Sprintf("Sources Library (%d sources)", len(sources)),
			fyne.TextAlignCenter, fyne.TextStyle{Bold: true})
		content.Add(header)
		content.Add(widget.NewSeparator())

		// Add New Source button
		addBtn := widget.NewButton("+ Add New Source", func() {
			showAddSourceDialog(sourcesLibraryWindow, s, rebuildContent)
		})
		content.Add(addBtn)
		content.Add(widget.NewSeparator())

		if len(sources) == 0 {
			content.Add(widget.NewLabel("No sources in library."))
			content.Add(widget.NewLabel("Click '+ Add New Source' to add your first source."))
		} else {
			// Display sources
			for _, source := range sources {
				sourceCopy := source // Capture for closure
				sourceCard := makeSourceCard(sourceCopy, s, rebuildContent, sourcesLibraryWindow)
				content.Add(sourceCard)
				content.Add(widget.NewSeparator())
			}
		}

		scroll := container.NewVScroll(content)
		scroll.SetMinSize(fyne.NewSize(800, 500))
		sourcesLibraryWindow.SetContent(scroll)
	}

	rebuildContent()
	sourcesLibraryWindow.Resize(fyne.NewSize(900, 650))
	sourcesLibraryWindow.SetOnClosed(func() {
		sourcesLibraryWindow = nil
	})
	sourcesLibraryWindow.Show()
}

// makeSourceCard creates a card widget for displaying a source.
func makeSourceCard(source store.Source, s *store.Store, refresh func(), w fyne.Window) fyne.CanvasObject {
	// Source type icon
	typeIcon := "📄"
	switch source.SourceType {
	case "vital_record":
		typeIcon = "📜"
	case "census":
		typeIcon = "📊"
	case "church":
		typeIcon = "⛪"
	case "book":
		typeIcon = "📚"
	case "website":
		typeIcon = "🌐"
	}

	// Title
	titleLabel := widget.NewLabelWithStyle(
		fmt.Sprintf("%s %s", typeIcon, source.Title),
		fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	titleLabel.Wrapping = fyne.TextWrapWord

	// Details
	var details []string
	if source.Author != "" {
		details = append(details, fmt.Sprintf("Author: %s", source.Author))
	}
	if source.Publication != "" {
		details = append(details, fmt.Sprintf("Publication: %s", source.Publication))
	}
	if source.Repository != "" {
		details = append(details, fmt.Sprintf("Repository: %s", source.Repository))
	}
	if source.CallNumber != "" {
		details = append(details, fmt.Sprintf("Call/Reference: %s", source.CallNumber))
	}

	// Get citation count
	citations, _ := s.GetCitationsForSource(source.ID)
	citationCount := len(citations)
	details = append(details, fmt.Sprintf("Citations: %d", citationCount))

	detailsText := strings.Join(details, " | ")
	detailsLabel := widget.NewLabel(detailsText)
	detailsLabel.TextStyle.Italic = true
	detailsLabel.Wrapping = fyne.TextWrapWord

	// Notes (if any)
	var notesWidget fyne.CanvasObject
	if source.Notes != "" {
		notesLabel := widget.NewLabel("Notes: " + source.Notes)
		notesLabel.Wrapping = fyne.TextWrapWord
		notesWidget = notesLabel
	}

	// Buttons
	editBtn := widget.NewButton("Edit", func() {
		showEditSourceDialog(w, s, &source, refresh)
	})

	viewCitationsBtn := widget.NewButton(fmt.Sprintf("View Citations (%d)", citationCount), func() {
		showSourceCitationsDialog(w, s, &source)
	})

	deleteBtn := widget.NewButton("Delete", func() {
		if citationCount > 0 {
			dialog.ShowConfirm("Delete Source",
				fmt.Sprintf("This source has %d citations. Deleting the source will also delete all citations.\n\nAre you sure?", citationCount),
				func(confirmed bool) {
					if confirmed {
						if err := s.DeleteSource(source.ID); err != nil {
							dialog.ShowError(err, w)
						} else {
							refresh()
						}
					}
				}, w)
		} else {
			dialog.ShowConfirm("Delete Source",
				"Are you sure you want to delete this source?",
				func(confirmed bool) {
					if confirmed {
						if err := s.DeleteSource(source.ID); err != nil {
							dialog.ShowError(err, w)
						} else {
							refresh()
						}
					}
				}, w)
		}
	})

	buttons := container.NewHBox(editBtn, viewCitationsBtn, deleteBtn)

	// Build card
	card := container.NewVBox(titleLabel, detailsLabel)
	if notesWidget != nil {
		card.Add(notesWidget)
	}
	card.Add(buttons)

	return card
}

// showAddSourceDialog shows a dialog to add a new source.
func showAddSourceDialog(w fyne.Window, s *store.Store, onSave func()) {
	titleEntry := widget.NewEntry()
	titleEntry.SetPlaceHolder("e.g., 1900 US Federal Census")

	authorEntry := widget.NewEntry()
	authorEntry.SetPlaceHolder("Author or creator (optional)")

	publicationEntry := widget.NewEntry()
	publicationEntry.SetPlaceHolder("e.g., Ancestry.com, FindMyPast (optional)")

	repositoryEntry := widget.NewEntry()
	repositoryEntry.SetPlaceHolder("e.g., National Archives, FamilySearch (optional)")

	callNumberEntry := widget.NewEntry()
	callNumberEntry.SetPlaceHolder("Reference number, URL, or call number (optional)")

	sourceTypeSelect := widget.NewSelect([]string{"vital_record", "census", "church", "book", "website", "other"}, func(string) {})
	sourceTypeSelect.SetSelected("other")

	notesEntry := widget.NewMultiLineEntry()
	notesEntry.SetPlaceHolder("Additional notes about this source (optional)")
	notesEntry.SetMinRowsVisible(3)

	// Validation error label
	validationLabel := widget.NewLabel("")
	validationLabel.Importance = widget.DangerImportance
	validationLabel.Wrapping = fyne.TextWrapWord
	validationLabel.Hide()

	form := container.NewVBox(
		validationLabel,
		widget.NewLabel("Title: *"),
		titleEntry,
		widget.NewLabel("Author:"),
		authorEntry,
		widget.NewLabel("Publication:"),
		publicationEntry,
		widget.NewLabel("Repository:"),
		repositoryEntry,
		widget.NewLabel("Call Number/URL:"),
		callNumberEntry,
		widget.NewLabel("Source Type:"),
		sourceTypeSelect,
		widget.NewLabel("Notes:"),
		notesEntry,
	)

	// Create custom window instead of dialog
	sourceWindow := fyne.CurrentApp().NewWindow("Add New Source")
	sourceWindow.Resize(fyne.NewSize(700, 550))

	formScroll := container.NewVScroll(form)

	// Save button handler
	saveBtn := widget.NewButton("Save", func() {
		// Clear any previous validation error
		validationLabel.SetText("")
		validationLabel.Hide()

		title := strings.TrimSpace(titleEntry.Text)
		if title == "" {
			validationLabel.SetText("❌ Title is required")
			validationLabel.Show()
			formScroll.ScrollToTop()
			return
		}

		source := &store.Source{
			Title:       title,
			Author:      strings.TrimSpace(authorEntry.Text),
			Publication: strings.TrimSpace(publicationEntry.Text),
			Repository:  strings.TrimSpace(repositoryEntry.Text),
			CallNumber:  strings.TrimSpace(callNumberEntry.Text),
			SourceType:  sourceTypeSelect.Selected,
			Notes:       strings.TrimSpace(notesEntry.Text),
		}

		if err := s.CreateSource(source); err != nil {
			validationLabel.SetText(fmt.Sprintf("❌ Failed to save: %v", err))
			validationLabel.Show()
			formScroll.ScrollToTop()
			return
		}

		sourceWindow.Close()
		onSave()
	})

	cancelBtn := widget.NewButton("Cancel", func() {
		sourceWindow.Close()
	})

	buttons := container.NewHBox(saveBtn, cancelBtn)
	content := container.NewBorder(nil, buttons, nil, nil, formScroll)
	sourceWindow.SetContent(content)
	sourceWindow.Show()
}

// showEditSourceDialog shows a dialog to edit an existing source.
func showEditSourceDialog(w fyne.Window, s *store.Store, source *store.Source, onSave func()) {
	titleEntry := widget.NewEntry()
	titleEntry.SetText(source.Title)

	authorEntry := widget.NewEntry()
	authorEntry.SetText(source.Author)

	publicationEntry := widget.NewEntry()
	publicationEntry.SetText(source.Publication)

	repositoryEntry := widget.NewEntry()
	repositoryEntry.SetText(source.Repository)

	callNumberEntry := widget.NewEntry()
	callNumberEntry.SetText(source.CallNumber)

	sourceTypeSelect := widget.NewSelect([]string{"vital_record", "census", "church", "book", "website", "other"}, func(string) {})
	sourceTypeSelect.SetSelected(source.SourceType)

	notesEntry := widget.NewMultiLineEntry()
	notesEntry.SetText(source.Notes)
	notesEntry.SetMinRowsVisible(3)

	// Validation error label
	validationLabel := widget.NewLabel("")
	validationLabel.Importance = widget.DangerImportance
	validationLabel.Wrapping = fyne.TextWrapWord
	validationLabel.Hide()

	form := container.NewVBox(
		validationLabel,
		widget.NewLabel("Title: *"),
		titleEntry,
		widget.NewLabel("Author:"),
		authorEntry,
		widget.NewLabel("Publication:"),
		publicationEntry,
		widget.NewLabel("Repository:"),
		repositoryEntry,
		widget.NewLabel("Call Number/URL:"),
		callNumberEntry,
		widget.NewLabel("Source Type:"),
		sourceTypeSelect,
		widget.NewLabel("Notes:"),
		notesEntry,
	)

	// Create custom window instead of dialog
	editWindow := fyne.CurrentApp().NewWindow("Edit Source")
	editWindow.Resize(fyne.NewSize(700, 550))

	formScroll := container.NewVScroll(form)

	// Save button handler
	saveBtn := widget.NewButton("Save", func() {
		// Clear any previous validation error
		validationLabel.SetText("")
		validationLabel.Hide()

		title := strings.TrimSpace(titleEntry.Text)
		if title == "" {
			validationLabel.SetText("❌ Title is required")
			validationLabel.Show()
			formScroll.ScrollToTop()
			return
		}

		source.Title = title
		source.Author = strings.TrimSpace(authorEntry.Text)
		source.Publication = strings.TrimSpace(publicationEntry.Text)
		source.Repository = strings.TrimSpace(repositoryEntry.Text)
		source.CallNumber = strings.TrimSpace(callNumberEntry.Text)
		source.SourceType = sourceTypeSelect.Selected
		source.Notes = strings.TrimSpace(notesEntry.Text)

		if err := s.UpdateSource(source); err != nil {
			validationLabel.SetText(fmt.Sprintf("❌ Failed to update: %v", err))
			validationLabel.Show()
			formScroll.ScrollToTop()
			return
		}

		editWindow.Close()
		onSave()
	})

	cancelBtn := widget.NewButton("Cancel", func() {
		editWindow.Close()
	})

	buttons := container.NewHBox(saveBtn, cancelBtn)
	content := container.NewBorder(nil, buttons, nil, nil, formScroll)
	editWindow.SetContent(content)
	editWindow.Show()
}

// showSourceCitationsDialog shows all citations for a source.
func showSourceCitationsDialog(parentWindow fyne.Window, s *store.Store, source *store.Source) {
	citations, err := s.GetCitationsForSource(source.ID)
	if err != nil {
		dialog.ShowError(err, parentWindow)
		return
	}

	content := container.NewVBox()

	title := widget.NewLabelWithStyle(
		fmt.Sprintf("Citations for: %s", source.Title),
		fyne.TextAlignCenter, fyne.TextStyle{Bold: true})
	content.Add(title)
	content.Add(widget.NewSeparator())

	if len(citations) == 0 {
		content.Add(widget.NewLabel("No citations for this source yet."))
	} else {
		content.Add(widget.NewLabel(fmt.Sprintf("%d citations:", len(citations))))
		content.Add(widget.NewSeparator())

		for _, citation := range citations {
			person, _ := s.GetPersonByID(citation.PersonID)
			personName := "Unknown"
			if person != nil {
				personName = formatPersonName(*person)
			}

			citationText := fmt.Sprintf("Person: %s", personName)
			if citation.CitationDetail != "" {
				citationText += fmt.Sprintf("\nDetail: %s", citation.CitationDetail)
			}
			if citation.Confidence != "" {
				citationText += fmt.Sprintf("\nConfidence: %s", citation.Confidence)
			}

			label := widget.NewLabel(citationText)
			label.Wrapping = fyne.TextWrapWord
			content.Add(label)
			content.Add(widget.NewSeparator())
		}
	}

	scroll := container.NewVScroll(content)
	scroll.SetMinSize(fyne.NewSize(600, 400))

	dlg := dialog.NewCustom("Source Citations", "Close", scroll, parentWindow)
	dlg.Resize(fyne.NewSize(700, 500))
	dlg.Show()
}

// showCitationManager shows the citation manager for a specific person.
var citationManagerWindows = make(map[int64]fyne.Window) // Track open windows by person ID

func showCitationManager(parentWindow fyne.Window, s *store.Store, personID int64, personName string) {
	// Check if window already open for this person
	if existingWindow, exists := citationManagerWindows[personID]; exists {
		existingWindow.RequestFocus()
		existingWindow.Show()
		return
	}

	// Create new window
	citationWindow := fyne.CurrentApp().NewWindow(fmt.Sprintf("Sources for: %s", personName))

	// Function to rebuild content
	var rebuildContent func()
	rebuildContent = func() {
		// Reload citations
		citations, err := s.GetCitationsForPerson(personID)
		if err != nil {
			dialog.ShowError(fmt.Errorf("Failed to load citations: %w", err), parentWindow)
			return
		}

		content := container.NewVBox()

		// Header
		header := widget.NewLabelWithStyle(
			fmt.Sprintf("Sources for %s (%d citations)", personName, len(citations)),
			fyne.TextAlignCenter, fyne.TextStyle{Bold: true})
		content.Add(header)
		content.Add(widget.NewSeparator())

		// Add Citation button
		addBtn := widget.NewButton("+ Add Source Citation", func() {
			showAddCitationDialog(citationWindow, s, personID, rebuildContent)
		})
		content.Add(addBtn)
		content.Add(widget.NewSeparator())

		if len(citations) == 0 {
			content.Add(widget.NewLabel("No source citations yet."))
			content.Add(widget.NewLabel("Click '+ Add Source Citation' to link a source to this person."))
		} else {
			// Display citations
			for _, citation := range citations {
				citationCopy := citation // Capture for closure
				citationCard := makeCitationCard(citationCopy, s, rebuildContent, citationWindow)
				content.Add(citationCard)
				content.Add(widget.NewSeparator())
			}
		}

		scroll := container.NewVScroll(content)
		scroll.SetMinSize(fyne.NewSize(700, 500))
		citationWindow.SetContent(scroll)
	}

	rebuildContent()
	citationWindow.Resize(fyne.NewSize(800, 900))
	citationWindow.SetOnClosed(func() {
		delete(citationManagerWindows, personID)
	})

	citationManagerWindows[personID] = citationWindow
	citationWindow.Show()
}

// makeCitationCard creates a card widget for displaying a citation.
func makeCitationCard(citation store.Citation, s *store.Store, refresh func(), w fyne.Window) fyne.CanvasObject {
	// Get source info
	source, err := s.GetSourceByID(citation.SourceID)
	if err != nil {
		return widget.NewLabel(fmt.Sprintf("Error loading source: %v", err))
	}

	// Source type icon
	typeIcon := "📄"
	switch source.SourceType {
	case "vital_record":
		typeIcon = "📜"
	case "census":
		typeIcon = "📊"
	case "church":
		typeIcon = "⛪"
	case "book":
		typeIcon = "📚"
	case "website":
		typeIcon = "🌐"
	}

	// Source title
	titleLabel := widget.NewLabelWithStyle(
		fmt.Sprintf("%s %s", typeIcon, source.Title),
		fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	titleLabel.Wrapping = fyne.TextWrapWord

	// Citation details
	var details []string
	if citation.CitationDetail != "" {
		details = append(details, fmt.Sprintf("Detail: %s", citation.CitationDetail))
	}
	if citation.Confidence != "" {
		details = append(details, fmt.Sprintf("Confidence: %s", citation.Confidence))
	}

	var detailsWidget fyne.CanvasObject
	if len(details) > 0 {
		detailsText := strings.Join(details, " | ")
		detailsLabel := widget.NewLabel(detailsText)
		detailsLabel.TextStyle.Italic = true
		detailsLabel.Wrapping = fyne.TextWrapWord
		detailsWidget = detailsLabel
	}

	// Transcription (if any)
	var transcriptionWidget fyne.CanvasObject
	if citation.Transcription != "" {
		transcriptionLabel := widget.NewLabel("Transcription: " + citation.Transcription)
		transcriptionLabel.Wrapping = fyne.TextWrapWord
		transcriptionWidget = transcriptionLabel
	}

	// Notes (if any)
	var notesWidget fyne.CanvasObject
	if citation.Notes != "" {
		notesLabel := widget.NewLabel("Notes: " + citation.Notes)
		notesLabel.Wrapping = fyne.TextWrapWord
		notesWidget = notesLabel
	}

	// Buttons
	editBtn := widget.NewButton("Edit", func() {
		showEditCitationDialog(w, s, &citation, refresh)
	})

	deleteBtn := widget.NewButton("Delete", func() {
		dialog.ShowConfirm("Delete Citation",
			"Are you sure you want to delete this citation?",
			func(confirmed bool) {
				if confirmed {
					if err := s.DeleteCitation(citation.ID); err != nil {
						dialog.ShowError(err, w)
					} else {
						refresh()
					}
				}
			}, w)
	})

	buttons := container.NewHBox(editBtn, deleteBtn)

	// Build card
	card := container.NewVBox(titleLabel)
	if detailsWidget != nil {
		card.Add(detailsWidget)
	}
	if transcriptionWidget != nil {
		card.Add(transcriptionWidget)
	}
	if notesWidget != nil {
		card.Add(notesWidget)
	}
	card.Add(buttons)

	return card
}

// showAddCitationDialog shows a dialog to add a new citation.
func showAddCitationDialog(w fyne.Window, s *store.Store, personID int64, onSave func()) {
	// Load all sources
	sources, err := s.GetAllSources()
	if err != nil {
		dialog.ShowError(err, w)
		return
	}

	if len(sources) == 0 {
		dialog.ShowInformation("No Sources",
			"You need to add sources to the Sources Library first.\n\nGo to Media → Sources Library to add sources.",
			w)
		return
	}

	// Create source selection
	sourceNames := make([]string, len(sources))
	for i, source := range sources {
		sourceNames[i] = source.Title
	}

	sourceSelect := widget.NewSelect(sourceNames, func(string) {})
	if len(sourceNames) > 0 {
		sourceSelect.SetSelected(sourceNames[0])
	}

	citationDetailEntry := widget.NewEntry()
	citationDetailEntry.SetPlaceHolder("e.g., Page 123, Line 42, Entry 567")

	transcriptionEntry := widget.NewMultiLineEntry()
	transcriptionEntry.SetPlaceHolder("Transcription of relevant text (optional)")
	transcriptionEntry.SetMinRowsVisible(3)

	confidenceSelect := widget.NewSelect([]string{"low", "medium", "high"}, func(string) {})
	confidenceSelect.SetSelected("medium")

	notesEntry := widget.NewMultiLineEntry()
	notesEntry.SetPlaceHolder("Additional notes about this citation (optional)")
	notesEntry.SetMinRowsVisible(2)

	// Additional people selection (for linking to multiple people at once)
	var selectedPeopleIDs []int64
	selectedPeopleIDs = append(selectedPeopleIDs, personID) // Always include current person

	additionalPeopleLabel := widget.NewLabel("Also link this source to (optional):")
	additionalPeopleLabel.TextStyle.Italic = true

	// Selection count label
	selectionCountLabel := widget.NewLabel("Selected: 1 person (current person always included)")
	selectionCountLabel.TextStyle.Italic = true

	// Search/filter for people
	searchEntry := widget.NewEntry()
	searchEntry.SetPlaceHolder("Type to filter people...")

	// Create checkboxes for other people
	type PersonCheck struct {
		Person store.Person
		Check  *widget.Check
	}
	var peopleChecks []PersonCheck
	peopleContainer := container.NewVBox()

	// Show loading message initially
	loadingLabel := widget.NewLabel("Loading people list...")
	loadingLabel.TextStyle.Italic = true
	peopleContainer.Add(loadingLabel)

	var rebuildPeopleList func(filter string)
	rebuildPeopleList = func(filter string) {
		peopleContainer.Objects = nil

		matchCount := 0
		for _, pc := range peopleChecks {
			// Use accent-insensitive substring matching
			if searchMatch(formatPersonName(pc.Person), filter) {
				peopleContainer.Add(pc.Check)
				matchCount++
			}
		}

		if filter != "" && matchCount == 0 {
			peopleContainer.Add(widget.NewLabel("No matching people found"))
		}

		peopleContainer.Refresh()
	}

	// Load people asynchronously to avoid blocking dialog
	go func() {
		allPeople, err := s.GetPeople()
		if err != nil {
			return
		}

		// Build checkbox list on UI thread
		for _, person := range allPeople {
			if person.ID == personID {
				continue // Skip current person (already included)
			}

			personCopy := person
			check := widget.NewCheck(formatPersonName(personCopy), func(checked bool) {
				if checked {
					// Add to selected
					selectedPeopleIDs = append(selectedPeopleIDs, personCopy.ID)
				} else {
					// Remove from selected
					for i, id := range selectedPeopleIDs {
						if id == personCopy.ID {
							selectedPeopleIDs = append(selectedPeopleIDs[:i], selectedPeopleIDs[i+1:]...)
							break
						}
					}
				}
				// Update count label
				if len(selectedPeopleIDs) == 1 {
					selectionCountLabel.SetText("Selected: 1 person (current person)")
				} else {
					selectionCountLabel.SetText(fmt.Sprintf("Selected: %d people (including current person)", len(selectedPeopleIDs)))
				}
			})
			peopleChecks = append(peopleChecks, PersonCheck{Person: personCopy, Check: check})
		}

		// Rebuild list on UI thread
		rebuildPeopleList("")
	}()

	// Set up search filtering
	searchEntry.OnChanged = func(text string) {
		rebuildPeopleList(text)
	}

	peopleScroll := container.NewVScroll(peopleContainer)
	peopleScroll.SetMinSize(fyne.NewSize(0, 350))

	// Validation error label
	validationLabel := widget.NewLabel("")
	validationLabel.Importance = widget.DangerImportance
	validationLabel.Wrapping = fyne.TextWrapWord
	validationLabel.Hide()

	// Requirement hint
	requirementHint := widget.NewLabel("* At least one field (Citation Detail, Transcription, or Notes) is required")
	requirementHint.Wrapping = fyne.TextWrapWord

	form := container.NewVBox(
		validationLabel, // Error messages appear at top
		requirementHint,
		widget.NewSeparator(),
		widget.NewLabel("Source:"),
		sourceSelect,
		widget.NewLabel("Citation Detail (page, line, entry): *"),
		citationDetailEntry,
		widget.NewLabel("Transcription: *"),
		transcriptionEntry,
		widget.NewLabel("Confidence:"),
		confidenceSelect,
		widget.NewLabel("Notes: *"),
		notesEntry,
		widget.NewSeparator(),
		additionalPeopleLabel,
		selectionCountLabel,
		searchEntry,
		peopleScroll,
	)

	// Wrap in scroll container with fixed size so dialog can scroll
	formScroll := container.NewVScroll(form)
	formScroll.SetMinSize(fyne.NewSize(730, 820))

	// Create custom window instead of dialog
	citationWindow := fyne.CurrentApp().NewWindow("Add Source Citation")
	citationWindow.Resize(fyne.NewSize(750, 900))

	// Save button handler
	saveBtn := widget.NewButton("Save", func() {
		// Clear any previous validation error
		validationLabel.SetText("")
		validationLabel.Hide()

		// Find selected source
		var selectedSource *store.Source
		for i, name := range sourceNames {
			if name == sourceSelect.Selected {
				selectedSource = &sources[i]
				break
			}
		}

		if selectedSource == nil {
			validationLabel.SetText("❌ Please select a source")
			validationLabel.Show()
			formScroll.ScrollToTop()
			return
		}

		// Create citations for all selected people
		citationDetail := strings.TrimSpace(citationDetailEntry.Text)
		transcription := strings.TrimSpace(transcriptionEntry.Text)
		confidence := confidenceSelect.Selected
		citationNotes := strings.TrimSpace(notesEntry.Text)

		// Require at least one substantive field
		if citationDetail == "" && transcription == "" && citationNotes == "" {
			validationLabel.SetText("❌ Please enter at least one of: Citation Detail, Transcription, or Notes")
			validationLabel.Show()
			formScroll.ScrollToTop()
			return
		}

		successCount := 0
		for _, pID := range selectedPeopleIDs {
			citation := &store.Citation{
				SourceID:       selectedSource.ID,
				PersonID:       pID,
				CitationDetail: citationDetail,
				Transcription:  transcription,
				Confidence:     confidence,
				Notes:          citationNotes,
			}

			if err := s.CreateCitation(citation); err == nil {
				successCount++
			}
		}

		if successCount > 0 {
			citationWindow.Close()
			onSave()
			if successCount > 1 {
				dialog.ShowInformation("Citations Added",
					fmt.Sprintf("Successfully linked source to %d people.", successCount),
					w)
			}
		} else {
			validationLabel.SetText("❌ Failed to create citations")
			validationLabel.Show()
			formScroll.ScrollToTop()
		}
	})

	cancelBtn := widget.NewButton("Cancel", func() {
		citationWindow.Close()
	})

	buttons := container.NewHBox(saveBtn, cancelBtn)
	content := container.NewBorder(nil, buttons, nil, nil, formScroll)
	citationWindow.SetContent(content)
	citationWindow.Show()
}

// showEditCitationDialog shows a dialog to edit an existing citation.
func showEditCitationDialog(w fyne.Window, s *store.Store, citation *store.Citation, onSave func()) {
	citationDetailEntry := widget.NewEntry()
	citationDetailEntry.SetText(citation.CitationDetail)

	transcriptionEntry := widget.NewMultiLineEntry()
	transcriptionEntry.SetText(citation.Transcription)
	transcriptionEntry.SetMinRowsVisible(3)

	confidenceSelect := widget.NewSelect([]string{"low", "medium", "high"}, func(string) {})
	confidenceSelect.SetSelected(citation.Confidence)

	notesEntry := widget.NewMultiLineEntry()
	notesEntry.SetText(citation.Notes)
	notesEntry.SetMinRowsVisible(2)

	// Validation error label
	validationLabel := widget.NewLabel("")
	validationLabel.Importance = widget.DangerImportance
	validationLabel.Wrapping = fyne.TextWrapWord
	validationLabel.Hide()

	form := container.NewVBox(
		validationLabel,
		widget.NewLabel("Citation Detail (page, line, entry):"),
		citationDetailEntry,
		widget.NewLabel("Transcription:"),
		transcriptionEntry,
		widget.NewLabel("Confidence:"),
		confidenceSelect,
		widget.NewLabel("Notes:"),
		notesEntry,
	)

	// Create custom window instead of dialog
	editWindow := fyne.CurrentApp().NewWindow("Edit Citation")
	editWindow.Resize(fyne.NewSize(700, 500))

	formScroll := container.NewVScroll(form)

	// Save button handler
	saveBtn := widget.NewButton("Save", func() {
		// Clear any previous validation error
		validationLabel.SetText("")
		validationLabel.Hide()

		citation.CitationDetail = strings.TrimSpace(citationDetailEntry.Text)
		citation.Transcription = strings.TrimSpace(transcriptionEntry.Text)
		citation.Confidence = confidenceSelect.Selected
		citation.Notes = strings.TrimSpace(notesEntry.Text)

		if err := s.UpdateCitation(citation); err != nil {
			validationLabel.SetText(fmt.Sprintf("❌ Failed to update: %v", err))
			validationLabel.Show()
			formScroll.ScrollToTop()
			return
		}

		editWindow.Close()
		onSave()
	})

	cancelBtn := widget.NewButton("Cancel", func() {
		editWindow.Close()
	})

	buttons := container.NewHBox(saveBtn, cancelBtn)
	content := container.NewBorder(nil, buttons, nil, nil, formScroll)
	editWindow.SetContent(content)
	editWindow.Show()
}

// showAllSourcesReport shows a report of all sources with citation counts.
var allSourcesReportWindow fyne.Window

func showAllSourcesReport(parentWindow fyne.Window, s *store.Store, navigateFunc func(int64)) {
	// Check if already open
	if allSourcesReportWindow != nil {
		allSourcesReportWindow.RequestFocus()
		allSourcesReportWindow.Show()
		return
	}

	// Load all sources
	sources, err := s.GetAllSources()
	if err != nil {
		dialog.ShowError(err, parentWindow)
		return
	}

	content := container.NewVBox()

	title := widget.NewLabelWithStyle("All Sources",
		fyne.TextAlignCenter, fyne.TextStyle{Bold: true})
	content.Add(title)
	content.Add(widget.NewSeparator())

	if len(sources) == 0 {
		content.Add(widget.NewLabel("No sources in library."))
		content.Add(widget.NewLabel("Add sources via Media → Sources Library."))
	} else {
		summary := widget.NewLabel(fmt.Sprintf("Showing %d sources:", len(sources)))
		content.Add(summary)
		content.Add(widget.NewSeparator())

		// Count total citations
		totalCitations := 0
		for _, source := range sources {
			citations, _ := s.GetCitationsForSource(source.ID)
			totalCitations += len(citations)
		}
		statsLabel := widget.NewLabel(fmt.Sprintf("Total citations: %d", totalCitations))
		statsLabel.TextStyle.Italic = true
		content.Add(statsLabel)
		content.Add(widget.NewSeparator())

		// Display each source
		for _, source := range sources {
			sourceCopy := source // Capture for closure

			// Get citation count
			citations, _ := s.GetCitationsForSource(sourceCopy.ID)
			citationCount := len(citations)

			// Source type icon
			typeIcon := "📄"
			switch sourceCopy.SourceType {
			case "vital_record":
				typeIcon = "📜"
			case "census":
				typeIcon = "📊"
			case "church":
				typeIcon = "⛪"
			case "book":
				typeIcon = "📚"
			case "website":
				typeIcon = "🌐"
			}

			// Source info
			sourceText := fmt.Sprintf("%s %s (%d citations)", typeIcon, sourceCopy.Title, citationCount)
			sourceLabel := widget.NewLabelWithStyle(sourceText, fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
			sourceLabel.Wrapping = fyne.TextWrapWord
			content.Add(sourceLabel)

			// Details
			var details []string
			if sourceCopy.Author != "" {
				details = append(details, "Author: "+sourceCopy.Author)
			}
			if sourceCopy.Repository != "" {
				details = append(details, "Repository: "+sourceCopy.Repository)
			}
			if len(details) > 0 {
				detailsLabel := widget.NewLabel("    " + strings.Join(details, " | "))
				detailsLabel.TextStyle.Italic = true
				detailsLabel.Wrapping = fyne.TextWrapWord
				content.Add(detailsLabel)
			}

			// List people cited
			if citationCount > 0 {
				peopleLabel := widget.NewLabel("    People cited: ")
				peopleLabel.TextStyle.Italic = true
				content.Add(peopleLabel)

				for i, citation := range citations {
					if i >= 5 { // Limit to first 5
						moreLabel := widget.NewLabel(fmt.Sprintf("        ... and %d more", citationCount-5))
						moreLabel.TextStyle.Italic = true
						content.Add(moreLabel)
						break
					}

					citationCopy := citation
					person, _ := s.GetPersonByID(citationCopy.PersonID)
					if person != nil {
						personBtn := widget.NewButton(
							fmt.Sprintf("        → %s", formatPersonName(*person)),
							func() {
								navigateFunc(citationCopy.PersonID)
								if allSourcesReportWindow != nil {
									allSourcesReportWindow.Close()
								}
							})
						content.Add(personBtn)
					}
				}
			}

			content.Add(widget.NewSeparator())
		}
	}

	scroll := container.NewVScroll(content)
	scroll.SetMinSize(fyne.NewSize(700, 400))

	allSourcesReportWindow = fyne.CurrentApp().NewWindow("All Sources Report")
	allSourcesReportWindow.SetContent(scroll)
	allSourcesReportWindow.Resize(fyne.NewSize(900, 600))
	allSourcesReportWindow.SetOnClosed(func() {
		allSourcesReportWindow = nil
	})
	allSourcesReportWindow.Show()
}

// showWellDocumentedPeopleReport shows people with the most source citations.
var wellDocumentedReportWindow fyne.Window

func showWellDocumentedPeopleReport(parentWindow fyne.Window, s *store.Store, navigateFunc func(int64)) {
	// Check if already open
	if wellDocumentedReportWindow != nil {
		wellDocumentedReportWindow.RequestFocus()
		wellDocumentedReportWindow.Show()
		return
	}

	// Get all people
	allPeople, err := s.GetPeople()
	if err != nil {
		dialog.ShowError(err, parentWindow)
		return
	}

	// Count citations for each person
	type PersonCitationCount struct {
		Person        store.Person
		CitationCount int
	}

	var peopleCitations []PersonCitationCount
	for _, person := range allPeople {
		count, _ := s.CountCitationsForPerson(person.ID)
		if count > 0 {
			peopleCitations = append(peopleCitations, PersonCitationCount{
				Person:        person,
				CitationCount: count,
			})
		}
	}

	// Sort by citation count (highest first)
	sort.Slice(peopleCitations, func(i, j int) bool {
		return peopleCitations[i].CitationCount > peopleCitations[j].CitationCount
	})

	content := container.NewVBox()

	title := widget.NewLabelWithStyle("Well-Documented People",
		fyne.TextAlignCenter, fyne.TextStyle{Bold: true})
	content.Add(title)
	content.Add(widget.NewSeparator())

	if len(peopleCitations) == 0 {
		content.Add(widget.NewLabel("No people have source citations yet."))
		content.Add(widget.NewLabel("Add citations via the 📚 Manage Sources button."))
	} else {
		summary := widget.NewLabel(fmt.Sprintf("Showing %d people with sources (out of %d total):",
			len(peopleCitations), len(allPeople)))
		content.Add(summary)

		avgCitations := 0
		for _, pc := range peopleCitations {
			avgCitations += pc.CitationCount
		}
		avgCitations = avgCitations / len(peopleCitations)
		statsLabel := widget.NewLabel(fmt.Sprintf("Average citations per documented person: %d", avgCitations))
		statsLabel.TextStyle.Italic = true
		content.Add(statsLabel)
		content.Add(widget.NewSeparator())

		// Show top people
		for i, pc := range peopleCitations {
			if i >= 50 { // Limit to top 50
				break
			}

			pcCopy := pc
			personName := formatPersonName(pcCopy.Person)
			info := formatPersonInfo(pcCopy.Person)

			buttonText := fmt.Sprintf("%d. %s - %d sources", i+1, personName, pcCopy.CitationCount)
			if info != "" {
				buttonText += fmt.Sprintf(" (%s)", info)
			}

			personBtn := widget.NewButton(buttonText, func() {
				navigateFunc(pcCopy.Person.ID)
				if wellDocumentedReportWindow != nil {
					wellDocumentedReportWindow.Close()
				}
			})

			content.Add(personBtn)
		}

		if len(peopleCitations) > 50 {
			moreLabel := widget.NewLabel(fmt.Sprintf("... and %d more people with sources", len(peopleCitations)-50))
			moreLabel.TextStyle.Italic = true
			content.Add(moreLabel)
		}
	}

	scroll := container.NewVScroll(content)
	scroll.SetMinSize(fyne.NewSize(700, 400))

	wellDocumentedReportWindow = fyne.CurrentApp().NewWindow("Well-Documented People")
	wellDocumentedReportWindow.SetContent(scroll)
	wellDocumentedReportWindow.Resize(fyne.NewSize(900, 600))
	wellDocumentedReportWindow.SetOnClosed(func() {
		wellDocumentedReportWindow = nil
	})
	wellDocumentedReportWindow.Show()
}
