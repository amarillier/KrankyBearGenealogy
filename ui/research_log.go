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

// Track open research log windows
var researchLogWindow fyne.Window

// showResearchLogManager shows the main research log manager window.
func showResearchLogManager(parentWindow fyne.Window, s *store.Store) {
	// Check if window already open
	if researchLogWindow != nil {
		researchLogWindow.RequestFocus()
		researchLogWindow.Show()
		return
	}

	// Create new window
	researchLogWindow = fyne.CurrentApp().NewWindow("Research Log")

	// Function to rebuild content
	var rebuildContent func()
	rebuildContent = func() {
		// Reload logs
		logs, err := s.GetAllResearchLogs()
		if err != nil {
			dialog.ShowError(fmt.Errorf("Failed to load research logs: %w", err), parentWindow)
			return
		}

		content := container.NewVBox()

		// Header
		header := widget.NewLabelWithStyle(
			fmt.Sprintf("Research Log (%d entries)", len(logs)),
			fyne.TextAlignCenter, fyne.TextStyle{Bold: true})
		content.Add(header)
		content.Add(widget.NewSeparator())

		// Add New button
		addBtn := widget.NewButton("+ Add Research Log Entry", func() {
			showAddResearchLogDialog(researchLogWindow, s, nil, rebuildContent)
		})
		content.Add(addBtn)
		content.Add(widget.NewSeparator())

		if len(logs) == 0 {
			content.Add(widget.NewLabel("No research log entries yet."))
			content.Add(widget.NewLabel("Click '+ Add Research Log Entry' to record your research activities."))
		} else {
			// Display logs
			for _, log := range logs {
				logCopy := log // Capture for closure
				logCard := makeResearchLogCard(logCopy, s, rebuildContent, researchLogWindow)
				content.Add(logCard)
				content.Add(widget.NewSeparator())
			}
		}

		scroll := container.NewVScroll(content)
		scroll.SetMinSize(fyne.NewSize(800, 800))
		researchLogWindow.SetContent(scroll)
	}

	rebuildContent()
	researchLogWindow.Resize(fyne.NewSize(900, 900))
	researchLogWindow.SetOnClosed(func() {
		researchLogWindow = nil
	})

	researchLogWindow.Show()
}

// makeResearchLogCard creates a UI card for a research log entry.
func makeResearchLogCard(log store.ResearchLog, s *store.Store, onUpdate func(), w fyne.Window) *fyne.Container {
	// Format search date
	searchDateStr := log.SearchDate.Format("January 2, 2006")

	// Get linked people
	linkedPeople, err := s.GetPeopleForResearchLog(log.ID)
	personText := "General research"
	if err == nil && len(linkedPeople) > 0 {
		names := make([]string, len(linkedPeople))
		for i, p := range linkedPeople {
			names[i] = fmt.Sprintf("%s %s", p.GivenName, p.Surname)
		}
		if len(linkedPeople) == 1 {
			personText = names[0]
		} else {
			personText = fmt.Sprintf("%d people: %s", len(linkedPeople), strings.Join(names, ", "))
		}
	}

	// Build header
	header := widget.NewLabelWithStyle(
		fmt.Sprintf("📅 %s - %s", searchDateStr, log.Repository),
		fyne.TextAlignLeading, fyne.TextStyle{Bold: true})

	// Person line
	personLabel := widget.NewLabel(fmt.Sprintf("People: %s", personText))
	personLabel.Wrapping = fyne.TextWrapWord

	// Record type if specified
	recordTypeLabel := widget.NewLabel("")
	if log.RecordType != "" {
		recordTypeLabel.SetText(fmt.Sprintf("Record Type: %s", log.RecordType))
	}

	// Search goal
	goalLabel := widget.NewLabel(fmt.Sprintf("Looking for: %s", log.SearchGoal))
	goalLabel.Wrapping = fyne.TextWrapWord

	// Results
	resultsLabel := widget.NewLabel(fmt.Sprintf("Results: %s", log.Results))
	resultsLabel.Wrapping = fyne.TextWrapWord

	// Notes if specified
	notesLabel := widget.NewLabel("")
	if log.Notes != "" {
		notesLabel.SetText(fmt.Sprintf("Notes: %s", log.Notes))
		notesLabel.Wrapping = fyne.TextWrapWord
	}

	// Edit and Delete buttons
	editBtn := widget.NewButton("Edit", func() {
		showEditResearchLogDialog(w, s, &log, onUpdate)
	})

	deleteBtn := widget.NewButton("Delete", func() {
		dialog.ShowConfirm("Delete Research Log Entry",
			"Are you sure you want to delete this research log entry?",
			func(confirmed bool) {
				if confirmed {
					if err := s.DeleteResearchLog(log.ID); err != nil {
						dialog.ShowError(err, w)
					} else {
						onUpdate()
					}
				}
			}, w)
	})

	buttons := container.NewHBox(editBtn, deleteBtn)

	card := container.NewVBox(
		header,
		personLabel,
		recordTypeLabel,
		goalLabel,
		resultsLabel,
		notesLabel,
		buttons,
	)

	return card
}

