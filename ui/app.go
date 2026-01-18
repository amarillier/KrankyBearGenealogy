package ui

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/widget"

	"genealogy/config"
	"genealogy/importer"
	"genealogy/store"
)

var mainWindowRef fyne.Window

// Theme and dialog callback functions from main.go
var (
	LightThemeFunc  func()
	DarkThemeFunc   func()
	SystemThemeFunc func()
	ShowAboutFunc   func()
	ShowHelpFunc    func()
	CheckUpdateFunc func()
)

// SetThemeFunctions sets theme switching callbacks from main.go
func SetThemeFunctions(light, dark, system func()) {
	LightThemeFunc = light
	DarkThemeFunc = dark
	SystemThemeFunc = system
}

// SetDialogFunctions sets dialog callbacks from main.go
func SetDialogFunctions(about, help, update func()) {
	ShowAboutFunc = about
	ShowHelpFunc = help
	CheckUpdateFunc = update
}

// GetMainWindow returns the main window reference
func GetMainWindow() fyne.Window {
	return mainWindowRef
}

// RunApp starts a Fyne application with PAF-style tabbed interface.
func RunApp(a fyne.App, s *store.Store, cfgInterface interface{}, dbPath string) {
	w := a.NewWindow(fmt.Sprintf("KrankyBear Genealogy - %s", filepath.Base(dbPath)))
	w.SetMaster()     // Set as master window, closes all child windows when closed
	mainWindowRef = w // Store reference for system tray

	// Extract config
	cfg, _ := cfgInterface.(*config.Config)

	// Use a container so we can update the store reference when switching databases
	// (Go closures capture variables, not values, so we need indirection)
	storeRef := []*store.Store{s}
	getStore := func() *store.Store { return storeRef[0] }

	// Load all people for the index list
	people, err := getStore().GetPeople()
	if err != nil {
		w.SetContent(widget.NewLabel(fmt.Sprintf("Error loading people: %v", err)))
		w.ShowAndRun()
		return
	}

	// Sort people alphabetically, with incomplete records at the end
	sort.Slice(people, func(i, j int) bool {
		// Helper function to check if a name is incomplete
		isIncomplete := func(p store.Person) bool {
			surname := strings.TrimSpace(p.Surname)
			given := strings.TrimSpace(p.GivenName)
			return surname == "" || given == "" ||
				strings.EqualFold(surname, "unknown") ||
				strings.EqualFold(given, "unknown")
		}

		iIncomplete := isIncomplete(people[i])
		jIncomplete := isIncomplete(people[j])

		// Complete records come before incomplete ones
		if iIncomplete != jIncomplete {
			return !iIncomplete // i is complete (not incomplete)
		}

		// Within same category, sort alphabetically
		if people[i].Surname != people[j].Surname {
			return strings.ToLower(people[i].Surname) < strings.ToLower(people[j].Surname)
		}
		return strings.ToLower(people[i].GivenName) < strings.ToLower(people[j].GivenName)
	})

	// Current selected person - check preference: Focus User or Last Person
	var currentPersonID int64
	if cfg != nil {
		if cfg.OpenWithFocusUser {
			// Use Focus User if set
			if focusID := cfg.GetFocusUserForDatabase(dbPath); focusID > 0 {
				// Verify this person exists
				if _, err := getStore().GetPersonByID(focusID); err == nil {
					currentPersonID = focusID
				}
			}
		} else {
			// Use Last Person (default)
			if lastID := cfg.GetLastPersonForDatabase(dbPath); lastID > 0 {
				// Verify this person exists
				if _, err := getStore().GetPersonByID(lastID); err == nil {
					currentPersonID = lastID
				}
			}
		}
	}

	// Fallback to first person if no valid saved person
	if currentPersonID == 0 && len(people) > 0 {
		currentPersonID = people[0].ID
	}

	// Create views
	var familyView *FamilyView
	var pedigreeView *PedigreeView
	var individualView *IndividualView
	var peopleList *widget.List
	var searchEntry *widget.Entry
	var tabs *container.AppTabs

	// Edit person handler
	// Forward declare refresh function
	var refreshPeopleList func()

	editPerson := func(personID int64) {
		person, err := getStore().GetPersonByID(personID)
		if err != nil {
			dialog.ShowError(err, w)
			return
		}
		showPersonDialog(w, getStore(), person, func() {
			// Refresh the people list data
			if refreshPeopleList != nil {
				refreshPeopleList()
			}

			// Refresh all views and maintain selection on edited person
			if updatedPerson, err := getStore().GetPersonByID(personID); err == nil {
				// Update current person ID to maintain focus
				currentPersonID = personID

				familyView.SetPerson(updatedPerson)
				pedigreeView.SetPerson(updatedPerson)
				individualView.Refresh()
				individualView.SetPerson(updatedPerson)
			}
		})
	}

	// Function to navigate to a person
	// Declare updateStatus placeholder (will be set after status bar is created)
	var updateStatus func()

	navigateToPerson := func(personID int64) {
		currentPersonID = personID
		person, err := getStore().GetPersonByID(personID)
		if err != nil {
			dialog.ShowError(err, w)
			return
		}

		// Update current view (check tabs is initialized)
		if tabs != nil && tabs.Selected() != nil {
			switch tabs.Selected().Text {
			case "Family":
				familyView.SetPerson(person)
			case "Pedigree":
				pedigreeView.SetPerson(person)
			}
		} else {
			// Initialize all views if tabs not ready yet
			familyView.SetPerson(person)
			pedigreeView.SetPerson(person)
		}

		// Update selection in people list
		for i, p := range people {
			if p.ID == personID {
				peopleList.Select(i)
				break
			}
		}

		// Update status bar
		if updateStatus != nil {
			updateStatus()
		}
	}

	// Switch to family view with person
	switchToFamily := func(personID int64) {
		currentPersonID = personID
		person, err := getStore().GetPersonByID(personID)
		if err != nil {
			dialog.ShowError(err, w)
			return
		}
		familyView.SetPerson(person)
		tabs.SelectIndex(0) // Switch to Family tab

		// Update selection in people list
		for i, p := range people {
			if p.ID == personID {
				peopleList.Select(i)
				break
			}
		}
	}

	// Switch to pedigree view with person
	switchToPedigree := func(personID int64) {
		currentPersonID = personID
		person, err := getStore().GetPersonByID(personID)
		if err != nil {
			dialog.ShowError(err, w)
			return
		}
		pedigreeView.SetPerson(person)
		tabs.SelectIndex(1) // Switch to Pedigree tab

		// Update selection in people list
		for i, p := range people {
			if p.ID == personID {
				peopleList.Select(i)
				break
			}
		}
	}

	// Create views
	familyView = NewFamilyView(getStore(), w, navigateToPerson)
	pedigreeView = NewPedigreeView(getStore(), w, navigateToPerson, editPerson)
	individualView = NewIndividualView(getStore(), w, navigateToPerson, editPerson)
	individualView.SetSwitchHandlers(switchToFamily, switchToPedigree)

	// Filter function for search - now updates Individual view too
	filteredPeople := make([]store.Person, len(people))
	copy(filteredPeople, people)

	var onPersonSelectedFunc func(int)

	updateFilteredList := func(searchText string) {
		filteredPeople = filteredPeople[:0]
		searchText = strings.ToLower(strings.TrimSpace(searchText))
		for _, p := range people {
			if searchText == "" ||
				strings.Contains(strings.ToLower(p.GivenName), searchText) ||
				strings.Contains(strings.ToLower(p.Surname), searchText) {
				filteredPeople = append(filteredPeople, p)
			}
		}

		// Sort filtered list alphabetically, with incomplete records at the end
		sort.Slice(filteredPeople, func(i, j int) bool {
			// Helper function to check if a name is incomplete
			isIncomplete := func(p store.Person) bool {
				surname := strings.TrimSpace(p.Surname)
				given := strings.TrimSpace(p.GivenName)
				return surname == "" || given == "" ||
					strings.EqualFold(surname, "unknown") ||
					strings.EqualFold(given, "unknown")
			}

			iIncomplete := isIncomplete(filteredPeople[i])
			jIncomplete := isIncomplete(filteredPeople[j])

			// Complete records come before incomplete ones
			if iIncomplete != jIncomplete {
				return !iIncomplete
			}

			// Within same category, sort alphabetically
			if filteredPeople[i].Surname != filteredPeople[j].Surname {
				return strings.ToLower(filteredPeople[i].Surname) < strings.ToLower(filteredPeople[j].Surname)
			}
			return strings.ToLower(filteredPeople[i].GivenName) < strings.ToLower(filteredPeople[j].GivenName)
		})

		// Update Individual view with filtered list
		individualView.SetFilteredPeople(filteredPeople)

		// Try to keep the current person selected if they're in the filtered list
		selectedIndex := -1
		if currentPersonID > 0 {
			for i, p := range filteredPeople {
				if p.ID == currentPersonID {
					selectedIndex = i
					break
				}
			}
		}

		// Unselect to reset widget state
		peopleList.UnselectAll()
		peopleList.Refresh()

		// Select the current person if they're in the list, otherwise select first
		if selectedIndex >= 0 {
			// Current person is in filtered list, keep them selected
			peopleList.Select(selectedIndex)
		} else if len(filteredPeople) > 0 && onPersonSelectedFunc != nil {
			// Current person not in list (or no current person), select first
			peopleList.Select(0)
			// Force trigger the callback in case Select doesn't fire it
			onPersonSelectedFunc(0)
		}
	}

	// Search entry
	searchEntry = widget.NewEntry()
	searchEntry.SetPlaceHolder("Search by name...")
	searchEntry.OnChanged = updateFilteredList

	// People list (index)
	peopleList = widget.NewList(
		func() int { return len(filteredPeople) },
		func() fyne.CanvasObject { return widget.NewLabel("genealogy") },
		func(i int, o fyne.CanvasObject) {
			if i >= len(filteredPeople) {
				return
			}
			p := filteredPeople[i]
			label := o.(*widget.Label)
			text := fmt.Sprintf("%s %s", p.GivenName, p.Surname)
			if p.BirthDate != "" {
				text += fmt.Sprintf(" (%s)", p.BirthDate)
			}
			label.SetText(text)
		},
	)

	onPersonSelectedFunc = func(i int) {
		if i >= 0 && i < len(filteredPeople) {
			personID := filteredPeople[i].ID
			person, err := getStore().GetPersonByID(personID)
			if err != nil {
				dialog.ShowError(err, w)
				return
			}

			// Update current person ID
			currentPersonID = personID

			// Save to config for next session
			if cfg != nil {
				if cfg.LastPersonID == nil {
					cfg.LastPersonID = make(map[string]int64)
				}
				cfg.LastPersonID[dbPath] = personID
				go cfg.Save() // Save in background
			}

			// Update all views
			familyView.SetPerson(person)
			pedigreeView.SetPerson(person)
			individualView.SetPerson(person)
		}
	}

	peopleList.OnSelected = onPersonSelectedFunc

	// Initialize with saved person (or first person)
	initialIndex := 0
	if currentPersonID > 0 {
		// Find the saved person in the list
		for i, p := range people {
			if p.ID == currentPersonID {
				initialIndex = i
				break
			}
		}
	}

	// Select and load the initial person
	if len(people) > 0 {
		peopleList.Select(initialIndex)
		if person, err := getStore().GetPersonByID(people[initialIndex].ID); err == nil {
			familyView.SetPerson(person)
			pedigreeView.SetPerson(person)
			individualView.SetPerson(person)
		}
	}

	// Refresh people list (actual implementation)
	refreshPeopleList = func() {
		var err error
		people, err = getStore().GetPeople()
		if err != nil {
			dialog.ShowError(err, w)
			return
		}
		// Sort people alphabetically, with incomplete records at the end
		sort.Slice(people, func(i, j int) bool {
			// Helper function to check if a name is incomplete
			isIncomplete := func(p store.Person) bool {
				surname := strings.TrimSpace(p.Surname)
				given := strings.TrimSpace(p.GivenName)
				return surname == "" || given == "" ||
					strings.EqualFold(surname, "unknown") ||
					strings.EqualFold(given, "unknown")
			}

			iIncomplete := isIncomplete(people[i])
			jIncomplete := isIncomplete(people[j])

			// Complete records come before incomplete ones
			if iIncomplete != jIncomplete {
				return !iIncomplete
			}

			// Within same category, sort alphabetically
			if people[i].Surname != people[j].Surname {
				return strings.ToLower(people[i].Surname) < strings.ToLower(people[j].Surname)
			}
			return strings.ToLower(people[i].GivenName) < strings.ToLower(people[j].GivenName)
		})
		updateFilteredList(searchEntry.Text)
	}

	// Refresh function to reload all
	refreshAll := func() {
		refreshPeopleList()
		individualView.Refresh()
		// Refresh current person view
		if currentPersonID > 0 {
			if person, err := getStore().GetPersonByID(currentPersonID); err == nil {
				familyView.SetPerson(person)
				pedigreeView.SetPerson(person)
			}
		}
	}

	// Menu buttons
	importBtn := widget.NewButton("Import GEDCOM", func() {
		fd := dialog.NewFileOpen(func(r fyne.URIReadCloser, err error) {
			if err != nil || r == nil {
				return
			}
			path := r.URI().Path()
			r.Close()
			if err := importer.Import(path, getStore()); err != nil {
				dialog.ShowError(err, w)
				return
			}
			dialog.ShowInformation("Import", "Import completed successfully", w)
			refreshAll()
		}, w)
		fd.SetFilter(storageFilter{ext: ".ged"})
		fd.Show()
	})

	importGenoProBtn := widget.NewButton("Import GenoPro", func() {
		fd := dialog.NewFileOpen(func(r fyne.URIReadCloser, err error) {
			if err != nil || r == nil {
				return
			}
			path := r.URI().Path()
			r.Close()
			if err := importer.ImportGenoPro(path, getStore()); err != nil {
				dialog.ShowError(err, w)
				return
			}
			dialog.ShowInformation("Import", "GenoPro import completed successfully", w)
			refreshAll()
		}, w)
		fd.SetFilter(storageFilter{ext: ".gno"})
		fd.Show()
	})

	importGrampsBtn := widget.NewButton("Import Gramps", func() {
		fd := dialog.NewFileOpen(func(r fyne.URIReadCloser, err error) {
			if err != nil || r == nil {
				return
			}
			path := r.URI().Path()
			r.Close()

			// Show progress dialog
			dialog.ShowInformation("Importing", "Importing from Gramps database...\nThis may take a moment.", w)

			if err := importer.ImportGramps(path, getStore()); err != nil {
				dialog.ShowError(fmt.Errorf("Gramps import failed: %w", err), w)
				return
			}
			dialog.ShowInformation("Import", "Gramps import completed successfully", w)
			refreshAll()
		}, w)
		fd.SetFilter(storageFilter{ext: ".db"})
		fd.Show()
	})

	exportBtn := widget.NewButton("Export GEDCOM", func() {
		showExportOptionsDialog(w, cfg, dbPath, getStore, &currentPersonID)
	})

	addPersonBtn := widget.NewButton("Add Person", func() {
		showPersonDialog(w, getStore(), nil, func() {
			refreshAll()
		})
	})

	deletePersonBtn := widget.NewButton("Delete Person", func() {
		if currentPersonID <= 0 {
			dialog.ShowInformation("Delete", "Select a person first", w)
			return
		}
		person, err := getStore().GetPersonByID(currentPersonID)
		if err != nil {
			dialog.ShowError(err, w)
			return
		}
		dialog.ShowConfirm("Delete Person",
			fmt.Sprintf("Are you sure you want to delete %s %s? This will remove all relationships.",
				person.GivenName, person.Surname),
			func(confirmed bool) {
				if !confirmed {
					return
				}
				if err := getStore().DeletePerson(currentPersonID); err != nil {
					dialog.ShowError(err, w)
					return
				}
				// Navigate to first person or clear view
				refreshAll()
				if len(people) > 0 {
					navigateToPerson(people[0].ID)
				} else {
					currentPersonID = 0
					familyView.SetPerson(nil)
				}
			}, w)
	})

	focusPersonBtn := widget.NewButton("Focus Person", func() {
		// Navigate to the focus user if configured, otherwise last person
		focusID := cfg.GetFocusUserForDatabase(dbPath)
		if focusID == 0 {
			// Fallback to last person if no focus user configured
			focusID = cfg.LastPersonID[dbPath]
		}
		if focusID == 0 && len(people) > 0 {
			// Fallback to first person if nothing configured
			focusID = people[0].ID
		}
		if focusID > 0 {
			navigateToPerson(focusID)
		} else {
			dialog.ShowInformation("No Focus Person",
				"No focus person is configured.\n\nPlease set a focus person in Settings → Preferences.", w)
		}
	})

	addMediaBtn := widget.NewButton("📷 Add Media", func() {
		showGlobalMediaDialog(w, getStore())
	})

	mediaLibraryBtn := widget.NewButton("🖼️ Media Library", func() {
		showMediaLibrary(w, getStore())
	})

	// Function to reload the app with a different database
	reloadWithDatabase := func(newDBPath string) {
		// Save the new database path to config
		cfg.LastDatabase = newDBPath
		cfg.AddRecentDatabase(newDBPath)
		if err := cfg.Save(); err != nil {
			dialog.ShowError(fmt.Errorf("Failed to save config: %w", err), w)
			return
		}

		// Keep reference to old store to close it AFTER everything is set up
		oldStore := getStore()

		// Open new database
		newStore, err := store.Open(newDBPath)
		if err != nil {
			dialog.ShowError(fmt.Errorf("Failed to open database: %w", err), w)
			return
		}

		// Initialize schema for new database
		if err := newStore.InitSchema(); err != nil {
			dialog.ShowError(fmt.Errorf("Failed to initialize schema: %w", err), w)
			newStore.Close()
			return
		}

		if err := newStore.MigrateSchema(); err != nil {
			dialog.ShowError(fmt.Errorf("Failed to migrate schema: %w", err), w)
			newStore.Close()
			return
		}

		// Update store reference (this updates all closures!)
		storeRef[0] = newStore
		dbPath = newDBPath

		// Update window title
		w.SetTitle(fmt.Sprintf("KrankyBear Genealogy - %s", filepath.Base(dbPath)))

		// Recreate views with new store
		familyView = NewFamilyView(getStore(), w, navigateToPerson)
		pedigreeView = NewPedigreeView(getStore(), w, navigateToPerson, editPerson)
		individualView = NewIndividualView(getStore(), w, navigateToPerson, editPerson)
		individualView.SetSwitchHandlers(switchToFamily, switchToPedigree)

		// Update tabs with new views
		tabs.Items[0].Content = familyView
		tabs.Items[1].Content = pedigreeView
		tabs.Items[2].Content = individualView
		tabs.Refresh()

		// Reload all data
		refreshPeopleList()

		// Reset to first person or clear view
		if len(people) > 0 {
			navigateToPerson(people[0].ID)
		} else {
			currentPersonID = 0
			familyView.SetPerson(nil)
			pedigreeView.SetPerson(nil)
			individualView.SetPerson(nil)
		}

		// NOW close the old database after everything is set up
		if oldStore != nil {
			oldStore.Close()
		}

		dialog.ShowInformation("Database Loaded",
			fmt.Sprintf("Successfully switched to:\n%s", filepath.Base(newDBPath)), w)
	}

	openDatabaseBtn := widget.NewButton("Open Database...", func() {
		fd := dialog.NewFileOpen(func(r fyne.URIReadCloser, err error) {
			if err != nil || r == nil {
				return
			}
			dbPath := r.URI().Path()
			r.Close()
			reloadWithDatabase(dbPath)
		}, w)
		fd.SetFilter(storageFilter{ext: ".db"})
		fd.Show()
	})

	newDatabaseBtn := widget.NewButton("New Database...", func() {
		fd := dialog.NewFileSave(func(uc fyne.URIWriteCloser, err error) {
			if err != nil || uc == nil {
				return
			}
			dbPath := uc.URI().Path()
			uc.Close()

			// Ensure .db extension
			if !strings.HasSuffix(strings.ToLower(dbPath), ".db") {
				dbPath = dbPath + ".db"
			}

			// Create and initialize the new database
			newStore, err := store.Open(dbPath)
			if err != nil {
				dialog.ShowError(fmt.Errorf("Failed to create database: %w", err), w)
				return
			}

			if err := newStore.InitSchema(); err != nil {
				newStore.Close()
				dialog.ShowError(fmt.Errorf("Failed to initialize database schema: %w", err), w)
				return
			}

			if err := newStore.MigrateSchema(); err != nil {
				newStore.Close()
				dialog.ShowError(fmt.Errorf("Failed to migrate database schema: %w", err), w)
				return
			}

			newStore.Close()

			// Reload with the new database
			reloadWithDatabase(dbPath)
		}, w)
		fd.SetFileName("genealogy.db")
		fd.Show()
	})

	backupBtn := widget.NewButton("Backup Database", func() {
		showBackupDialog(w, dbPath)
	})

	restoreBtn := widget.NewButton("Restore Database", func() {
		showRestoreDialog(w, dbPath, reloadWithDatabase)
	})

	dataQualityBtn := widget.NewButton("Data Quality Report", func() {
		showDataQualityReport(w, getStore(), navigateToPerson)
	})

	livingStatusBtn := widget.NewButton("Living Status Report", func() {
		showLivingStatusReport(w, getStore(), navigateToPerson)
	})

	settingsBtn := widget.NewButton("Settings", func() {
		showSettingsDialog(w, cfg, dbPath, getStore(), func() {
			// Callback when settings change - no need to refresh views
			_ = cfg.Save()
		})
	})

	// Top toolbar - simplified, most actions moved to menus
	toolbar := container.NewHBox(focusPersonBtn, addPersonBtn, deletePersonBtn, addMediaBtn, mediaLibraryBtn)

	// Status bar for statistics and relationship info
	statusLabel := widget.NewLabel("Ready")
	statusLabel.TextStyle = fyne.TextStyle{Monospace: true}

	// Function to update status bar
	updateStatus = func() {
		stats := getStats(getStore())
		relationshipText := ""

		// Calculate relationship if we have both a focus user and a current person
		focusID := cfg.GetFocusUserForDatabase(dbPath)
		if focusID > 0 && currentPersonID > 0 && focusID != currentPersonID {
			currentPerson, err := getStore().GetPersonByID(currentPersonID)
			if err == nil {
				relationship := calculateRelationship(getStore(), focusID, currentPersonID)
				if relationship != "" {
					relationshipText = fmt.Sprintf(" | %s %s is your %s",
						currentPerson.GivenName, currentPerson.Surname, relationship)
				}
			}
		}

		statusLabel.SetText(fmt.Sprintf("%s%s", stats, relationshipText))
	}

	// Initial status update
	updateStatus()

	// Create tabs for different views
	tabs = container.NewAppTabs(
		container.NewTabItem("Family", familyView),
		container.NewTabItem("Pedigree", pedigreeView),
		container.NewTabItem("Individual", individualView),
	)

	// Add tab switch handler to refresh current person when switching tabs
	tabs.OnSelected = func(item *container.TabItem) {
		if currentPersonID > 0 {
			person, err := getStore().GetPersonByID(currentPersonID)
			if err != nil {
				return
			}
			switch item.Text {
			case "Family":
				familyView.SetPerson(person)
			case "Pedigree":
				pedigreeView.SetPerson(person)
			case "Individual":
				individualView.SetPerson(person)
			}
		}
	}

	// Left panel: search + people list
	leftPanel := container.NewBorder(searchEntry, nil, nil, nil, peopleList)

	// Main split: left panel (people index) and tabbed views
	split := container.NewHSplit(leftPanel, tabs)
	split.SetOffset(0.2) // 20% for the index, 80% for views

	// Main layout with status bar at bottom
	content := container.NewBorder(toolbar, statusLabel, nil, nil, split)

	w.SetContent(content)
	w.Resize(fyne.NewSize(1200, 800))

	// Setup system tray and menus (if supported)
	setupMenus(a, w, newDatabaseBtn, openDatabaseBtn, backupBtn, restoreBtn, importBtn, importGenoProBtn, importGrampsBtn, exportBtn, dataQualityBtn, livingStatusBtn, settingsBtn, getStore, navigateToPerson)

	w.ShowAndRun()
}