// showAddResearchLogDialog shows the dialog to add a new research log entry.
func showAddResearchLogDialog(w fyne.Window, s *store.Store, defaultPersonID *int64, onSave func()) {
	// Get all people for selection
	allPeople, err := s.GetPeople()
	if err != nil {
		dialog.ShowError(err, w)
		return
	}

	// Search date picker (default to today)
	searchDate := time.Now()
	searchDateEntry := widget.NewEntry()
	searchDateEntry.SetText(searchDate.Format("2006-01-02"))
	searchDateEntry.SetPlaceHolder("YYYY-MM-DD")

	// Repository
	repositoryEntry := widget.NewEntry()
	repositoryEntry.SetPlaceHolder("e.g., FamilySearch, Ancestry.com, National Archives, Local Library")

	// Record type
	recordTypeEntry := widget.NewEntry()
	recordTypeEntry.SetPlaceHolder("e.g., Census, Vital Records, Newspapers, Church Records (optional)")

	// Search goal
	searchGoalEntry := widget.NewMultiLineEntry()
	searchGoalEntry.SetPlaceHolder("What were you looking for?")
	searchGoalEntry.SetMinRowsVisible(2)

	// Results
	resultsEntry := widget.NewMultiLineEntry()
	resultsEntry.SetPlaceHolder("What did you find? (or 'Nothing found')")
	resultsEntry.SetMinRowsVisible(3)

	// Notes
	notesEntry := widget.NewMultiLineEntry()
	notesEntry.SetPlaceHolder("Additional notes, URLs, reference numbers (optional)")
	notesEntry.SetMinRowsVisible(2)

	// Multi-person selection (for linking to multiple people at once)
	var selectedPeopleIDs []int64
	if defaultPersonID != nil && *defaultPersonID > 0 {
		selectedPeopleIDs = append(selectedPeopleIDs, *defaultPersonID) // Include default person
	}

	additionalPeopleLabel := widget.NewLabel("Link to people (optional - for census, marriage records, etc.):")
	additionalPeopleLabel.TextStyle.Italic = true

	// Selection count label
	selectionCountText := "Selected: 0 people"
	if defaultPersonID != nil && *defaultPersonID > 0 {
		selectionCountText = "Selected: 1 person"
	}
	selectionCountLabel := widget.NewLabel(selectionCountText)
	selectionCountLabel.TextStyle.Italic = true

	// Search/filter for people
	searchEntry := widget.NewEntry()
	searchEntry.SetPlaceHolder("Type to filter people...")

	// Create checkboxes for people
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
		// Build checkbox list and update UI on main thread
		fyne.Do(func() {
			for _, person := range allPeople {
				personCopy := person

				// Check if this person should be pre-selected
				isPreSelected := false
				if defaultPersonID != nil && personCopy.ID == *defaultPersonID {
					isPreSelected = true
				}

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
					if len(selectedPeopleIDs) == 0 {
						selectionCountLabel.SetText("Selected: 0 people")
					} else if len(selectedPeopleIDs) == 1 {
						selectionCountLabel.SetText("Selected: 1 person")
					} else {
						selectionCountLabel.SetText(fmt.Sprintf("Selected: %d people", len(selectedPeopleIDs)))
					}
				})
				check.Checked = isPreSelected
				peopleChecks = append(peopleChecks, PersonCheck{Person: personCopy, Check: check})
			}

			// Rebuild list
			rebuildPeopleList("")
		})
	}()

	// Set up search filtering
	searchEntry.OnChanged = func(text string) {
		rebuildPeopleList(text)
	}

	peopleScroll := container.NewVScroll(peopleContainer)
	peopleScroll.SetMinSize(fyne.NewSize(0, 200))

	// Validation error label
	validationLabel := widget.NewLabel("")
	validationLabel.Importance = widget.DangerImportance
	validationLabel.Wrapping = fyne.TextWrapWord
	validationLabel.Hide()

	form := container.NewVBox(
		validationLabel, // Error messages appear at top
		widget.NewLabel("Search Date: *"),
		searchDateEntry,
		widget.NewLabel("Repository/Location: *"),
		repositoryEntry,
		widget.NewLabel("Record Type (optional):"),
		recordTypeEntry,
		widget.NewLabel("What were you looking for: *"),
		searchGoalEntry,
		widget.NewLabel("Results: *"),
		resultsEntry,
		widget.NewLabel("Notes (optional):"),
		notesEntry,
		widget.NewSeparator(),
		additionalPeopleLabel,
		selectionCountLabel,
		searchEntry,
		peopleScroll,
	)

	// Wrap in scroll container
	formScroll := container.NewVScroll(form)
	formScroll.SetMinSize(fyne.NewSize(730, 820))

	// Create a custom window instead of dialog for better validation control
	addWindow := fyne.CurrentApp().NewWindow("Add Research Log Entry")
	addWindow.Resize(fyne.NewSize(750, 920))

	// Save button handler
	saveBtn := widget.NewButton("Save", func() {
		// Clear any previous validation error
		validationLabel.SetText("")
		validationLabel.Hide()

		// Validate inputs - show inline errors
		parsedDate, err := time.Parse("2006-01-02", strings.TrimSpace(searchDateEntry.Text))
		if err != nil {
			validationLabel.SetText("❌ Invalid date format. Please use YYYY-MM-DD")
			validationLabel.Show()
			formScroll.ScrollToTop()
			return
		}

		repository := strings.TrimSpace(repositoryEntry.Text)
		if repository == "" {
			validationLabel.SetText("❌ Repository is required")
			validationLabel.Show()
			formScroll.ScrollToTop()
			return
		}

		searchGoal := strings.TrimSpace(searchGoalEntry.Text)
		if searchGoal == "" {
			validationLabel.SetText("❌ 'What were you looking for' is required")
			validationLabel.Show()
			formScroll.ScrollToTop()
			return
		}

		results := strings.TrimSpace(resultsEntry.Text)
		if results == "" {
			validationLabel.SetText("❌ Results field is required (or enter 'Nothing found')")
			validationLabel.Show()
			formScroll.ScrollToTop()
			return
		}

		// Create the research log entry
		log := &store.ResearchLog{
			PersonID:   nil, // Legacy field, not used with multi-person linking
			SearchDate: parsedDate,
			Repository: repository,
			RecordType: strings.TrimSpace(recordTypeEntry.Text),
			SearchGoal: searchGoal,
			Results:    results,
			Notes:      strings.TrimSpace(notesEntry.Text),
		}

		if err := s.CreateResearchLog(log); err != nil {
			validationLabel.SetText(fmt.Sprintf("❌ Failed to save: %v", err))
			validationLabel.Show()
			formScroll.ScrollToTop()
			return
		}

		// Link to selected people
		successCount := 0
		for _, personID := range selectedPeopleIDs {
			if err := s.LinkResearchLogToPerson(log.ID, personID); err == nil {
				successCount++
			}
		}

		// Close window and refresh
		addWindow.Close()
		onSave()
		if successCount > 1 {
			dialog.ShowInformation("Research Log Added",
				fmt.Sprintf("Successfully linked to %d people.", successCount),
				w)
		}
	})

	cancelBtn := widget.NewButton("Cancel", func() {
		addWindow.Close()
	})

	buttons := container.NewHBox(saveBtn, cancelBtn)
	content := container.NewBorder(nil, buttons, nil, nil, formScroll)
	addWindow.SetContent(content)
	addWindow.Show()
}