// setupMenus creates the system tray and window menus
func setupMenus(a fyne.App, w fyne.Window, newDBBtn, openDBBtn, backupBtn, restoreBtn, importBtn, importGNOBtn, importGrampsBtn, exportBtn, dataQBtn, livingStatusBtn, settingsBtn *widget.Button, getStore func() *store.Store, navigateToPerson func(int64)) {
	desk, ok := a.(desktop.App)
	if !ok {
		return // System tray not supported on this platform
	}

	// Window actions
	show := fyne.NewMenuItem("Show", func() {
		w.Show()
		w.RequestFocus()
	})
	hide := fyne.NewMenuItem("Hide", func() {
		w.Hide()
	})

	// File menu items (using button callbacks)
	newDB := fyne.NewMenuItem("New Database...", func() {
		newDBBtn.OnTapped()
	})
	openDB := fyne.NewMenuItem("Open Database...", func() {
		openDBBtn.OnTapped()
	})
	backup := fyne.NewMenuItem("Backup Database...", func() {
		backupBtn.OnTapped()
	})
	restore := fyne.NewMenuItem("Restore Database...", func() {
		restoreBtn.OnTapped()
	})
	importGED := fyne.NewMenuItem("Import GEDCOM...", func() {
		importBtn.OnTapped()
	})
	importGNO := fyne.NewMenuItem("Import GenoPro...", func() {
		importGNOBtn.OnTapped()
	})
	importGramps := fyne.NewMenuItem("Import Gramps...", func() {
		importGrampsBtn.OnTapped()
	})
	exportGED := fyne.NewMenuItem("Export GEDCOM...", func() {
		exportBtn.OnTapped()
	})
	quit := fyne.NewMenuItem("Quit", func() {
		a.Quit()
	})

	// Reports menu items
	dataQuality := fyne.NewMenuItem("Data Quality Report", func() {
		showDataQualityReport(w, getStore(), navigateToPerson)
	})

	livingStatus := fyne.NewMenuItem("Living Status Report", func() {
		showLivingStatusReport(w, getStore(), navigateToPerson)
	})

	// Media menu items
	mediaLibraryMenuItem := fyne.NewMenuItem("Media Library", func() {
		showMediaLibrary(w, getStore())
	})

	addMediaMenuItem := fyne.NewMenuItem("Add Media", func() {
		showGlobalMediaDialog(w, getStore())
	})

	// Settings menu items
	settingsDialog := fyne.NewMenuItem("Preferences...", func() {
		settingsBtn.OnTapped()
	})
	settingsLight := fyne.NewMenuItem("Light Theme", func() {
		if LightThemeFunc != nil {
			LightThemeFunc()
		}
	})
	settingsDark := fyne.NewMenuItem("Dark Theme", func() {
		if DarkThemeFunc != nil {
			DarkThemeFunc()
		}
	})
	settingsSystem := fyne.NewMenuItem("System Theme", func() {
		if SystemThemeFunc != nil {
			SystemThemeFunc()
		}
	})

	// Help menu items
	about := fyne.NewMenuItem("About", func() {
		if ShowAboutFunc != nil {
			ShowAboutFunc()
		}
	})
	updtchk := fyne.NewMenuItem("Check for Update", func() {
		if CheckUpdateFunc != nil {
			CheckUpdateFunc()
		}
	})
	help := fyne.NewMenuItem("Help", func() {
		if ShowHelpFunc != nil {
			ShowHelpFunc()
		}
	})

	// System tray menu (flat structure)
	menu := fyne.NewMenu("KrankyBear Genealogy",
		show, hide,
		fyne.NewMenuItemSeparator(),
		newDB, openDB, backup, restore,
		fyne.NewMenuItemSeparator(),
		importGED, importGNO, importGramps, exportGED,
		fyne.NewMenuItemSeparator(),
		dataQuality, livingStatus, settingsDialog,
		fyne.NewMenuItemSeparator(),
		about, updtchk, help,
		fyne.NewMenuItemSeparator(),
		settingsLight, settingsDark, settingsSystem,
		fyne.NewMenuItemSeparator(),
		quit)
	desk.SetSystemTrayMenu(menu)
	// Icon is set from main package, we just set up the menu here

	// Setup main menu bar (for window menu bar with submenus)
	fileMenu := fyne.NewMenu("File", newDB, openDB, backup, restore, fyne.NewMenuItemSeparator(),
		importGED, importGNO, importGramps, exportGED, fyne.NewMenuItemSeparator(), quit)
	mediaMenu := fyne.NewMenu("Media", mediaLibraryMenuItem, addMediaMenuItem)
	reportsMenu := fyne.NewMenu("Reports", dataQuality, livingStatus)
	settingsMenu := fyne.NewMenu("Settings", settingsDialog, fyne.NewMenuItemSeparator(),
		settingsLight, settingsDark, settingsSystem)
	helpMenu := fyne.NewMenu("Help", about, updtchk, help)
	cmenu := fyne.NewMainMenu(fileMenu, mediaMenu, reportsMenu, settingsMenu, helpMenu)
	w.SetMainMenu(cmenu)
}