// showEditResearchLogDialog shows the dialog to edit an existing research log entry.
func showEditResearchLogDialog(w fyne.Window, s *store.Store, log *store.ResearchLog, onSave func()) {
	// Get all people for selection
	allPeople, err := s.GetPeople()
	if err != nil {
		dialog.ShowError(err, w)
		return
	}

	// Search date picker
	searchDateEntry := widget.NewEntry()
	searchDateEntry.SetText(log.SearchDate.Format("2006-01-02"))
	searchDateEntry.SetPlaceHolder("YYYY-MM-DD")

	// Repository
	repositoryEntry := widget.NewEntry()
	repositoryEntry.SetText(log.Repository)
	repositoryEntry.SetPlaceHolder("e.g., FamilySearch, Ancestry.com, National Archives, Local Library")

	// Record type
	recordTypeEntry := widget.NewEntry()
	recordTypeEntry.SetText(log.RecordType)
	recordTypeEntry.SetPlaceHolder("e.g., Census, Vital Records, Newspapers, Church Records (optional)")

	// Search goal
	searchGoalEntry := widget.NewMultiLineEntry()
	searchGoalEntry.SetText(log.SearchGoal)
	searchGoalEntry.SetPlaceHolder("What were you looking for?")
	searchGoalEntry.SetMinRowsVisible(2)

	// Results
	resultsEntry := widget.NewMultiLineEntry()
	resultsEntry.SetText(log.Results)
	resultsEntry.SetPlaceHolder("What did you find? (or 'Nothing found')")
	resultsEntry.SetMinRowsVisible(3)

	// Notes
	notesEntry := widget.NewMultiLineEntry()
	notesEntry.SetText(log.Notes)
	notesEntry.SetPlaceHolder("Additional notes, URLs, reference numbers (optional)")
	notesEntry.SetMinRowsVisible(2)

	// Get currently linked people
	currentlyLinked, _ := s.GetPeopleForResearchLog(log.ID)
	
	currentLinkedIDs := make(map[int64]bool)
	linkedNames := []string{}
	for _, p := range currentlyLinked {
		currentLinkedIDs[p.ID] = true
		linkedNames = append(linkedNames, formatPersonName(p))
	}

	// Multi-person selection with checkboxes
	selectedPeopleIDs := make(map[int64]bool)
	for id := range currentLinkedIDs {
		selectedPeopleIDs[id] = true
	}

	// Container for currently linked people chips
	linkedPeopleChipsContainer := container.NewVBox()
	selectionCountLabel := widget.NewLabel(fmt.Sprintf("Selected: %d people", len(selectedPeopleIDs)))
	
	// Declare functions
	var refreshLinkedChips func()
	var updateSelectionCount func()
	
	refreshLinkedChips = func() {
		linkedPeopleChipsContainer.Objects = nil
		
		if len(selectedPeopleIDs) == 0 {
			linkedPeopleChipsContainer.Add(widget.NewLabel("Currently linked to: General research (no specific people)"))
		} else {
			linkedPeopleChipsContainer.Add(widget.NewLabel(fmt.Sprintf("Currently linked to %d people:", len(selectedPeopleIDs))))
			
			// Create a chip for each selected person (need to get their names from allPeople)
			chipsFlow := container.NewGridWrap(fyne.NewSize(250, 35))
			for _, p := range allPeople {
				if selectedPeopleIDs[p.ID] {
					person := p // Capture for closure
					personName := formatPersonName(person)
					
					// Create a button with person name and X to remove
					removeBtn := widget.NewButton("✕ "+personName, func() {
						delete(selectedPeopleIDs, person.ID)
						updateSelectionCount()
					})
					removeBtn.Importance = widget.LowImportance
					chipsFlow.Add(removeBtn)
				}
			}
			linkedPeopleChipsContainer.Add(chipsFlow)
		}
		linkedPeopleChipsContainer.Refresh()
	}

	updateSelectionCount = func() {
		count := len(selectedPeopleIDs)
		if count == 0 {
			selectionCountLabel.SetText("Selected: None (will be general research)")
		} else {
			selectionCountLabel.SetText(fmt.Sprintf("Selected: %d people", count))
		}
		refreshLinkedChips()
	}
	
	// Initial display
	refreshLinkedChips()

	peopleChecks := make([]*widget.Check, 0)
	peopleContainer := container.NewVBox()

	// Search filter for people
	searchEntry := widget.NewEntry()
	searchEntry.SetPlaceHolder("Search people by name...")

	var filterPeople func(string)
	filterPeople = func(searchText string) {
		peopleContainer.Objects = nil
		peopleChecks = make([]*widget.Check, 0)

		for _, p := range allPeople {
			person := p
			personName := formatPersonName(person)

			// Filter by search text (accent-insensitive, substring matching)
			if !searchMatch(personName, searchText) {
				continue
			}

			isChecked := selectedPeopleIDs[person.ID]
			check := widget.NewCheck(personName, func(checked bool) {
				if checked {
					selectedPeopleIDs[person.ID] = true
				} else {
					delete(selectedPeopleIDs, person.ID)
				}
				updateSelectionCount()
			})
			check.Checked = isChecked
			peopleChecks = append(peopleChecks, check)
			peopleContainer.Add(check)
		}
		peopleContainer.Refresh()
	}

	searchEntry.OnChanged = filterPeople
	filterPeople("") // Initial population

	peopleScroll := container.NewVScroll(peopleContainer)
	peopleScroll.SetMinSize(fyne.NewSize(600, 200))

	additionalPeopleLabel := widget.NewLabel("Link to people (optional, check to add/remove):")

	// Validation error label
	validationLabel := widget.NewLabel("")
	validationLabel.Importance = widget.DangerImportance
	validationLabel.Wrapping = fyne.TextWrapWord
	validationLabel.Hide()

	form := container.NewVBox(
		validationLabel,
		linkedPeopleChipsContainer,
		widget.NewSeparator(),
		widget.NewLabel("Search Date: *"),
		searchDateEntry,
		widget.NewLabel("Repository/Location: *"),
		repositoryEntry,
		widget.NewLabel("Record Type (optional):"),
		recordTypeEntry,
		widget.NewLabel("What were you looking for: *"),
		searchGoalEntry,
		widget.NewLabel("Results: *"),
		resultsEntry,
		widget.NewLabel("Notes (optional):"),
		notesEntry,
		widget.NewSeparator(),
		additionalPeopleLabel,
		selectionCountLabel,
		searchEntry,
		peopleScroll,
	)

	// Create a custom window instead of dialog
	editWindow := fyne.CurrentApp().NewWindow("Edit Research Log Entry")
	editWindow.Resize(fyne.NewSize(700, 650))

	formScroll := container.NewVScroll(form)

	// Save button handler
	saveBtn := widget.NewButton("Save", func() {
		// Clear any previous validation error
		validationLabel.SetText("")
		validationLabel.Hide()

		// Validate inputs
		parsedDate, err := time.Parse("2006-01-02", strings.TrimSpace(searchDateEntry.Text))
		if err != nil {
			validationLabel.SetText("❌ Invalid date format. Please use YYYY-MM-DD")
			validationLabel.Show()
			formScroll.ScrollToTop()
			return
		}

		repository := strings.TrimSpace(repositoryEntry.Text)
		if repository == "" {
			validationLabel.SetText("❌ Repository is required")
			validationLabel.Show()
			formScroll.ScrollToTop()
			return
		}

		searchGoal := strings.TrimSpace(searchGoalEntry.Text)
		if searchGoal == "" {
			validationLabel.SetText("❌ 'What were you looking for' is required")
			validationLabel.Show()
			formScroll.ScrollToTop()
			return
		}

		results := strings.TrimSpace(resultsEntry.Text)
		if results == "" {
			validationLabel.SetText("❌ Results field is required (or enter 'Nothing found')")
			validationLabel.Show()
			formScroll.ScrollToTop()
			return
		}

		log.PersonID = nil // Legacy field, not used with multi-person linking
		log.SearchDate = parsedDate
		log.Repository = repository
		log.RecordType = strings.TrimSpace(recordTypeEntry.Text)
		log.SearchGoal = searchGoal
		log.Results = results
		log.Notes = strings.TrimSpace(notesEntry.Text)

		if err := s.UpdateResearchLog(log); err != nil {
			validationLabel.SetText(fmt.Sprintf("❌ Failed to update: %v", err))
			validationLabel.Show()
			formScroll.ScrollToTop()
			return
		}

		// Update people links - remove old links and add new ones
		// First, remove all existing links
		for _, p := range currentlyLinked {
			s.UnlinkResearchLogFromPerson(log.ID, p.ID)
		}

		// Add new links based on selection
		successCount := 0
		for personID := range selectedPeopleIDs {
			if err := s.LinkResearchLogToPerson(log.ID, personID); err == nil {
				successCount++
			}
		}

		// Close window and refresh
		editWindow.Close()
		onSave()
		if successCount > 1 {
			dialog.ShowInformation("Research Log Updated",
				fmt.Sprintf("Successfully linked to %d people.", successCount),
				w)
		}
	})

	cancelBtn := widget.NewButton("Cancel", func() {
		editWindow.Close()
	})

	buttons := container.NewHBox(saveBtn, cancelBtn)
	content := container.NewBorder(nil, buttons, nil, nil, formScroll)
	editWindow.SetContent(content)
	editWindow.Show()
}

// showRecentResearchReport shows a report of recent research activities.
func showRecentResearchReport(parentWindow fyne.Window, s *store.Store) {
	logs, err := s.GetRecentResearchLogs(50) // Show last 50 entries
	if err != nil {
		dialog.ShowError(err, parentWindow)
		return
	}

	w := fyne.CurrentApp().NewWindow("Recent Research Activity")

	content := container.NewVBox()

	// Header
	header := widget.NewLabelWithStyle(
		fmt.Sprintf("Recent Research Activity (%d entries)", len(logs)),
		fyne.TextAlignCenter, fyne.TextStyle{Bold: true})
	content.Add(header)
	content.Add(widget.NewSeparator())

	if len(logs) == 0 {
		content.Add(widget.NewLabel("No research log entries yet."))
	} else {
		for _, log := range logs {
			logCopy := log // Capture for closure

			// Get person name if linked
			personName := "General"
			if logCopy.PersonID != nil {
				person, err := s.GetPersonByID(*logCopy.PersonID)
				if err == nil {
					personName = fmt.Sprintf("%s %s", person.GivenName, person.Surname)
				}
			}

			dateStr := logCopy.SearchDate.Format("Jan 2, 2006")
			text := fmt.Sprintf("📅 %s | %s | %s\n   Goal: %s\n   Results: %s",
				dateStr, personName, logCopy.Repository, logCopy.SearchGoal, logCopy.Results)

			label := widget.NewLabel(text)
			label.Wrapping = fyne.TextWrapWord

			content.Add(label)
			content.Add(widget.NewSeparator())
		}
	}

	scroll := container.NewVScroll(content)
	scroll.SetMinSize(fyne.NewSize(700, 500))

	w.SetContent(scroll)
	w.Resize(fyne.NewSize(800, 600))
	w.Show()
}