// storageFilter implements fyne.FileFilter for file dialogs.
type storageFilter struct{ ext string }

func (f storageFilter) Matches(uri fyne.URI) bool {
	return strings.HasSuffix(strings.ToLower(uri.Path()), strings.ToLower(f.ext))
}

func (f storageFilter) Name() string { return f.ext + " files" }

// showPersonDialog shows a dialog to create or edit a person.
func showPersonDialog(w fyne.Window, s *store.Store, p *store.Person, onSave func()) {
	var isEdit bool
	var person store.Person
	if p != nil {
		person = *p
		isEdit = true
	}

	// Form fields
	givenEntry := widget.NewEntry()
	givenEntry.SetPlaceHolder("Given name(s)")
	givenEntry.SetText(person.GivenName)

	surnameEntry := widget.NewEntry()
	surnameEntry.SetPlaceHolder("Surname")
	surnameEntry.SetText(person.Surname)

	genderSelect := widget.NewSelect([]string{"", "M", "F"}, func(string) {})
	if person.Gender != "" {
		genderSelect.SetSelected(person.Gender)
	}

	birthDateEntry := widget.NewEntry()
	birthDateEntry.SetPlaceHolder("DD MMM YYYY or YYYY-MM-DD")
	birthDateEntry.SetText(person.BirthDate)

	birthPlaceEntry := widget.NewEntry()
	birthPlaceEntry.SetPlaceHolder("City, State/Province, Country")
	birthPlaceEntry.SetText(person.BirthPlace)

	deathDateEntry := widget.NewEntry()
	deathDateEntry.SetPlaceHolder("DD MMM YYYY or YYYY-MM-DD")
	deathDateEntry.SetText(person.DeathDate)

	deathPlaceEntry := widget.NewEntry()
	deathPlaceEntry.SetPlaceHolder("City, State/Province, Country")
	deathPlaceEntry.SetText(person.DeathPlace)

	isLivingCheck := widget.NewCheck("Still living", func(bool) {})
	// Default to living for new persons, or use existing value for edits
	if isEdit {
		isLivingCheck.SetChecked(person.IsLiving)
	} else {
		isLivingCheck.SetChecked(true) // Default to living for new persons
	}

	// Auto-update living checkbox based on death date
	deathDateEntry.OnChanged = func(text string) {
		if strings.TrimSpace(text) != "" {
			// Death date entered - uncheck living
			isLivingCheck.SetChecked(false)
		} else {
			// Death date cleared - check living (if this is a new person or was living before)
			if !isEdit || person.IsLiving {
				isLivingCheck.SetChecked(true)
			}
		}
	}

	// Contact information fields (for living relatives)
	addressEntry := widget.NewEntry()
	addressEntry.SetPlaceHolder("Street address")
	addressEntry.SetText(person.Address)

	cityEntry := widget.NewEntry()
	cityEntry.SetPlaceHolder("City")
	cityEntry.SetText(person.City)

	stateEntry := widget.NewEntry()
	stateEntry.SetPlaceHolder("State/Province")
	stateEntry.SetText(person.State)

	postalCodeEntry := widget.NewEntry()
	postalCodeEntry.SetPlaceHolder("ZIP/Postal code")
	postalCodeEntry.SetText(person.PostalCode)

	countryEntry := widget.NewEntry()
	countryEntry.SetPlaceHolder("Country")
	countryEntry.SetText(person.Country)

	emailEntry := widget.NewEntry()
	emailEntry.SetPlaceHolder("name@example.com")
	emailEntry.SetText(person.Email)

	phoneEntry := widget.NewEntry()
	phoneEntry.SetPlaceHolder("+1 555-123-4567")
	phoneEntry.SetText(person.Phone)

	uidEntry := widget.NewEntry()
	uidEntry.SetPlaceHolder("External/GEDCOM UID (optional)")
	uidEntry.SetText(person.UID)

	notesEntry := widget.NewMultiLineEntry()
	notesEntry.SetPlaceHolder("Notes, sources, or additional information")
	notesEntry.SetText(person.Notes)
	notesEntry.SetMinRowsVisible(4)

	// Media button (only for existing persons)
	var mediaSection *fyne.Container
	if isEdit {
		mediaBtn := widget.NewButton("📷 Manage Photos & Media", func() {
			personName := fmt.Sprintf("%s %s", person.GivenName, person.Surname)
			showMediaManager(w, s, person.ID, personName)
		})
		mediaSection = container.NewVBox(
			widget.NewSeparator(),
			widget.NewLabel("Media:"),
			mediaBtn,
		)
	}

	formItems := []fyne.CanvasObject{
		widget.NewLabel("Given Name(s):"), givenEntry,
		widget.NewLabel("Surname:"), surnameEntry,
		widget.NewLabel("Gender:"), genderSelect,
		widget.NewSeparator(),
		widget.NewLabel("Birth Date:"), birthDateEntry,
		widget.NewLabel("Birth Place:"), birthPlaceEntry,
		widget.NewSeparator(),
		widget.NewLabel("Death Date:"), deathDateEntry,
		widget.NewLabel("Death Place:"), deathPlaceEntry,
		isLivingCheck,
		widget.NewSeparator(),
		widget.NewLabel("Contact Information (for living relatives):"),
		widget.NewLabel("Address:"), addressEntry,
		widget.NewLabel("City:"), cityEntry,
		widget.NewLabel("State/Province:"), stateEntry,
		widget.NewLabel("Postal Code:"), postalCodeEntry,
		widget.NewLabel("Country:"), countryEntry,
		widget.NewLabel("Email:"), emailEntry,
		widget.NewLabel("Phone:"), phoneEntry,
		widget.NewSeparator(),
		widget.NewLabel("UID:"), uidEntry,
		widget.NewLabel("Notes:"), notesEntry,
	}

	// Add media section if editing
	if mediaSection != nil {
		formItems = append(formItems, mediaSection.Objects...)
	}

	form := container.NewVBox(formItems...)

	scrollable := container.NewScroll(form)
	scrollable.SetMinSize(fyne.NewSize(500, 600))

	title := "Add Person"
	if isEdit {
		title = "Edit Person"
	}

	dialog.ShowCustomConfirm(title, "Save", "Cancel", scrollable, func(ok bool) {
		if !ok {
			return
		}

		// Validate required fields
		if strings.TrimSpace(givenEntry.Text) == "" && strings.TrimSpace(surnameEntry.Text) == "" {
			dialog.ShowInformation("Validation", "Please enter at least a given name or surname", w)
			return
		}

		person.GivenName = strings.TrimSpace(givenEntry.Text)
		person.Surname = strings.TrimSpace(surnameEntry.Text)
		person.Gender = genderSelect.Selected
		person.BirthDate = strings.TrimSpace(birthDateEntry.Text)
		person.BirthPlace = strings.TrimSpace(birthPlaceEntry.Text)
		person.DeathDate = strings.TrimSpace(deathDateEntry.Text)
		person.DeathPlace = strings.TrimSpace(deathPlaceEntry.Text)
		person.IsLiving = isLivingCheck.Checked
		person.Address = strings.TrimSpace(addressEntry.Text)
		person.City = strings.TrimSpace(cityEntry.Text)
		person.State = strings.TrimSpace(stateEntry.Text)
		person.PostalCode = strings.TrimSpace(postalCodeEntry.Text)
		person.Country = strings.TrimSpace(countryEntry.Text)
		person.Email = strings.TrimSpace(emailEntry.Text)
		person.Phone = strings.TrimSpace(phoneEntry.Text)
		person.UID = strings.TrimSpace(uidEntry.Text)
		person.Notes = strings.TrimSpace(notesEntry.Text)

		var err error
		if isEdit {
			err = s.UpdatePerson(&person)
		} else {
			err = s.CreatePerson(&person)
		}

		if err != nil {
			dialog.ShowError(err, w)
			return
		}

		if onSave != nil {
			onSave()
		}
	}, w)
}