// showPeopleWithResearchLogsReport shows all people who have research log entries.
func showPeopleWithResearchLogsReport(parentWindow fyne.Window, s *store.Store, navigateFunc func(int64)) {
	people, err := s.GetPeople()
	if err != nil {
		dialog.ShowError(fmt.Errorf("Failed to load people: %w", err), parentWindow)
		return
	}

	// Find people with research logs
	type personWithLogs struct {
		person   store.Person
		logCount int
	}

	var peopleWithLogs []personWithLogs

	for _, p := range people {
		logs, err := s.GetResearchLogsForPerson(p.ID)
		if err == nil && len(logs) > 0 {
			peopleWithLogs = append(peopleWithLogs, personWithLogs{
				person:   p,
				logCount: len(logs),
			})
		}
	}

	// Sort by log count (most logs first)
	sort.Slice(peopleWithLogs, func(i, j int) bool {
		return peopleWithLogs[i].logCount > peopleWithLogs[j].logCount
	})

	w := fyne.CurrentApp().NewWindow("People with Research Logs")

	content := container.NewVBox()

	// Header
	header := widget.NewLabelWithStyle(
		fmt.Sprintf("People with Research Logs (%d people)", len(peopleWithLogs)),
		fyne.TextAlignCenter, fyne.TextStyle{Bold: true})
	content.Add(header)
	content.Add(widget.NewSeparator())

	if len(peopleWithLogs) == 0 {
		content.Add(widget.NewLabel("No people have research log entries yet."))
		content.Add(widget.NewLabel("Add research logs from individual person edit dialogs or Family View."))
	} else {
		summary := widget.NewLabel(fmt.Sprintf("Showing %d people with research documentation:", len(peopleWithLogs)))
		content.Add(summary)
		content.Add(widget.NewSeparator())

		for _, pwl := range peopleWithLogs {
			pwlCopy := pwl // Capture for closure

			nameBtn := widget.NewButton(
				formatPersonName(pwl.person),
				func() {
					navigateFunc(pwlCopy.person.ID)
					w.Close()
				})

			logCountLabel := widget.NewLabel(fmt.Sprintf("    %d research log %s",
				pwl.logCount,
				map[bool]string{true: "entry", false: "entries"}[pwl.logCount == 1]))
			logCountLabel.TextStyle.Italic = true

			viewLogsBtn := widget.NewButton("View Research Log", func() {
				showResearchLogForPerson(w, s, pwlCopy.person.ID, formatPersonName(pwlCopy.person))
			})

			personBox := container.NewVBox(
				nameBtn,
				logCountLabel,
				viewLogsBtn,
			)
			content.Add(personBox)
			content.Add(widget.NewSeparator())
		}
	}

	scroll := container.NewVScroll(content)
	scroll.SetMinSize(fyne.NewSize(600, 500))

	w.SetContent(scroll)
	w.Resize(fyne.NewSize(650, 600))
	w.Show()
}