// showDataQualityReport displays a report of records with missing or incomplete data.
func showDataQualityReport(w fyne.Window, s *store.Store, navigateFunc func(int64)) {
	people, err := s.GetPeople()
	if err != nil {
		dialog.ShowError(err, w)
		return
	}

	type issueRecord struct {
		person store.Person
		issues []string
	}

	var problemRecords []issueRecord

	// Check for various data quality issues
	for _, p := range people {
		surname := strings.TrimSpace(p.Surname)
		given := strings.TrimSpace(p.GivenName)

		personIssues := []string{}

		// Missing or "Unknown" surname
		if surname == "" {
			personIssues = append(personIssues, "missing surname")
		} else if strings.EqualFold(surname, "unknown") {
			personIssues = append(personIssues, "surname is 'Unknown'")
		}

		// Missing or "Unknown" given name
		if given == "" {
			personIssues = append(personIssues, "missing given name")
		} else if strings.EqualFold(given, "unknown") {
			personIssues = append(personIssues, "given name is 'Unknown'")
		}

		// Missing gender
		if strings.TrimSpace(p.Gender) == "" {
			personIssues = append(personIssues, "missing gender")
		}

		// Missing birth date
		if strings.TrimSpace(p.BirthDate) == "" {
			personIssues = append(personIssues, "missing birth date")
		}

		if len(personIssues) > 0 {
			problemRecords = append(problemRecords, issueRecord{
				person: p,
				issues: personIssues,
			})
		}
	}

	// Build interactive report
	if len(problemRecords) == 0 {
		dialog.ShowInformation("Data Quality Report",
			fmt.Sprintf("✓ All %d records have complete basic information!\n\nNo data quality issues found.", len(people)), w)
		return
	}

	// Create a list of clickable issue items
	content := container.NewVBox()

	header := widget.NewLabelWithStyle(
		fmt.Sprintf("Found %d records with issues out of %d total\n\nClick any record to view/edit/delete it:",
			len(problemRecords), len(people)),
		fyne.TextAlignLeading,
		fyne.TextStyle{Bold: true},
	)
	content.Add(header)
	content.Add(widget.NewSeparator())

	var dialogRef dialog.Dialog

	for _, record := range problemRecords {
		p := record.person // Capture for closure

		displayName := fmt.Sprintf("%s %s", p.GivenName, p.Surname)
		if strings.TrimSpace(displayName) == "" {
			displayName = "(no name)"
		}

		issueText := fmt.Sprintf("%s (ID: %d)", displayName, p.ID)
		issueSubtext := fmt.Sprintf("Issues: %s", strings.Join(record.issues, ", "))

		btn := widget.NewButton(issueText, func() {
			if dialogRef != nil {
				dialogRef.Hide()
			}
			// Navigate to this person
			navigateFunc(p.ID)
		})

		issueLabel := widget.NewLabel(issueSubtext)
		issueLabel.TextStyle = fyne.TextStyle{Italic: true}

		itemBox := container.NewVBox(btn, issueLabel, widget.NewSeparator())
		content.Add(itemBox)
	}

	scroll := container.NewScroll(content)
	scroll.SetMinSize(fyne.NewSize(600, 400))

	dialogRef = dialog.NewCustom("Data Quality Report", "Close", scroll, w)
	dialogRef.Show()
}