// showResearchLogForPerson shows research log entries for a specific person.
func showResearchLogForPerson(parentWindow fyne.Window, s *store.Store, personID int64, personName string) {
	w := fyne.CurrentApp().NewWindow(fmt.Sprintf("Research Log: %s", personName))

	var rebuildContent func()
	rebuildContent = func() {
		// Reload logs
		logs, err := s.GetResearchLogsForPerson(personID)
		if err != nil {
			dialog.ShowError(fmt.Errorf("Failed to load research logs: %w", err), parentWindow)
			return
		}

		content := container.NewVBox()

		// Header
		header := widget.NewLabelWithStyle(
			fmt.Sprintf("Research Log for %s (%d entries)", personName, len(logs)),
			fyne.TextAlignCenter, fyne.TextStyle{Bold: true})
		content.Add(header)
		content.Add(widget.NewSeparator())

		// Add New button
		addBtn := widget.NewButton("+ Add Research Log Entry", func() {
			showAddResearchLogDialog(w, s, &personID, rebuildContent)
		})
		content.Add(addBtn)
		content.Add(widget.NewSeparator())

		if len(logs) == 0 {
			content.Add(widget.NewLabel("No research log entries for this person yet."))
		} else {
			// Display logs
			for _, log := range logs {
				logCopy := log // Capture for closure
				logCard := makeResearchLogCard(logCopy, s, rebuildContent, w)
				content.Add(logCard)
				content.Add(widget.NewSeparator())
			}
		}

		scroll := container.NewVScroll(content)
		scroll.SetMinSize(fyne.NewSize(700, 800))
		w.SetContent(scroll)
	}

	rebuildContent()
	w.Resize(fyne.NewSize(800, 900))
	w.Show()
}