// showLivingStatusReport displays a report analyzing ages and living status
func showLivingStatusReport(w fyne.Window, s *store.Store, navigateFunc func(int64)) {
	people, err := s.GetPeople()
	if err != nil {
		dialog.ShowError(err, w)
		return
	}

	type statusIssue struct {
		person store.Person
		issue  string
		age    int
	}

	var issues []statusIssue
	currentYear := time.Now().Year()

	// Check for various living status issues
	for _, p := range people {
		birthYear := parseBirthYear(p.BirthDate)
		if birthYear == 0 {
			continue // Skip if no birth date
		}

		age := currentYear - birthYear
		hasDeathDate := strings.TrimSpace(p.DeathDate) != ""

		// Check 1: Marked as living but very old (likely deceased)
		if p.IsLiving && !hasDeathDate && age > 110 {
			issues = append(issues, statusIssue{
				person: p,
				issue:  fmt.Sprintf("Marked as living but would be %d years old", age),
				age:    age,
			})
		}

		// Check 2: Probably deceased (over 100) but marked as living
		if p.IsLiving && !hasDeathDate && age > 100 && age <= 110 {
			issues = append(issues, statusIssue{
				person: p,
				issue:  fmt.Sprintf("Marked as living but would be %d years old (possible)", age),
				age:    age,
			})
		}

		// Check 3: NOT marked as living, no death date, but relatively young (might still be alive)
		if !p.IsLiving && !hasDeathDate && age < 100 {
			issues = append(issues, statusIssue{
				person: p,
				issue:  fmt.Sprintf("Not marked as living but no death date (age %d)", age),
				age:    age,
			})
		}

		// Check 4: Has death date but still marked as living
		if p.IsLiving && hasDeathDate {
			issues = append(issues, statusIssue{
				person: p,
				issue:  fmt.Sprintf("Has death date (%s) but marked as still living", p.DeathDate),
				age:    age,
			})
		}
	}

	// Build interactive report
	if len(issues) == 0 {
		dialog.ShowInformation("Living Status Report",
			fmt.Sprintf("✓ No living status inconsistencies found in %d records!\n\nAll living/deceased statuses appear reasonable.", len(people)), w)
		return
	}

	// Create a list of clickable issue items
	content := container.NewVBox()

	header := widget.NewLabelWithStyle(
		fmt.Sprintf("Found %d potential living status issues out of %d total\n\nClick any record to view/edit:",
			len(issues), len(people)),
		fyne.TextAlignLeading,
		fyne.TextStyle{Bold: true},
	)
	content.Add(header)
	content.Add(widget.NewSeparator())

	var dialogRef dialog.Dialog

	for _, rec := range issues {
		p := rec.person // Capture for closure

		displayName := fmt.Sprintf("%s %s", p.GivenName, p.Surname)
		if strings.TrimSpace(displayName) == "" {
			displayName = "(no name)"
		}

		issueText := fmt.Sprintf("%s (ID: %d)", displayName, p.ID)

		btn := widget.NewButton(issueText, func() {
			if dialogRef != nil {
				dialogRef.Hide()
			}
			// Navigate to this person
			navigateFunc(p.ID)
		})

		issueLabel := widget.NewLabel(rec.issue)
		issueLabel.TextStyle = fyne.TextStyle{Italic: true}

		birthInfo := fmt.Sprintf("Birth: %s", p.BirthDate)
		if p.BirthDate == "" {
			birthInfo = "Birth: (unknown)"
		}

		deathInfo := fmt.Sprintf("Death: %s", p.DeathDate)
		if p.DeathDate == "" {
			deathInfo = "Death: (no date recorded)"
		}

		livingStatus := "Not marked as living"
		if p.IsLiving {
			livingStatus = "Marked as LIVING"
		}

		detailLabel := widget.NewLabel(fmt.Sprintf("%s | %s | %s", birthInfo, deathInfo, livingStatus))
		detailLabel.TextStyle = fyne.TextStyle{Monospace: true}

		itemBox := container.NewVBox(btn, issueLabel, detailLabel, widget.NewSeparator())
		content.Add(itemBox)
	}

	scroll := container.NewScroll(content)
	scroll.SetMinSize(fyne.NewSize(700, 400))

	dialogRef = dialog.NewCustom("Living Status Report", "Close", scroll, w)
	dialogRef.Show()
}

// parseBirthYear extracts the year from various date formats
func parseBirthYear(dateStr string) int {
	dateStr = strings.TrimSpace(dateStr)
	if dateStr == "" {
		return 0
	}

	// Try YYYY format first
	if len(dateStr) == 4 {
		if year, err := strconv.Atoi(dateStr); err == nil {
			return year
		}
	}

	// Try YYYY-MM-DD format
	if len(dateStr) >= 4 && dateStr[4] == '-' {
		if year, err := strconv.Atoi(dateStr[:4]); err == nil {
			return year
		}
	}

	// Try "DD MMM YYYY" format (e.g., "15 Jan 1950")
	parts := strings.Fields(dateStr)
	if len(parts) >= 3 {
		if year, err := strconv.Atoi(parts[len(parts)-1]); err == nil {
			return year
		}
	}

	// Try last 4 characters as year (fallback)
	if len(dateStr) >= 4 {
		if year, err := strconv.Atoi(dateStr[len(dateStr)-4:]); err == nil {
			return year
		}
	}

	return 0
}

// showSettingsDialog displays application settings including Focus User preference
func showSettingsDialog(w fyne.Window, cfg *config.Config, dbPath string, s *store.Store, onSave func()) {
	// Get all people for search
	people, err := s.GetPeople()
	if err != nil {
		dialog.ShowError(fmt.Errorf("failed to load people: %w", err), w)
		return
	}

	// Get current Focus User
	focusUserID := cfg.GetFocusUserForDatabase(dbPath)
	var focusUserName string
	if focusUserID > 0 {
		if person, err := s.GetPersonByID(focusUserID); err == nil {
			focusUserName = fmt.Sprintf("%s %s (ID: %d)", person.GivenName, person.Surname, person.ID)
		}
	}

	content := container.NewVBox()

	// Section: Startup Behavior
	startupLabel := widget.NewLabelWithStyle("Startup Behavior", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	content.Add(startupLabel)

	// Radio buttons for startup mode
	var openModeRadio *widget.RadioGroup
	openModeRadio = widget.NewRadioGroup([]string{
		"Open with last focused person",
		"Open with Focus User",
	}, func(selected string) {
		cfg.OpenWithFocusUser = (selected == "Open with Focus User")
	})

	if cfg.OpenWithFocusUser {
		openModeRadio.SetSelected("Open with Focus User")
	} else {
		openModeRadio.SetSelected("Open with last focused person")
	}
	content.Add(openModeRadio)
	content.Add(widget.NewSeparator())

	// Section: Focus User
	focusLabel := widget.NewLabelWithStyle("Focus User Settings", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	content.Add(focusLabel)

	// Current Focus User display
	currentFocusLabel := widget.NewLabel("Current Focus User: " +
		func() string {
			if focusUserName != "" {
				return focusUserName
			}
			return "(not set)"
		}())
	content.Add(currentFocusLabel)

	// Search by ID or Name
	searchLabel := widget.NewLabel("Search by ID or Name:")
	searchEntry := widget.NewEntry()
	searchEntry.SetPlaceHolder("Enter ID (e.g., 123) or name (e.g., Ivan Marillier)")

	resultsLabel := widget.NewLabel("")

	// List to display search results
	var searchResults []store.Person
	resultsList := widget.NewList(
		func() int {
			return len(searchResults)
		},
		func() fyne.CanvasObject {
			return widget.NewLabel("")
		},
		func(i widget.ListItemID, o fyne.CanvasObject) {
			label := o.(*widget.Label)
			p := &searchResults[i]
			label.SetText(fmt.Sprintf("%s %s (ID: %d) - Born: %s",
				p.GivenName, p.Surname, p.ID, p.BirthDate))
		},
	)

	resultsList.OnSelected = func(id widget.ListItemID) {
		if id >= 0 && id < len(searchResults) {
			selectedPerson := &searchResults[id]
			// Set as Focus User
			cfg.SetFocusUserForDatabase(dbPath, selectedPerson.ID)
			focusUserName = fmt.Sprintf("%s %s (ID: %d)", selectedPerson.GivenName, selectedPerson.Surname, selectedPerson.ID)
			currentFocusLabel.SetText("Current Focus User: " + focusUserName)
			resultsLabel.SetText(fmt.Sprintf("✓ Focus User set to: %s", focusUserName))
		}
	}

	// Search function (shared by button and Enter key)
	performSearch := func() {
		query := strings.TrimSpace(searchEntry.Text)
		if query == "" {
			resultsLabel.SetText("Please enter a search term")
			searchResults = []store.Person{}
			resultsList.Refresh()
			return
		}

		// Check if query is numeric (ID search)
		if id, err := strconv.ParseInt(query, 10, 64); err == nil {
			// ID search
			if person, err := s.GetPersonByID(id); err == nil {
				searchResults = []store.Person{*person}
				resultsLabel.SetText(fmt.Sprintf("Found 1 match for ID %d", id))
			} else {
				searchResults = []store.Person{}
				resultsLabel.SetText(fmt.Sprintf("No person found with ID %d", id))
			}
		} else {
			// Name search
			lowerQuery := strings.ToLower(query)
			searchResults = []store.Person{}
			for _, p := range people {
				fullName := strings.ToLower(fmt.Sprintf("%s %s", p.GivenName, p.Surname))
				if strings.Contains(fullName, lowerQuery) {
					searchResults = append(searchResults, p)
				}
			}
			resultsLabel.SetText(fmt.Sprintf("Found %d matches", len(searchResults)))
		}
		resultsList.Refresh()
	}

	// Add Enter key support to search entry
	searchEntry.OnSubmitted = func(text string) {
		performSearch()
	}

	searchButton := widget.NewButton("Search", func() {
		performSearch()
	})

	searchBox := container.NewBorder(nil, nil, nil, searchButton, searchEntry)
	content.Add(searchLabel)
	content.Add(searchBox)
	content.Add(resultsLabel)

	// Results list with scroll
	resultsScroll := container.NewScroll(resultsList)
	resultsScroll.SetMinSize(fyne.NewSize(500, 200))
	content.Add(resultsScroll)

	// Clear Focus User button
	clearBtn := widget.NewButton("Clear Focus User", func() {
		cfg.SetFocusUserForDatabase(dbPath, 0)
		focusUserName = ""
		currentFocusLabel.SetText("Current Focus User: (not set)")
		resultsLabel.SetText("Focus User cleared")
	})
	content.Add(clearBtn)

	// Scroll container for entire dialog
	scroll := container.NewScroll(content)
	scroll.SetMinSize(fyne.NewSize(600, 500))

	// Dialog with Save and Cancel
	d := dialog.NewCustom("Settings", "Close", scroll, w)
	d.SetOnClosed(func() {
		onSave()
	})
	d.Show()
}

// getStats returns a formatted string with database statistics
func getStats(s *store.Store) string {
	numPeople, _ := s.CountPeople()
	relationships, _ := s.GetRelationships()
	numRelationships := len(relationships)

	return fmt.Sprintf("People: %d | Relationships: %d", numPeople, numRelationships)
}

// calculateRelationship determines the relationship between two people
func calculateRelationship(s *store.Store, fromID, toID int64) string {
	// Check for direct relationships first

	// Is spouse/partner?
	spouses, _ := s.GetSpouses(fromID)
	for _, sp := range spouses {
		if sp.Person.ID == toID {
			return "spouse"
		}
	}

	// Is parent?
	parents, _ := s.GetRelatedPeople(fromID, "parent")
	for _, p := range parents {
		if p.ID == toID {
			if p.Gender == "M" {
				return "father"
			}
			return "mother"
		}
	}

	// Is child?
	children, _ := s.GetRelatedPeople(fromID, "child")
	for _, c := range children {
		if c.ID == toID {
			if c.Gender == "M" {
				return "son"
			}
			return "daughter"
		}
	}

	// Is sibling?
	myParents, _ := s.GetRelatedPeople(fromID, "parent")
	if len(myParents) > 0 {
		for _, parent := range myParents {
			theirChildren, _ := s.GetRelatedPeople(parent.ID, "child")
			for _, sibling := range theirChildren {
				if sibling.ID == toID && sibling.ID != fromID {
					if sibling.Gender == "M" {
						return "brother"
					}
					return "sister"
				}
			}
		}
	}

	// Check for more complex relationships (grandparents, aunts/uncles, cousins)
	// Grandparent?
	for _, parent := range myParents {
		grandparents, _ := s.GetRelatedPeople(parent.ID, "parent")
		for _, gp := range grandparents {
			if gp.ID == toID {
				if gp.Gender == "M" {
					return "grandfather"
				}
				return "grandmother"
			}

			// Great-grandparent?
			greatGrandparents, _ := s.GetRelatedPeople(gp.ID, "parent")
			for _, ggp := range greatGrandparents {
				if ggp.ID == toID {
					if ggp.Gender == "M" {
						return "great-grandfather"
					}
					return "great-grandmother"
				}
			}
		}
	}

	// Grandchild?
	for _, child := range children {
		grandchildren, _ := s.GetRelatedPeople(child.ID, "child")
		for _, gc := range grandchildren {
			if gc.ID == toID {
				if gc.Gender == "M" {
					return "grandson"
				}
				return "granddaughter"
			}

			// Great-grandchild?
			greatGrandchildren, _ := s.GetRelatedPeople(gc.ID, "child")
			for _, ggc := range greatGrandchildren {
				if ggc.ID == toID {
					if ggc.Gender == "M" {
						return "great-grandson"
					}
					return "great-granddaughter"
				}
			}
		}
	}

	// Aunt/Uncle? (siblings of my parents)
	for _, parent := range myParents {
		// Get grandparents (parents of my parent)
		grandparents, _ := s.GetRelatedPeople(parent.ID, "parent")
		for _, grandparent := range grandparents {
			// Get all children of grandparent (my parent's siblings)
			parentSiblings, _ := s.GetRelatedPeople(grandparent.ID, "child")
			for _, parentSibling := range parentSiblings {
				if parentSibling.ID == toID && parentSibling.ID != parent.ID {
					if parentSibling.Gender == "M" {
						return "uncle"
					}
					return "aunt"
				}

				// Check if toID is spouse of parent's sibling (aunt/uncle by marriage)
				if parentSibling.ID != parent.ID {
					spouses, _ := s.GetSpouses(parentSibling.ID)
					for _, sp := range spouses {
						if sp.Person.ID == toID {
							if parentSibling.Gender == "M" {
								return "aunt" // wife of uncle
							}
							return "uncle" // husband of aunt
						}
					}
				}
			}

			// Great-aunt/Great-uncle? (siblings of grandparents)
			greatGrandparents, _ := s.GetRelatedPeople(grandparent.ID, "parent")
			for _, ggp := range greatGrandparents {
				ggpChildren, _ := s.GetRelatedPeople(ggp.ID, "child")
				for _, ggpChild := range ggpChildren {
					if ggpChild.ID == toID && ggpChild.ID != grandparent.ID {
						if ggpChild.Gender == "M" {
							return "great-uncle"
						}
						return "great-aunt"
					}

					// Check if toID is spouse of grandparent's sibling
					if ggpChild.ID != grandparent.ID {
						spouses, _ := s.GetSpouses(ggpChild.ID)
						for _, sp := range spouses {
							if sp.Person.ID == toID {
								if ggpChild.Gender == "M" {
									return "great-aunt" // wife of great-uncle
								}
								return "great-uncle" // husband of great-aunt
							}
						}
					}
				}
			}
		}
	}

	// Niece/Nephew?
	for _, sibling := range getSiblings(s, fromID) {
		niblings, _ := s.GetRelatedPeople(sibling.ID, "child")
		for _, n := range niblings {
			if n.ID == toID {
				if n.Gender == "M" {
					return "nephew"
				}
				return "niece"
			}

			// Great-niece/Great-nephew?
			greatNiblings, _ := s.GetRelatedPeople(n.ID, "child")
			for _, gn := range greatNiblings {
				if gn.ID == toID {
					if gn.Gender == "M" {
						return "great-nephew"
					}
					return "great-niece"
				}
			}
		}
	}

	// Cousins - more complex
	cousins := getCousinInfo(s, fromID, toID)
	if cousins != "" {
		return cousins
	}

	// In-laws
	inLaw := getInLawRelationship(s, fromID, toID)
	if inLaw != "" {
		return inLaw
	}

	return "" // No relationship found
}

// getSiblings returns all siblings of a person
func getSiblings(s *store.Store, personID int64) []store.Person {
	var siblings []store.Person
	parents, _ := s.GetRelatedPeople(personID, "parent")
	for _, parent := range parents {
		children, _ := s.GetRelatedPeople(parent.ID, "child")
		for _, child := range children {
			if child.ID != personID {
				// Check if already in siblings (both parents might return same sibling)
				found := false
				for _, existing := range siblings {
					if existing.ID == child.ID {
						found = true
						break
					}
				}
				if !found {
					siblings = append(siblings, child)
				}
			}
		}
	}
	return siblings
}

// getCousinInfo calculates cousin relationships
func getCousinInfo(s *store.Store, fromID, toID int64) string {
	// Get my parents and their siblings
	myParents, _ := s.GetRelatedPeople(fromID, "parent")

	for _, myParent := range myParents {
		// Get grandparents
		grandparents, _ := s.GetRelatedPeople(myParent.ID, "parent")

		for _, grandparent := range grandparents {
			// Get all children of grandparent (my parent's siblings and my parent)
			auntsUncles, _ := s.GetRelatedPeople(grandparent.ID, "child")

			for _, auntUncle := range auntsUncles {
				if auntUncle.ID == myParent.ID {
					continue // Skip my parent
				}

				// Get children of aunt/uncle (my first cousins)
				firstCousins, _ := s.GetRelatedPeople(auntUncle.ID, "child")
				for _, cousin := range firstCousins {
					if cousin.ID == toID {
						return "1st cousin"
					}

					// Check if toID is spouse of my 1st cousin
					cousinSpouses, _ := s.GetSpouses(cousin.ID)
					for _, cousinSp := range cousinSpouses {
						if cousinSp.Person.ID == toID {
							if cousin.Gender == "M" {
								return "wife of 1st cousin"
							}
							return "husband of 1st cousin"
						}
					}

					// Check for 1st cousin once removed (their children)
					cousinChildren, _ := s.GetRelatedPeople(cousin.ID, "child")
					for _, cousinChild := range cousinChildren {
						if cousinChild.ID == toID {
							return "1st cousin once removed"
						}

						// Check if toID is spouse of 1st cousin once removed
						cousinChildSpouses, _ := s.GetSpouses(cousinChild.ID)
						for _, ccSp := range cousinChildSpouses {
							if ccSp.Person.ID == toID {
								if cousinChild.Gender == "M" {
									return "wife of 1st cousin once removed"
								}
								return "husband of 1st cousin once removed"
							}
						}

						// Check for 1st cousin twice removed
						cousinGrandchildren, _ := s.GetRelatedPeople(cousinChild.ID, "child")
						for _, cousinGrandchild := range cousinGrandchildren {
							if cousinGrandchild.ID == toID {
								return "1st cousin twice removed"
							}
						}
					}
				}
			}
		}
	}

	return ""
}

// getInLawRelationship checks for in-law relationships
func getInLawRelationship(s *store.Store, fromID, toID int64) string {
	// Get my spouses
	mySpouses, _ := s.GetSpouses(fromID)

	for _, sp := range mySpouses {
		// Check if toID is parent of my spouse (father-in-law, mother-in-law)
		spouseParents, _ := s.GetRelatedPeople(sp.Person.ID, "parent")
		for _, spParent := range spouseParents {
			if spParent.ID == toID {
				if spParent.Gender == "M" {
					return "father-in-law"
				}
				return "mother-in-law"
			}
		}

		// Check if toID is sibling of my spouse (brother-in-law, sister-in-law)
		spouseSiblings := getSiblings(s, sp.Person.ID)
		for _, spSib := range spouseSiblings {
			if spSib.ID == toID {
				if spSib.Gender == "M" {
					return "brother-in-law"
				}
				return "sister-in-law"
			}
		}

		// Check if toID is child of my spouse (stepchild)
		spouseChildren, _ := s.GetRelatedPeople(sp.Person.ID, "child")
		for _, spChild := range spouseChildren {
			if spChild.ID == toID {
				// Check if also my child
				myChildren, _ := s.GetRelatedPeople(fromID, "child")
				isMyChild := false
				for _, myChild := range myChildren {
					if myChild.ID == spChild.ID {
						isMyChild = true
						break
					}
				}
				if !isMyChild {
					if spChild.Gender == "M" {
						return "stepson"
					}
					return "stepdaughter"
				}
			}
		}
	}

	// Check if toID is spouse of my sibling (brother-in-law, sister-in-law)
	mySiblings := getSiblings(s, fromID)
	for _, sibling := range mySiblings {
		siblingSpouses, _ := s.GetSpouses(sibling.ID)
		for _, sibSp := range siblingSpouses {
			if sibSp.Person.ID == toID {
				if sibSp.Person.Gender == "M" {
					return "brother-in-law"
				}
				return "sister-in-law"
			}
		}
	}

	// Check if toID is spouse of my child (son-in-law, daughter-in-law)
	myChildren, _ := s.GetRelatedPeople(fromID, "child")
	for _, child := range myChildren {
		childSpouses, _ := s.GetSpouses(child.ID)
		for _, childSp := range childSpouses {
			if childSp.Person.ID == toID {
				if childSp.Person.Gender == "M" {
					return "son-in-law"
				}
				return "daughter-in-law"
			}
		}
	}

	return ""
}

// showExportOptionsDialog displays options for GEDCOM export
func showExportOptionsDialog(w fyne.Window, cfg *config.Config, dbPath string, getStore func() *store.Store, currentPersonID *int64) {
	// Get current person's name for display
	var currentPersonName string
	if currentPersonID != nil && *currentPersonID > 0 {
		if person, err := getStore().GetPersonByID(*currentPersonID); err == nil {
			currentPersonName = fmt.Sprintf("%s %s", person.GivenName, person.Surname)
		}
	}

	// Export options
	exportTypeOptions := []string{
		"Export entire database",
	}

	// Only offer branch export if a person is selected
	if currentPersonName != "" {
		exportTypeOptions = append(exportTypeOptions,
			fmt.Sprintf("Export %s and descendants only", currentPersonName))
	}

	exportType := widget.NewRadioGroup(exportTypeOptions, func(string) {})
	exportType.SetSelected(exportTypeOptions[0]) // Default to entire database

	content := container.NewVBox(
		widget.NewLabelWithStyle("Export Options", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		widget.NewSeparator(),
		exportType,
	)

	dialog.ShowCustomConfirm("Export GEDCOM", "Export", "Cancel", content, func(ok bool) {
		if !ok {
			return
		}

		// Show file save dialog
		fd := dialog.NewFileSave(func(uc fyne.URIWriteCloser, err error) {
			if err != nil || uc == nil {
				return
			}
			path := uc.URI().Path()
			uc.Close()

			var exportErr error
			if len(exportTypeOptions) > 1 && exportType.Selected == exportTypeOptions[1] {
				// Export current person branch only
				if currentPersonID == nil || *currentPersonID == 0 {
					dialog.ShowError(fmt.Errorf("No person currently selected."), w)
					return
				}
				exportErr = importer.ExportBranch(path, getStore(), *currentPersonID)
			} else {
				// Export entire database
				exportErr = importer.Export(path, getStore())
			}

			if exportErr != nil {
				dialog.ShowError(exportErr, w)
				return
			}

			dialog.ShowInformation("Export", "Export completed successfully", w)
		}, w)
		fd.SetFileName("export.ged")
		fd.Show()
	}, w)
}

// showBackupDialog creates a timestamped zip backup of the current database
func showBackupDialog(w fyne.Window, dbPath string) {
	// Generate backup filename with timestamp
	baseName := strings.TrimSuffix(filepath.Base(dbPath), filepath.Ext(dbPath))
	timestamp := time.Now().Format("01-02-2006") // MM-DD-YYYY
	defaultFileName := fmt.Sprintf("%s-%s.zip", baseName, timestamp)

	fd := dialog.NewFileSave(func(uc fyne.URIWriteCloser, err error) {
		if err != nil || uc == nil {
			return
		}
		backupPath := uc.URI().Path()
		uc.Close()

		// Create zip file
		zipFile, err := os.Create(backupPath)
		if err != nil {
			dialog.ShowError(fmt.Errorf("Failed to create backup file: %w", err), w)
			return
		}
		defer zipFile.Close()

		zipWriter := zip.NewWriter(zipFile)
		defer zipWriter.Close()

		// Open the database file
		dbFile, err := os.Open(dbPath)
		if err != nil {
			dialog.ShowError(fmt.Errorf("Failed to open database: %w", err), w)
			return
		}
		defer dbFile.Close()

		// Get file info
		dbFileInfo, err := dbFile.Stat()
		if err != nil {
			dialog.ShowError(fmt.Errorf("Failed to get database info: %w", err), w)
			return
		}

		// Create zip entry header
		header, err := zip.FileInfoHeader(dbFileInfo)
		if err != nil {
			dialog.ShowError(fmt.Errorf("Failed to create zip header: %w", err), w)
			return
		}
		header.Name = filepath.Base(dbPath)
		header.Method = zip.Deflate

		// Create writer for this file in the zip
		writer, err := zipWriter.CreateHeader(header)
		if err != nil {
			dialog.ShowError(fmt.Errorf("Failed to create zip entry: %w", err), w)
			return
		}

		// Copy database to zip
		_, err = io.Copy(writer, dbFile)
		if err != nil {
			dialog.ShowError(fmt.Errorf("Failed to write database to backup: %w", err), w)
			return
		}

		dialog.ShowInformation("Backup Complete",
			fmt.Sprintf("Database backed up successfully to:\n%s", backupPath), w)
	}, w)

	fd.SetFileName(defaultFileName)
	fd.Show()
}

// showRestoreDialog restores a database from a zip backup
func showRestoreDialog(w fyne.Window, currentDBPath string, reloadFunc func(string)) {
	fd := dialog.NewFileOpen(func(r fyne.URIReadCloser, err error) {
		if err != nil || r == nil {
			return
		}
		backupPath := r.URI().Path()
		r.Close()

		// Ask for confirmation
		dialog.ShowConfirm("Restore Database",
			"This will replace your current database with the backup.\n\nAre you sure you want to continue?",
			func(confirmed bool) {
				if !confirmed {
					return
				}

				// Open the zip file
				zipReader, err := zip.OpenReader(backupPath)
				if err != nil {
					dialog.ShowError(fmt.Errorf("Failed to open backup file: %w", err), w)
					return
				}

				// Find the .db file in the zip
				var dbFile *zip.File
				for _, file := range zipReader.File {
					if strings.HasSuffix(strings.ToLower(file.Name), ".db") {
						dbFile = file
						break
					}
				}

				if dbFile == nil {
					zipReader.Close()
					dialog.ShowError(fmt.Errorf("No database file found in backup"), w)
					return
				}

				// Extract the database to a temporary file first
				// This avoids issues with the zipReader being closed before the save dialog completes
				tempFile, err := os.CreateTemp("", "restore-*.db")
				if err != nil {
					zipReader.Close()
					dialog.ShowError(fmt.Errorf("Failed to create temporary file: %w", err), w)
					return
				}
				tempPath := tempFile.Name()

				// Open and copy the database from the zip to the temp file
				srcFile, err := dbFile.Open()
				if err != nil {
					tempFile.Close()
					os.Remove(tempPath)
					zipReader.Close()
					dialog.ShowError(fmt.Errorf("Failed to read database from backup: %w", err), w)
					return
				}

				_, err = io.Copy(tempFile, srcFile)
				srcFile.Close()
				tempFile.Close()
				originalDBFileName := dbFile.Name
				zipReader.Close() // Now we can safely close the zip

				if err != nil {
					os.Remove(tempPath)
					dialog.ShowError(fmt.Errorf("Failed to extract database: %w", err), w)
					return
				}

				// Ask where to save the restored database
				// Note: It's safe to restore over the currently open database because
				// we've already extracted to a temp file, so the copy operation
				// won't interfere with the zip file
				saveFd := dialog.NewFileSave(func(uc fyne.URIWriteCloser, err error) {
					// Clean up temp file when done
					defer os.Remove(tempPath)

					if err != nil || uc == nil {
						return
					}
					restorePath := uc.URI().Path()
					uc.Close()

					// Ensure .db extension
					if !strings.HasSuffix(strings.ToLower(restorePath), ".db") {
						restorePath = restorePath + ".db"
					}

					// Copy from temp file to final location
					srcTemp, err := os.Open(tempPath)
					if err != nil {
						dialog.ShowError(fmt.Errorf("Failed to read temporary file: %w", err), w)
						return
					}
					defer srcTemp.Close()

					dstFile, err := os.Create(restorePath)
					if err != nil {
						dialog.ShowError(fmt.Errorf("Failed to create restored database: %w", err), w)
						return
					}

					_, err = io.Copy(dstFile, srcTemp)
					dstFile.Close() // Close immediately to release file handle
					if err != nil {
						os.Remove(restorePath) // Clean up on error
						dialog.ShowError(fmt.Errorf("Failed to restore database: %w", err), w)
						return
					}

					// Reload with the restored database
					reloadFunc(restorePath)

					dialog.ShowInformation("Restore Complete",
						fmt.Sprintf("Database restored successfully from backup:\n%s\n\nNow using: %s",
							filepath.Base(backupPath), filepath.Base(restorePath)), w)
				}, w)

				saveFd.SetFileName(originalDBFileName)
				saveFd.Show()
			}, w)
	}, w)

	fd.SetFilter(storageFilter{ext: ".zip"})
	fd.Show()
}
