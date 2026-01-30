package ui

import (
	"archive/zip"
	"encoding/base64"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"image/color"
	"io"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/jung-kurt/gofpdf"
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/widget"
	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"

	"genealogy/config"
	"genealogy/demo"
	"genealogy/importer"
	"genealogy/store"
)

var mainWindowRef fyne.Window

// Dialog tracking to prevent duplicates
var (
	statisticsDialog   dialog.Dialog
	dataQualityDialog  dialog.Dialog
	livingStatusDialog dialog.Dialog
	settingsDialog     dialog.Dialog
)

// Theme and dialog callback functions from main.go
var (
	LightThemeFunc  func()
	DarkThemeFunc   func()
	SystemThemeFunc func()
	ShowAboutFunc   func()
	ShowHelpFunc    func()
	CheckUpdateFunc func()
)

// normalizeForSearch removes accents and converts to lowercase for better search matching.
// For example: "Jené" becomes "jene", "Müller" becomes "muller"
func normalizeForSearch(s string) string {
	// Transform to NFD (decomposed form) then remove combining marks
	t := transform.Chain(norm.NFD, runes.Remove(runes.In(unicode.Mn)), norm.NFC)
	result, _, _ := transform.String(t, s)
	return strings.ToLower(result)
}

// searchMatch checks if searchText matches within targetText using accent-insensitive substring matching.
// Returns true if normalized searchText is found anywhere in normalized targetText.
func searchMatch(targetText, searchText string) bool {
	if searchText == "" {
		return true
	}
	return strings.Contains(normalizeForSearch(targetText), normalizeForSearch(searchText))
}

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
	var split *container.Split      // Forward declare for resetting offset on reload
	var viewMediaBtn *widget.Button // Forward declare for use in navigation
	var bookmarkBtn *widget.Button  // Forward declare for bookmark toggle

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

		// Update any open fan charts and descendant charts
		UpdateAllFanCharts(getStore(), personID)
		UpdateAllDescendantCharts(getStore(), personID)
		person, err := getStore().GetPersonByID(personID)
		if err != nil {
			dialog.ShowError(err, w)
			return
		}

		// Track person access for Recent People feature
		_ = getStore().TrackPersonAccess(personID)

		// Update View Media button visibility based on whether person has media
		if viewMediaBtn != nil {
			if media, err := getStore().GetMediaForPerson(personID); err == nil && len(media) > 0 {
				viewMediaBtn.Show()
			} else {
				viewMediaBtn.Hide()
			}
		}

		// Update Bookmark button state based on current person's bookmark status
		if bookmarkBtn != nil {
			if person.Bookmarked {
				bookmarkBtn.SetText("★ Bookmarked")
			} else {
				bookmarkBtn.SetText("⭐ Bookmark")
			}
			bookmarkBtn.Refresh()
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
			text := formatPersonName(p)

			// Add bookmark indicator if bookmarked
			if p.Bookmarked {
				text = "★ " + text
			}

			// Add todo indicator if person has pending todos
			if count, err := getStore().CountPendingTodosForPerson(p.ID); err == nil && count > 0 {
				text = "📝 " + text
			}

			// Add source indicator if person has citations
			if count, err := getStore().CountCitationsForPerson(p.ID); err == nil && count > 0 {
				text = "📚 " + text
			}

			// Add research log indicator if person has research logs
			if count, err := getStore().CountResearchLogsForPerson(p.ID); err == nil && count > 0 {
				text = "🔍 " + text
			}

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

			// Update View Media button visibility based on whether person has media
			if viewMediaBtn != nil {
				if media, err := getStore().GetMediaForPerson(personID); err == nil && len(media) > 0 {
					viewMediaBtn.Show()
				} else {
					viewMediaBtn.Hide()
				}
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

	// Function to reload the app with a different database
	// (Defined early so import buttons can reference it)
	var reloadWithDatabase func(string, ...bool)
	reloadWithDatabase = func(newDBPath string, silent ...bool) {
		showSuccessDialog := true
		if len(silent) > 0 && silent[0] {
			showSuccessDialog = false
		}
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

		// Reset split offset to default (prevent narrow left panel issue)
		if split != nil {
			split.SetOffset(0.3) // 30% for the index, 70% for views
		}

		// Reload all data
		refreshPeopleList()

		// Navigate to appropriate person based on preferences (same logic as startup)
		var personToLoad int64
		if cfg != nil {
			if cfg.OpenWithFocusUser {
				// Use Focus User if set
				if focusID := cfg.GetFocusUserForDatabase(newDBPath); focusID > 0 {
					// Verify this person exists
					if _, err := newStore.GetPersonByID(focusID); err == nil {
						personToLoad = focusID
					}
				}
			} else {
				// Use Last Person (default)
				if lastID := cfg.GetLastPersonForDatabase(newDBPath); lastID > 0 {
					// Verify this person exists
					if _, err := newStore.GetPersonByID(lastID); err == nil {
						personToLoad = lastID
					}
				}
			}
		}

		// Fallback to first person if no preference or person not found
		if personToLoad == 0 && len(people) > 0 {
			personToLoad = people[0].ID
		}

		// Navigate to the selected person or clear view
		if personToLoad > 0 {
			navigateToPerson(personToLoad)
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

		if showSuccessDialog {
			dialog.ShowInformation("Database Loaded",
				fmt.Sprintf("Successfully switched to:\n%s\n\nRecent files menu will update on next restart.", filepath.Base(newDBPath)), w)
		}
	}

	// Menu buttons for import/export (defined after reloadWithDatabase)
	importBtn := widget.NewButton("Import GEDCOM", func() {
		showImportOptionsDialog(w, cfg, dbPath, getStore, reloadWithDatabase, refreshAll, ".ged", "GEDCOM", importer.Import)
	})

	importGenoProBtn := widget.NewButton("Import GenoPro", func() {
		showImportOptionsDialog(w, cfg, dbPath, getStore, reloadWithDatabase, refreshAll, ".gno", "GenoPro", importer.ImportGenoPro)
	})

	importGrampsBtn := widget.NewButton("Import Gramps", func() {
		showImportOptionsDialog(w, cfg, dbPath, getStore, reloadWithDatabase, refreshAll, ".db", "Gramps", importer.ImportGramps)
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
			fmt.Sprintf("Are you sure you want to delete %s? This will remove all relationships.",
				formatPersonName(*person)),
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

	setFocusBtn := widget.NewButton("⭐ Set Focus", func() {
		if currentPersonID <= 0 {
			dialog.ShowInformation("Set Focus", "Please select a person first", w)
			return
		}
		person, err := getStore().GetPersonByID(currentPersonID)
		if err != nil {
			dialog.ShowError(err, w)
			return
		}

		dialog.ShowConfirm("Set Focus Person",
			fmt.Sprintf("Set %s as your focus person?\n\nThe focus person is used for relationship calculations and quick navigation.",
				formatPersonName(*person)),
			func(confirmed bool) {
				if confirmed {
					cfg.SetFocusUserForDatabase(dbPath, currentPersonID)
					cfg.OpenWithFocusUser = true
					if err := cfg.Save(); err != nil {
						dialog.ShowError(fmt.Errorf("Failed to save focus person: %w", err), w)
						return
					}
					// Update status bar to show new relationships
					if updateStatus != nil {
						updateStatus()
					}
					dialog.ShowInformation("Focus Person Set",
						fmt.Sprintf("%s is now your focus person.\n\nUse the 'Focus Person' button to quickly navigate back to them.",
							formatPersonName(*person)), w)
				}
			}, w)
	})

	// Media Library button (for keyboard shortcut)
	mediaLibraryBtn := widget.NewButton("Media Library", func() {
		showMediaLibrary(w, getStore())
	})

	// View Media button - will be shown/hidden based on current person's media
	viewMediaBtn = widget.NewButton("📷 View Media", func() {
		if currentPersonID > 0 {
			person, err := getStore().GetPersonByID(currentPersonID)
			if err == nil {
				showMediaManager(w, getStore(), currentPersonID, fmt.Sprintf("%s %s", person.GivenName, person.Surname))
			}
		}
	})
	viewMediaBtn.Hide() // Initially hidden

	// Bookmark button - toggles bookmark status for current person
	bookmarkBtn = widget.NewButton("⭐ Bookmark", func() {
		if currentPersonID > 0 {
			person, err := getStore().GetPersonByID(currentPersonID)
			if err != nil {
				dialog.ShowError(err, w)
				return
			}
			// Toggle bookmark status
			newStatus := !person.Bookmarked
			if err := getStore().SetBookmarked(currentPersonID, newStatus); err != nil {
				dialog.ShowError(err, w)
				return
			}
			// Update button text to reflect new status
			if newStatus {
				bookmarkBtn.SetText("★ Bookmarked")
			} else {
				bookmarkBtn.SetText("⭐ Bookmark")
			}
			bookmarkBtn.Refresh()
		}
	})

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
		showBackupDialog(w, dbPath, getStore)
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

	conflictsBtn := widget.NewButton("Conflicts Report", func() {
		showConflictsReport(w, getStore(), navigateToPerson)
	})

	duplicatesBtn := widget.NewButton("Duplicate Detection", func() {
		showDuplicateDetectionReport(w, getStore(), navigateToPerson, refreshAll)
	})

	descendantBtn := widget.NewButton("Descendant (Pedigree) Report", func() {
		showDescendantReport(w, getStore(), currentPersonID, navigateToPerson)
	})

	ancestorBtn := widget.NewButton("Ancestor (Ahnentafel) Report", func() {
		showAncestorReport(w, getStore(), currentPersonID, navigateToPerson)
	})

	familyGroupBtn := widget.NewButton("Family Group Sheet", func() {
		showFamilyGroupSheet(w, getStore(), currentPersonID, navigateToPerson)
	})

	timelineBtn := widget.NewButton("Timeline View", func() {
		showTimelineView(w, getStore(), navigateToPerson)
	})

	settingsBtn := widget.NewButton("Settings", func() {
		showSettingsDialog(w, cfg, dbPath, getStore(), func() {
			// Callback when settings change - no need to refresh views
			_ = cfg.Save()
		})
	})

	// Popup menu buttons for toolbar (defined after all individual buttons)
	// File button - shows popup menu with file operations
	fileBtn := widget.NewButton("📁 File", func() {
		showFilePopupMenu(w, cfg, dbPath, getStore, reloadWithDatabase, newDatabaseBtn, openDatabaseBtn, importBtn, importGenoProBtn, importGrampsBtn, exportBtn)
	})

	// Media button - shows popup menu with media options
	mediaBtn := widget.NewButton("🖼️ Media", func() {
		showMediaPopupMenu(w, getStore(), currentPersonID)
	})

	// Tools button - shows popup menu with tools
	toolsBtn := widget.NewButton("🔧 Tools", func() {
		showToolsPopupMenu(w, getStore())
	})

	// Reports button - shows popup menu with all reports
	reportsBtn := widget.NewButton("📊 Reports", func() {
		showReportsPopupMenu(w, getStore(), currentPersonID, navigateToPerson)
	})

	// Settings button - shows popup menu with settings options
	settingsPopupBtn := widget.NewButton("⚙️ Settings", func() {
		showSettingsPopupMenu(w, cfg, dbPath, getStore, settingsBtn)
	})

	// Help button - shows popup menu with help options
	helpBtn := widget.NewButton("❓ Help", func() {
		showHelpPopupMenu(w, cfg, reloadWithDatabase)
	})

	// Top toolbar - with quick access popup menus
	toolbar := container.NewHBox(focusPersonBtn, setFocusBtn, bookmarkBtn, addPersonBtn, deletePersonBtn, fileBtn, mediaBtn, toolsBtn, reportsBtn, settingsPopupBtn, helpBtn)

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

	// Add minimum width constraint to left panel (prevents narrow panel with short names)
	minWidthSpacer := canvas.NewRectangle(color.Transparent)
	minWidthSpacer.SetMinSize(fyne.NewSize(220, 0))
	leftPanelWithMinWidth := container.NewMax(minWidthSpacer, leftPanel)

	// Main split: left panel (people index) and tabbed views
	split = container.NewHSplit(leftPanelWithMinWidth, tabs)
	split.SetOffset(0.3) // 30% for the index, 70% for views (more space for names and easier to resize)

	// Main layout with status bar at bottom
	content := container.NewBorder(toolbar, statusLabel, nil, nil, split)

	w.SetContent(content)
	w.Resize(fyne.NewSize(1400, 850)) // Increased default size for better usability

	// Setup system tray and menus (if supported)
	setupMenus(a, w, cfg, newDatabaseBtn, openDatabaseBtn, backupBtn, restoreBtn, importBtn, importGenoProBtn, importGrampsBtn, exportBtn, dataQualityBtn, livingStatusBtn, conflictsBtn, duplicatesBtn, descendantBtn, ancestorBtn, familyGroupBtn, timelineBtn, settingsBtn, getStore, navigateToPerson, refreshAll, func() int64 { return currentPersonID }, reloadWithDatabase)

	// Statistics dashboard button (for keyboard shortcut)
	statsBtn := widget.NewButton("Statistics", func() {
		showStatisticsDashboard(w, getStore())
	})

	// Setup keyboard shortcuts
	setupKeyboardShortcuts(a, w, cfg, tabs, searchEntry, addPersonBtn, deletePersonBtn, focusPersonBtn,
		settingsBtn, dataQualityBtn, reportsBtn, statsBtn, openDatabaseBtn, backupBtn, mediaLibraryBtn, bookmarkBtn, editPerson, &currentPersonID, getStore, navigateToPerson)

	w.ShowAndRun()
}

// setupMenus creates the system tray and window menus
func setupMenus(a fyne.App, w fyne.Window, cfg *config.Config, newDBBtn, openDBBtn, backupBtn, restoreBtn, importBtn, importGenoProBtn, importGrampsBtn, exportBtn, dataQBtn, livingStatusBtn, conflictsBtn, duplicatesBtn, descendantBtn, ancestorBtn, familyGroupBtn, timelineBtn, settingsBtn *widget.Button, getStore func() *store.Store, navigateToPerson func(int64), refreshAll func(), getCurrentPersonID func() int64, reloadWithDatabase func(string, ...bool)) {
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

	// Recent Files submenu
	recentFiles := fyne.NewMenuItem("Recent Files", nil)
	recentFilesMenu := fyne.NewMenu("")

	// Populate recent files from config
	if len(cfg.RecentDatabases) > 0 {
		for _, dbPath := range cfg.RecentDatabases {
			// Capture dbPath in closure
			path := dbPath
			menuLabel := filepath.Base(path)

			// Add full path as hint if name is generic
			if menuLabel == "genealogy.db" || strings.Contains(menuLabel, "genealogy") {
				menuLabel = fmt.Sprintf("%s  (%s)", filepath.Base(path), filepath.Dir(path))
			}

			recentItem := fyne.NewMenuItem(menuLabel, func() {
				// Check if file exists
				if _, err := os.Stat(path); err != nil {
					dialog.ShowError(fmt.Errorf("Database not found: %s", path), w)
					return
				}

				// Switch to this database
				reloadWithDatabase(path)
			})
			recentFilesMenu.Items = append(recentFilesMenu.Items, recentItem)
		}
	} else {
		// No recent files
		noRecent := fyne.NewMenuItem("(no recent files)", func() {})
		noRecent.Disabled = true
		recentFilesMenu.Items = append(recentFilesMenu.Items, noRecent)
	}

	recentFiles.ChildMenu = recentFilesMenu

	clearRecent := fyne.NewMenuItem("Clear Recent Files", func() {
		if len(cfg.RecentDatabases) == 0 {
			dialog.ShowInformation("Clear Recent Files",
				"The recent files list is already empty.", w)
			return
		}

		dialog.ShowConfirm("Clear Recent Files",
			fmt.Sprintf("Clear %d recent file(s) from the list?", len(cfg.RecentDatabases)),
			func(confirmed bool) {
				if confirmed {
					cfg.ClearRecentDatabases()
					if err := cfg.Save(); err != nil {
						dialog.ShowError(fmt.Errorf("Failed to save config: %w", err), w)
						return
					}
					dialog.ShowInformation("Recent Files Cleared",
						"The recent files list has been cleared.\n\nMenu will update on next restart.", w)
				}
			}, w)
	})

	backup := fyne.NewMenuItem("Backup Database...", func() {
		backupBtn.OnTapped()
	})
	restore := fyne.NewMenuItem("Restore Database...", func() {
		restoreBtn.OnTapped()
	})
	maintenance := fyne.NewMenuItem("Database Maintenance...", func() {
		showDatabaseMaintenanceDialog(w, getStore())
	})
	importGED := fyne.NewMenuItem("Import GEDCOM...", func() {
		importBtn.OnTapped()
	})
	importGNO := fyne.NewMenuItem("Import GenoPro...", func() {
		importGenoProBtn.OnTapped()
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
	statistics := fyne.NewMenuItem("Statistics Dashboard", func() {
		showStatisticsDashboard(w, getStore())
	})

	dataQuality := fyne.NewMenuItem("Data Quality Report", func() {
		showDataQualityReport(w, getStore(), navigateToPerson)
	})

	livingStatus := fyne.NewMenuItem("Living Status Report", func() {
		showLivingStatusReport(w, getStore(), navigateToPerson)
	})

	conflictsReport := fyne.NewMenuItem("Conflicts Report", func() {
		showConflictsReport(w, getStore(), navigateToPerson)
	})

	duplicatesReport := fyne.NewMenuItem("Duplicate Detection", func() {
		showDuplicateDetectionReport(w, getStore(), navigateToPerson, refreshAll)
	})

	descendantReport := fyne.NewMenuItem("Descendant (Pedigree) Report", func() {
		showDescendantReport(w, getStore(), getCurrentPersonID(), navigateToPerson)
	})

	ancestorReport := fyne.NewMenuItem("Ancestor (Ahnentafel) Report", func() {
		showAncestorReport(w, getStore(), getCurrentPersonID(), navigateToPerson)
	})

	familyGroupReport := fyne.NewMenuItem("Family Group Sheet", func() {
		showFamilyGroupSheet(w, getStore(), getCurrentPersonID(), navigateToPerson)
	})

	timelineReport := fyne.NewMenuItem("Timeline View", func() {
		showTimelineView(w, getStore(), navigateToPerson)
	})

	reviewedItemsReport := fyne.NewMenuItem("Reviewed Items", func() {
		showReviewedItemsReport(w, getStore(), navigateToPerson)
	})

	geographicReport := fyne.NewMenuItem("Geographic Distribution", func() {
		showGeographicDistributionReport(w, getStore(), navigateToPerson)
	})

	fanChartView := fyne.NewMenuItem("Fan Chart...", func() {
		showFanChartDialog(w, getStore(), getCurrentPersonID(), navigateToPerson)
	})

	descendantChartView := fyne.NewMenuItem("Descendant Chart...", func() {
		showDescendantChartDialog(w, getStore(), getCurrentPersonID(), navigateToPerson)
	})

	mapView := fyne.NewMenuItem("Map View...", func() {
		showMapViewDialog(w, getStore(), getCurrentPersonID(), navigateToPerson)
	})

	recentPeopleReport := fyne.NewMenuItem("Recent People", func() {
		showRecentPeopleReport(w, getStore(), navigateToPerson)
	})

	bookmarkedPeopleReport := fyne.NewMenuItem("Bookmarked People", func() {
		showBookmarkedPeopleReport(w, getStore(), navigateToPerson)
	})

	allTodosReport := fyne.NewMenuItem("All Pending To-Dos", func() {
		showAllTodosReport(w, getStore(), navigateToPerson)
	})

	completedTodosReport := fyne.NewMenuItem("Completed To-Dos", func() {
		showCompletedTodosReport(w, getStore(), navigateToPerson)
	})

	unsourcedPeopleReport := fyne.NewMenuItem("People Without Sources", func() {
		showUnsourcedPeopleReport(w, getStore(), navigateToPerson)
	})

	allSourcesReport := fyne.NewMenuItem("All Sources", func() {
		showAllSourcesReport(w, getStore(), navigateToPerson)
	})

	wellDocumentedReport := fyne.NewMenuItem("Well-Documented People", func() {
		showWellDocumentedPeopleReport(w, getStore(), navigateToPerson)
	})

	recentResearchReport := fyne.NewMenuItem("Recent Research Activity", func() {
		showRecentResearchReport(w, getStore())
	})

	advancedSearch := fyne.NewMenuItem("Advanced Search...", func() {
		showAdvancedSearchDialog(w, getStore(), navigateToPerson)
	})

	relationshipCalc := fyne.NewMenuItem("Relationship Calculator", func() {
		showRelationshipCalculator(w, getStore(), navigateToPerson)
	})

	massMarkLivingReport := fyne.NewMenuItem("Mark as Living (Bulk)", func() {
		showMassMarkLivingReport(w, getStore(), func() {
			refreshAll()
		})
	})

	// Media menu items
	mediaLibraryMenuItem := fyne.NewMenuItem("Media Library", func() {
		showMediaLibrary(w, getStore())
	})

	addMediaMenuItem := fyne.NewMenuItem("Add Media", func() {
		showGlobalMediaDialog(w, getStore())
	})

	// Sources menu item
	sourcesLibraryMenuItem := fyne.NewMenuItem("Sources Library", func() {
		showSourcesLibrary(w, getStore())
	})

	// Research Log menu item
	researchLogMenuItem := fyne.NewMenuItem("Research Log", func() {
		showResearchLogManager(w, getStore())
	})

	// Settings menu items
	settingsDialog := fyne.NewMenuItem("Preferences...", func() {
		settingsBtn.OnTapped()
	})
	keyboardShortcuts := fyne.NewMenuItem("Keyboard Shortcuts...", func() {
		showKeyboardShortcutsDialog(w, cfg)
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
	loadDemo := fyne.NewMenuItem("Load Demo Database", func() {
		showLoadDemoDialog(w, cfg, reloadWithDatabase)
	})

	// Tools menu items
	geocodingTool := fyne.NewMenuItem("Batch Geocoding...", func() {
		showGeocodingToolDialog(w, getStore())
	})

	globalSearchReplace := fyne.NewMenuItem("Global Search and Replace...", func() {
		showGlobalSearchReplaceDialog(w, getStore())
	})

	nameCaseConversion := fyne.NewMenuItem("Name Case Conversion...", func() {
		showNameCaseConversionDialog(w, getStore())
	})

	dateCalculator := fyne.NewMenuItem("Date Calculator...", func() {
		showDateCalculatorDialog(w)
	})

	// System tray menu with proper submenus using ChildMenu
	// Create File submenu
	fileSubMenu := fyne.NewMenu("File",
		newDB, openDB, recentFiles, clearRecent,
		fyne.NewMenuItemSeparator(),
		backup, restore, maintenance,
		fyne.NewMenuItemSeparator(),
		importGED, importGNO, importGramps,
		fyne.NewMenuItemSeparator(),
		exportGED)
	fileMenuItem := fyne.NewMenuItem("File", nil)
	fileMenuItem.ChildMenu = fileSubMenu

	// Create Media submenu
	mediaSubMenu := fyne.NewMenu("Media",
		mediaLibraryMenuItem,
		addMediaMenuItem,
		fyne.NewMenuItemSeparator(),
		sourcesLibraryMenuItem)
	mediaMenuItem := fyne.NewMenuItem("Media", nil)
	mediaMenuItem.ChildMenu = mediaSubMenu

	// Create Reports submenu (alphabetically sorted within groups)
	reportsSubMenu := fyne.NewMenu("Reports",
		// Utilities (alphabetical)
		advancedSearch,
		relationshipCalc,
		statistics,
		fyne.NewMenuItemSeparator(),
		// List Reports (alphabetical)
		allTodosReport,
		bookmarkedPeopleReport,
		completedTodosReport,
		recentPeopleReport,
		recentResearchReport,
		fyne.NewMenuItemSeparator(),
		// Data Quality Reports (alphabetical)
		allSourcesReport,
		conflictsReport,
		dataQuality,
		duplicatesReport,
		livingStatus,
		unsourcedPeopleReport,
		wellDocumentedReport,
		fyne.NewMenuItemSeparator(),
		// Individual Reports (alphabetical)
		ancestorReport,
		descendantReport,
		familyGroupReport,
		timelineReport,
		fyne.NewMenuItemSeparator(),
		// Chart Views (alphabetical)
		descendantChartView,
		fanChartView,
		geographicReport,
		fyne.NewMenuItemSeparator(),
		// Actions (alphabetical)
		massMarkLivingReport,
		reviewedItemsReport)
	reportsMenuItem := fyne.NewMenuItem("Reports", nil)
	reportsMenuItem.ChildMenu = reportsSubMenu

	// Create Settings submenu
	settingsSubMenu := fyne.NewMenu("Settings",
		settingsDialog,
		keyboardShortcuts,
		fyne.NewMenuItemSeparator(),
		settingsLight,
		settingsDark,
		settingsSystem)
	settingsMenuItem := fyne.NewMenuItem("Settings", nil)
	settingsMenuItem.ChildMenu = settingsSubMenu

	// Create Help submenu
	helpSubMenu := fyne.NewMenu("Help",
		about,
		updtchk,
		help,
		fyne.NewMenuItemSeparator(),
		loadDemo)
	helpMenuItem := fyne.NewMenuItem("Help", nil)
	helpMenuItem.ChildMenu = helpSubMenu

	// Main system tray menu with submenus
	// Tools menu items (defined earlier in the file)
	toolsSubMenu := fyne.NewMenu("Tools",
		geocodingTool,
		dateCalculator,
		globalSearchReplace,
		nameCaseConversion)
	toolsMenuItem := fyne.NewMenuItem("Tools", nil)
	toolsMenuItem.ChildMenu = toolsSubMenu

	menu := fyne.NewMenu("KrankyBear Genealogy",
		show, hide,
		fyne.NewMenuItemSeparator(),
		fileMenuItem,
		mediaMenuItem,
		toolsMenuItem,
		reportsMenuItem,
		settingsMenuItem,
		helpMenuItem,
		fyne.NewMenuItemSeparator(),
		quit)
	desk.SetSystemTrayMenu(menu)
	// Icon is set from main package, we just set up the menu here

	// Setup main menu bar (for window menu bar with submenus)
	fileMenu := fyne.NewMenu("File", newDB, openDB, recentFiles, clearRecent, fyne.NewMenuItemSeparator(),
		backup, restore, maintenance, fyne.NewMenuItemSeparator(),
		importGED, importGNO, importGramps, exportGED, fyne.NewMenuItemSeparator(), quit)
	mediaMenu := fyne.NewMenu("Media", mediaLibraryMenuItem, addMediaMenuItem, fyne.NewMenuItemSeparator(), sourcesLibraryMenuItem, researchLogMenuItem)
	toolsMenu := fyne.NewMenu("Tools", geocodingTool, dateCalculator, globalSearchReplace, nameCaseConversion)
	reportsMenu := fyne.NewMenu("Reports",
		// Utilities (alphabetical)
		advancedSearch, relationshipCalc, statistics,
		fyne.NewMenuItemSeparator(),
		// List Reports (alphabetical)
		allTodosReport, bookmarkedPeopleReport, completedTodosReport, recentPeopleReport, recentResearchReport,
		fyne.NewMenuItemSeparator(),
		// Data Quality Reports (alphabetical)
		allSourcesReport, conflictsReport, dataQuality, duplicatesReport, livingStatus, unsourcedPeopleReport, wellDocumentedReport,
		fyne.NewMenuItemSeparator(),
		// Individual Reports (alphabetical)
		ancestorReport, descendantReport, familyGroupReport, timelineReport,
		fyne.NewMenuItemSeparator(),
		// Chart Views (alphabetical)
		descendantChartView, fanChartView, geographicReport, mapView,
		fyne.NewMenuItemSeparator(),
		// Actions (alphabetical)
		massMarkLivingReport, reviewedItemsReport)
	settingsMenu := fyne.NewMenu("Settings", settingsDialog, keyboardShortcuts, fyne.NewMenuItemSeparator(),
		settingsLight, settingsDark, settingsSystem)
	helpMenu := fyne.NewMenu("Help", about, updtchk, help, fyne.NewMenuItemSeparator(), loadDemo)
	cmenu := fyne.NewMainMenu(fileMenu, mediaMenu, toolsMenu, reportsMenu, settingsMenu, helpMenu)
	w.SetMainMenu(cmenu)
}

// showReportsPopupMenu shows a popup menu with all reports organized in submenus
func showReportsPopupMenu(w fyne.Window, s *store.Store, currentPersonID int64, navigateFunc func(int64)) {
	// Utilities submenu
	utilitiesMenu := fyne.NewMenu("",
		fyne.NewMenuItem("Advanced Search", func() {
			showAdvancedSearchDialog(w, s, navigateFunc)
		}),
		fyne.NewMenuItem("Relationship Calculator", func() {
			showRelationshipCalculator(w, s, navigateFunc)
		}),
		fyne.NewMenuItem("Statistics Dashboard", func() {
			showStatisticsDashboard(w, s)
		}),
	)
	utilities := fyne.NewMenuItem("Utilities", nil)
	utilities.ChildMenu = utilitiesMenu
	
	// Lists submenu
	listsMenu := fyne.NewMenu("",
		fyne.NewMenuItem("All To-Dos", func() {
			showAllTodosReport(w, s, navigateFunc)
		}),
		fyne.NewMenuItem("Bookmarked People", func() {
			showBookmarkedPeopleReport(w, s, navigateFunc)
		}),
		fyne.NewMenuItem("Completed To-Dos", func() {
			showCompletedTodosReport(w, s, navigateFunc)
		}),
		fyne.NewMenuItem("Recent People", func() {
			showRecentPeopleReport(w, s, navigateFunc)
		}),
		fyne.NewMenuItem("Recent Research", func() {
			showRecentResearchReport(w, s)
		}),
	)
	lists := fyne.NewMenuItem("Lists", nil)
	lists.ChildMenu = listsMenu
	
	// Data Quality submenu
	dataQualityMenu := fyne.NewMenu("",
		fyne.NewMenuItem("All Sources", func() {
			showSourcesLibrary(w, s)
		}),
		fyne.NewMenuItem("Conflicts Report", func() {
			showConflictsReport(w, s, navigateFunc)
		}),
		fyne.NewMenuItem("Data Quality Report", func() {
			showDataQualityReport(w, s, navigateFunc)
		}),
		fyne.NewMenuItem("Duplicate Detection", func() {
			showDuplicateDetectionReport(w, s, navigateFunc, func() {})
		}),
		fyne.NewMenuItem("Living Status Report", func() {
			showLivingStatusReport(w, s, navigateFunc)
		}),
		fyne.NewMenuItem("Unsourced People", func() {
			showUnsourcedPeopleReport(w, s, navigateFunc)
		}),
		fyne.NewMenuItem("Well Documented", func() {
			showWellDocumentedPeopleReport(w, s, navigateFunc)
		}),
	)
	dataQuality := fyne.NewMenuItem("Data Quality", nil)
	dataQuality.ChildMenu = dataQualityMenu
	
	// Individual Reports submenu
	individualMenu := fyne.NewMenu("",
		fyne.NewMenuItem("Ancestor Report", func() {
			showAncestorReport(w, s, currentPersonID, navigateFunc)
		}),
		fyne.NewMenuItem("Descendant Report", func() {
			showDescendantReport(w, s, currentPersonID, navigateFunc)
		}),
		fyne.NewMenuItem("Family Group Sheet", func() {
			showFamilyGroupSheet(w, s, currentPersonID, navigateFunc)
		}),
		fyne.NewMenuItem("Timeline View", func() {
			showTimelineView(w, s, navigateFunc)
		}),
	)
	individual := fyne.NewMenuItem("Individual Reports", nil)
	individual.ChildMenu = individualMenu
	
	// Chart Views submenu
	chartsMenu := fyne.NewMenu("",
		fyne.NewMenuItem("Descendant Chart View", func() {
			showDescendantChartDialog(w, s, currentPersonID, navigateFunc)
		}),
		fyne.NewMenuItem("Fan Chart View", func() {
			showFanChartDialog(w, s, currentPersonID, navigateFunc)
		}),
		fyne.NewMenuItem("Map View", func() {
			showMapViewDialog(w, s, currentPersonID, navigateFunc)
		}),
		fyne.NewMenuItem("Geographic Distribution", func() {
			showGeographicDistributionReport(w, s, navigateFunc)
		}),
	)
	charts := fyne.NewMenuItem("Chart Views", nil)
	charts.ChildMenu = chartsMenu
	
	// Export submenu
	exportMenu := fyne.NewMenu("",
		fyne.NewMenuItem("People to CSV", func() {
			exportPeopleToCSV(w, s)
		}),
		fyne.NewMenuItem("Timeline to CSV", func() {
			exportTimelineToCSV(w, s)
		}),
		fyne.NewMenuItem("Surnames to CSV", func() {
			exportSurnamesToCSV(w, s)
		}),
		fyne.NewMenuItem("Places to CSV", func() {
			exportPlacesToCSV(w, s)
		}),
		fyne.NewMenuItem("Sources to CSV", func() {
			exportSourcesToCSV(w, s)
		}),
	)
	exports := fyne.NewMenuItem("Export to CSV", nil)
	exports.ChildMenu = exportMenu
	
	// Actions submenu
	actionsMenu := fyne.NewMenu("",
		fyne.NewMenuItem("Generate Family Website", func() {
			exportCompleteWebsite(w, s)
		}),
		fyne.NewMenuItemSeparator(),
		exports,
		fyne.NewMenuItemSeparator(),
		fyne.NewMenuItem("Mass Mark Living", func() {
			showMassMarkLivingReport(w, s, func() {})
		}),
		fyne.NewMenuItem("Reviewed Items", func() {
			showReviewedItemsReport(w, s, navigateFunc)
		}),
	)
	actions := fyne.NewMenuItem("Actions", nil)
	actions.ChildMenu = actionsMenu
	
	// Main menu with submenus
	items := []*fyne.MenuItem{
		utilities,
		lists,
		dataQuality,
		individual,
		charts,
		actions,
	}
	
	menu := fyne.NewMenu("", items...)
	
	// Show popup at center of window
	pos := fyne.NewPos(w.Canvas().Size().Width/2, w.Canvas().Size().Height/2)
	widget.ShowPopUpMenuAtPosition(menu, w.Canvas(), pos)
}

// showMediaPopupMenu shows a popup menu with all media options
func showMediaPopupMenu(w fyne.Window, s *store.Store, currentPersonID int64) {
	items := []*fyne.MenuItem{
		fyne.NewMenuItem("Add Media", func() {
			showGlobalMediaDialog(w, s)
		}),
		fyne.NewMenuItem("Media Library", func() {
			showMediaLibrary(w, s)
		}),
		fyne.NewMenuItem("Research Log Manager", func() {
			showResearchLogManager(w, s)
		}),
		fyne.NewMenuItem("Sources Library", func() {
			showSourcesLibrary(w, s)
		}),
	}
	
	// Add View Media if current person has media
	if currentPersonID > 0 {
		if media, err := s.GetMediaForPerson(currentPersonID); err == nil && len(media) > 0 {
			person, err := s.GetPersonByID(currentPersonID)
			if err == nil {
				items = append(items, fyne.NewMenuItemSeparator())
				items = append(items, fyne.NewMenuItem(fmt.Sprintf("View Media (%d)", len(media)), func() {
					showMediaManager(w, s, currentPersonID, fmt.Sprintf("%s %s", person.GivenName, person.Surname))
				}))
			}
		}
	}
	
	menu := fyne.NewMenu("", items...)
	pos := fyne.NewPos(w.Canvas().Size().Width/2, w.Canvas().Size().Height/2)
	widget.ShowPopUpMenuAtPosition(menu, w.Canvas(), pos)
}

// showFilePopupMenu shows a popup menu with file operations
func showFilePopupMenu(w fyne.Window, cfg *config.Config, dbPath string, getStore func() *store.Store, reloadWithDatabase func(string, ...bool), newDBBtn, openDBBtn, importBtn, importGenoProBtn, importGrampsBtn, exportBtn *widget.Button) {
	items := []*fyne.MenuItem{
		fyne.NewMenuItem("New Database", func() {
			if newDBBtn != nil {
				newDBBtn.OnTapped()
			}
		}),
		fyne.NewMenuItem("Open Database", func() {
			if openDBBtn != nil {
				openDBBtn.OnTapped()
			}
		}),
	}
	
	// Add Recent Files submenu if there are any
	if len(cfg.RecentDatabases) > 0 {
		recentFilesMenu := fyne.NewMenu("")
		for i, recentDB := range cfg.RecentDatabases {
			if i >= 10 {
				break // Limit to 10 recent files
			}
			dbPathCopy := recentDB
			recentFilesMenu.Items = append(recentFilesMenu.Items,
				fyne.NewMenuItem(filepath.Base(recentDB), func() {
					reloadWithDatabase(dbPathCopy, true) // true = skip confirmation
				}))
		}
		
		recentFilesItem := fyne.NewMenuItem("Recent Files", nil)
		recentFilesItem.ChildMenu = recentFilesMenu
		items = append(items, recentFilesItem)
		
		items = append(items, fyne.NewMenuItem("Clear Recent Files", func() {
			if len(cfg.RecentDatabases) == 0 {
				dialog.ShowInformation("Clear Recent Files", "The recent files list is already empty.", w)
				return
			}
			dialog.ShowConfirm("Clear Recent Files",
				fmt.Sprintf("Are you sure you want to clear %d recent file(s)?", len(cfg.RecentDatabases)),
				func(ok bool) {
					if ok {
						cfg.RecentDatabases = []string{}
						_ = cfg.Save()
						dialog.ShowInformation("Cleared", "Recent files list has been cleared.", w)
					}
				}, w)
		}))
	}
	
	items = append(items, fyne.NewMenuItemSeparator())
	items = append(items, fyne.NewMenuItem("Backup Database", func() {
		showBackupDialog(w, dbPath, getStore)
	}))
	items = append(items, fyne.NewMenuItem("Database Maintenance", func() {
		showDatabaseMaintenanceDialog(w, getStore())
	}))
	items = append(items, fyne.NewMenuItem("Restore Database", func() {
		showRestoreDialog(w, dbPath, reloadWithDatabase)
	}))
	items = append(items, fyne.NewMenuItemSeparator())
	items = append(items, fyne.NewMenuItem("Export GEDCOM", func() {
		if exportBtn != nil {
			exportBtn.OnTapped()
		}
	}))
	items = append(items, fyne.NewMenuItem("Import GEDCOM", func() {
		if importBtn != nil {
			importBtn.OnTapped()
		}
	}))
	items = append(items, fyne.NewMenuItem("Import from GenoPro", func() {
		if importGenoProBtn != nil {
			importGenoProBtn.OnTapped()
		}
	}))
	items = append(items, fyne.NewMenuItem("Import from Gramps", func() {
		if importGrampsBtn != nil {
			importGrampsBtn.OnTapped()
		}
	}))
	
	menu := fyne.NewMenu("", items...)
	pos := fyne.NewPos(w.Canvas().Size().Width/2, w.Canvas().Size().Height/2)
	widget.ShowPopUpMenuAtPosition(menu, w.Canvas(), pos)
}

// showToolsPopupMenu shows a popup menu with tools
func showToolsPopupMenu(w fyne.Window, s *store.Store) {
	items := []*fyne.MenuItem{
		fyne.NewMenuItem("Batch Geocoding", func() {
			showGeocodingToolDialog(w, s)
		}),
		fyne.NewMenuItem("Date Calculator", func() {
			showDateCalculatorDialog(w)
		}),
		fyne.NewMenuItem("Global Search & Replace", func() {
			showGlobalSearchReplaceDialog(w, s)
		}),
		fyne.NewMenuItem("Name Case Conversion", func() {
			showNameCaseConversionDialog(w, s)
		}),
	}
	
	menu := fyne.NewMenu("", items...)
	pos := fyne.NewPos(w.Canvas().Size().Width/2, w.Canvas().Size().Height/2)
	widget.ShowPopUpMenuAtPosition(menu, w.Canvas(), pos)
}

// showSettingsPopupMenu shows a popup menu with settings options
func showSettingsPopupMenu(w fyne.Window, cfg *config.Config, dbPath string, getStore func() *store.Store, settingsBtn *widget.Button) {
	items := []*fyne.MenuItem{
		fyne.NewMenuItem("Settings", func() {
			if settingsBtn != nil {
				settingsBtn.OnTapped()
			}
		}),
		fyne.NewMenuItem("Keyboard Shortcuts", func() {
			showKeyboardShortcutsDialog(w, cfg)
		}),
		fyne.NewMenuItemSeparator(),
		fyne.NewMenuItem("Light Theme", func() {
			if LightThemeFunc != nil {
				LightThemeFunc()
			}
		}),
		fyne.NewMenuItem("Dark Theme", func() {
			if DarkThemeFunc != nil {
				DarkThemeFunc()
			}
		}),
		fyne.NewMenuItem("System Theme", func() {
			if SystemThemeFunc != nil {
				SystemThemeFunc()
			}
		}),
	}
	
	menu := fyne.NewMenu("", items...)
	pos := fyne.NewPos(w.Canvas().Size().Width/2, w.Canvas().Size().Height/2)
	widget.ShowPopUpMenuAtPosition(menu, w.Canvas(), pos)
}

// showHelpPopupMenu shows a popup menu with help options
func showHelpPopupMenu(w fyne.Window, cfg *config.Config, reloadWithDatabase func(string, ...bool)) {
	items := []*fyne.MenuItem{
		fyne.NewMenuItem("About", func() {
			if ShowAboutFunc != nil {
				ShowAboutFunc()
			}
		}),
		fyne.NewMenuItem("Check for Updates", func() {
			if CheckUpdateFunc != nil {
				CheckUpdateFunc()
			}
		}),
		fyne.NewMenuItem("Help", func() {
			if ShowHelpFunc != nil {
				ShowHelpFunc()
			}
		}),
		fyne.NewMenuItemSeparator(),
		fyne.NewMenuItem("Load Demo Database", func() {
			showLoadDemoDialog(w, cfg, reloadWithDatabase)
		}),
	}
	
	menu := fyne.NewMenu("", items...)
	pos := fyne.NewPos(w.Canvas().Size().Width/2, w.Canvas().Size().Height/2)
	widget.ShowPopUpMenuAtPosition(menu, w.Canvas(), pos)
}

// setupKeyboardShortcuts registers keyboard shortcuts for common actions
func setupKeyboardShortcuts(a fyne.App, w fyne.Window, cfg *config.Config, tabs *container.AppTabs, searchEntry *widget.Entry,
	addPersonBtn, deletePersonBtn, focusPersonBtn, settingsBtn, dataQualityBtn, reportsBtn, statsBtn, openDatabaseBtn, backupBtn, mediaLibraryBtn, bookmarkBtn *widget.Button,
	editPerson func(int64), currentPersonID *int64, getStore func() *store.Store, navigateToPerson func(int64)) {

	// Helper to register both Cmd (Mac) and Ctrl (Win/Linux) shortcuts
	addShortcut := func(key fyne.KeyName, handler func()) {
		if key == fyne.KeyUnknown {
			return // Skip if key is not recognized
		}
		// Cmd+Key for Mac
		cmdShortcut := &desktop.CustomShortcut{KeyName: key, Modifier: fyne.KeyModifierSuper}
		w.Canvas().AddShortcut(cmdShortcut, func(shortcut fyne.Shortcut) { handler() })

		// Ctrl+Key for Windows/Linux
		ctrlShortcut := &desktop.CustomShortcut{KeyName: key, Modifier: fyne.KeyModifierControl}
		w.Canvas().AddShortcut(ctrlShortcut, func(shortcut fyne.Shortcut) { handler() })
	}

	// Add Person
	addShortcut(config.StringToKeyName(cfg.GetShortcut("AddPerson")), func() {
		if addPersonBtn != nil {
			addPersonBtn.OnTapped()
		}
	})

	// Delete Person
	addShortcut(config.StringToKeyName(cfg.GetShortcut("DeletePerson")), func() {
		if deletePersonBtn != nil {
			deletePersonBtn.OnTapped()
		}
	})

	// Focus Search
	addShortcut(config.StringToKeyName(cfg.GetShortcut("FocusSearch")), func() {
		if searchEntry != nil {
			w.Canvas().Focus(searchEntry)
		}
	})

	// Go to Focus Person
	addShortcut(config.StringToKeyName(cfg.GetShortcut("GoToFocusPerson")), func() {
		if focusPersonBtn != nil {
			focusPersonBtn.OnTapped()
		}
	})

	// Edit Person
	addShortcut(config.StringToKeyName(cfg.GetShortcut("EditPerson")), func() {
		if currentPersonID != nil && *currentPersonID > 0 && editPerson != nil {
			editPerson(*currentPersonID)
		}
	})

	// Open Database
	addShortcut(config.StringToKeyName(cfg.GetShortcut("OpenDatabase")), func() {
		if openDatabaseBtn != nil {
			openDatabaseBtn.OnTapped()
		}
	})

	// Backup Database
	addShortcut(config.StringToKeyName(cfg.GetShortcut("BackupDatabase")), func() {
		if backupBtn != nil {
			backupBtn.OnTapped()
		}
	})

	// Media Library
	addShortcut(config.StringToKeyName(cfg.GetShortcut("MediaLibrary")), func() {
		if mediaLibraryBtn != nil {
			mediaLibraryBtn.OnTapped()
		}
	})

	// Toggle Bookmark
	addShortcut(config.StringToKeyName(cfg.GetShortcut("ToggleBookmark")), func() {
		if bookmarkBtn != nil {
			bookmarkBtn.OnTapped()
		}
	})

	// Reports Menu (changed from Data Quality direct shortcut)
	addShortcut(config.StringToKeyName(cfg.GetShortcut("DataQuality")), func() {
		if reportsBtn != nil {
			reportsBtn.OnTapped()
		}
	})

	// Statistics Dashboard
	addShortcut(config.StringToKeyName(cfg.GetShortcut("Statistics")), func() {
		if statsBtn != nil {
			statsBtn.OnTapped()
		}
	})

	// Database Maintenance
	addShortcut(config.StringToKeyName(cfg.GetShortcut("DatabaseMaintenance")), func() {
		showDatabaseMaintenanceDialog(w, getStore())
	})

	// Advanced Search (Shift+F) - needs special handling
	advancedSearchShortcut := cfg.GetShortcut("AdvancedSearch")
	if config.HasShiftModifier(advancedSearchShortcut) {
		key := config.StringToKeyName(advancedSearchShortcut)
		if key != fyne.KeyUnknown {
			// Cmd+Shift+Key for Mac
			cmdShiftShortcut := &desktop.CustomShortcut{
				KeyName:  key,
				Modifier: fyne.KeyModifierSuper | fyne.KeyModifierShift,
			}
			w.Canvas().AddShortcut(cmdShiftShortcut, func(shortcut fyne.Shortcut) {
				showAdvancedSearchDialog(w, getStore(), navigateToPerson)
			})

			// Ctrl+Shift+Key for Windows/Linux
			ctrlShiftShortcut := &desktop.CustomShortcut{
				KeyName:  key,
				Modifier: fyne.KeyModifierControl | fyne.KeyModifierShift,
			}
			w.Canvas().AddShortcut(ctrlShiftShortcut, func(shortcut fyne.Shortcut) {
				showAdvancedSearchDialog(w, getStore(), navigateToPerson)
			})
		}
	}

	// Settings (primary and alternative)
	addShortcut(config.StringToKeyName(cfg.GetShortcut("Settings")), func() {
		if settingsBtn != nil {
			settingsBtn.OnTapped()
		}
	})

	addShortcut(config.StringToKeyName(cfg.GetShortcut("SettingsAlt")), func() {
		if settingsBtn != nil {
			settingsBtn.OnTapped()
		}
	})

	// Keyboard Shortcuts
	addShortcut(config.StringToKeyName(cfg.GetShortcut("KeyboardShortcuts")), func() {
		showKeyboardShortcutsDialog(w, cfg)
	})

	// About
	addShortcut(config.StringToKeyName(cfg.GetShortcut("About")), func() {
		if ShowAboutFunc != nil {
			ShowAboutFunc()
		}
	})

	// Check for Updates
	addShortcut(config.StringToKeyName(cfg.GetShortcut("CheckUpdate")), func() {
		if CheckUpdateFunc != nil {
			CheckUpdateFunc()
		}
	})

	// Help
	addShortcut(config.StringToKeyName(cfg.GetShortcut("Help")), func() {
		if ShowHelpFunc != nil {
			ShowHelpFunc()
		}
	})

	// Quit Application
	addShortcut(config.StringToKeyName(cfg.GetShortcut("Quit")), func() {
		// Show confirmation dialog before quitting
		dialog.ShowConfirm("Quit KrankyBear Genealogy",
			"Are you sure you want to quit? All changes have been saved.",
			func(confirmed bool) {
				if confirmed {
					a.Quit()
				}
			}, w)
	})

	// Tab switching shortcuts
	addShortcut(config.StringToKeyName(cfg.GetShortcut("SwitchToFamily")), func() {
		if tabs != nil && len(tabs.Items) > 0 {
			tabs.Select(tabs.Items[0])
		}
	})

	addShortcut(config.StringToKeyName(cfg.GetShortcut("SwitchToPedigree")), func() {
		if tabs != nil && len(tabs.Items) > 1 {
			tabs.Select(tabs.Items[1])
		}
	})

	addShortcut(config.StringToKeyName(cfg.GetShortcut("SwitchToIndividual")), func() {
		if tabs != nil && len(tabs.Items) > 2 {
			tabs.Select(tabs.Items[2])
		}
	})
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

	preferredNameEntry := widget.NewEntry()
	preferredNameEntry.SetPlaceHolder("Preferred name (optional, e.g. 'Allan' for 'Ivan Allan')")
	preferredNameEntry.SetText(person.PreferredName)

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
			personName := formatPersonName(person)
			showMediaManager(w, s, person.ID, personName)
		})
		mediaSection = container.NewVBox(
			widget.NewSeparator(),
			widget.NewLabel("Media:"),
			mediaBtn,
		)
	}

	// Research To-Do button (only for existing persons)
	var todoSection *fyne.Container
	if isEdit {
		todoBtn := widget.NewButton("📝 Manage Research To-Do", func() {
			personName := formatPersonName(person)
			showTodoManager(w, s, person.ID, personName)
		})
		todoSection = container.NewVBox(
			widget.NewLabel("Research To-Do:"),
			todoBtn,
		)
	}

	formItems := []fyne.CanvasObject{
		widget.NewLabel("Given Name(s):"), givenEntry,
		widget.NewLabel("Surname:"), surnameEntry,
		widget.NewLabel("Preferred Name:"), preferredNameEntry,
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

	// Add todo section if editing
	if todoSection != nil {
		formItems = append(formItems, todoSection.Objects...)
	}

	form := container.NewVBox(formItems...)

	// Validation error label
	validationLabel := widget.NewLabel("")
	validationLabel.Importance = widget.DangerImportance
	validationLabel.Wrapping = fyne.TextWrapWord
	validationLabel.Hide()

	// Insert validation label at the beginning
	formWithValidation := container.NewVBox(validationLabel, form)

	scrollable := container.NewVScroll(formWithValidation)
	scrollable.SetMinSize(fyne.NewSize(500, 600))

	title := "Add Person"
	if isEdit {
		title = "Edit Person"
	}

	// Create custom window instead of dialog
	personWindow := fyne.CurrentApp().NewWindow(title)
	personWindow.Resize(fyne.NewSize(550, 700))

	// Save button handler
	saveBtn := widget.NewButton("Save", func() {
		// Clear any previous validation error
		validationLabel.SetText("")
		validationLabel.Hide()

		// Validate required fields
		if strings.TrimSpace(givenEntry.Text) == "" && strings.TrimSpace(surnameEntry.Text) == "" {
			validationLabel.SetText("❌ Please enter at least a given name or surname")
			validationLabel.Show()
			scrollable.ScrollToTop()
			return
		}

		person.GivenName = strings.TrimSpace(givenEntry.Text)
		person.Surname = strings.TrimSpace(surnameEntry.Text)
		person.PreferredName = strings.TrimSpace(preferredNameEntry.Text)
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
			validationLabel.SetText(fmt.Sprintf("❌ Failed to save: %v", err))
			validationLabel.Show()
			scrollable.ScrollToTop()
			return
		}

		// Close window and refresh
		personWindow.Close()
		if onSave != nil {
			onSave()
		}
	})

	cancelBtn := widget.NewButton("Cancel", func() {
		personWindow.Close()
	})

	buttons := container.NewHBox(saveBtn, cancelBtn)
	content := container.NewBorder(nil, buttons, nil, nil, scrollable)
	personWindow.SetContent(content)
	personWindow.Show()
}

// showDataQualityReport displays a report of records with missing or incomplete data.
func showDataQualityReport(w fyne.Window, s *store.Store, navigateFunc func(int64)) {
	// Check if already open
	if dataQualityDialog != nil {
		dataQualityDialog.Show()
		return
	}

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

	for _, record := range problemRecords {
		p := record.person // Capture for closure

		displayName := fmt.Sprintf("%s %s", p.GivenName, p.Surname)
		if strings.TrimSpace(displayName) == "" {
			displayName = "(no name)"
		}

		issueText := fmt.Sprintf("%s (ID: %d)", displayName, p.ID)
		issueSubtext := fmt.Sprintf("Issues: %s", strings.Join(record.issues, ", "))

		btn := widget.NewButton(issueText, func() {
			if dataQualityDialog != nil {
				dataQualityDialog.Hide()
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

	dataQualityDialog = dialog.NewCustom("Data Quality Report", "Close", scroll, w)
	dataQualityDialog.SetOnClosed(func() {
		dataQualityDialog = nil
	})
	dataQualityDialog.Show()
}

// showStatisticsDashboard displays comprehensive database statistics
func showStatisticsDashboard(w fyne.Window, s *store.Store) {
	// Check if already open
	if statisticsDialog != nil {
		statisticsDialog.Show()
		return
	}

	people, err := s.GetPeople()
	if err != nil {
		dialog.ShowError(err, w)
		return
	}

	// Calculate statistics
	totalPeople := len(people)

	// Count living vs deceased
	living := 0
	deceased := 0
	for _, p := range people {
		if p.IsLiving {
			living++
		} else {
			deceased++
		}
	}

	// Count relationships
	relationshipCount, _ := s.GetRelationshipCount()

	// Count marriages (relationships where type contains "spouse", "partner", "married", etc.)
	marriages := 0
	for _, p := range people {
		spouses, _ := s.GetSpouses(p.ID)
		marriages += len(spouses)
	}
	marriages = marriages / 2 // Each marriage counted twice

	// Get media count
	mediaCount, _ := s.GetMediaCount()

	// Count people with complete data (name, birth date, and either death date or marked living)
	complete := 0
	incomplete := 0
	for _, p := range people {
		hasName := strings.TrimSpace(p.GivenName) != "" && strings.TrimSpace(p.Surname) != ""
		hasBirth := strings.TrimSpace(p.BirthDate) != ""
		hasDeathOrLiving := strings.TrimSpace(p.DeathDate) != "" || p.IsLiving

		if hasName && hasBirth && hasDeathOrLiving {
			complete++
		} else {
			incomplete++
		}
	}

	completionRate := 0.0
	if totalPeople > 0 {
		completionRate = float64(complete) / float64(totalPeople) * 100
	}

	// Find most common surnames
	surnameCount := make(map[string]int)
	for _, p := range people {
		surname := strings.TrimSpace(p.Surname)
		if surname != "" && surname != "Unknown" {
			surnameCount[surname]++
		}
	}

	// Sort surnames by frequency
	type surnameFreq struct {
		name  string
		count int
	}
	var surnames []surnameFreq
	for name, count := range surnameCount {
		surnames = append(surnames, surnameFreq{name, count})
	}
	sort.Slice(surnames, func(i, j int) bool {
		return surnames[i].count > surnames[j].count
	})

	// Get top 10 surnames
	topSurnames := ""
	for i := 0; i < 10 && i < len(surnames); i++ {
		topSurnames += fmt.Sprintf("  %d. %s (%d people)\n", i+1, surnames[i].name, surnames[i].count)
	}
	if topSurnames == "" {
		topSurnames = "  (no surnames recorded)"
	}

	// Find date range (oldest and newest birth)
	var oldestBirth, newestBirth string
	oldestYear := 9999
	newestYear := 0
	for _, p := range people {
		if p.BirthDate != "" {
			year := parseBirthYear(p.BirthDate)
			if year > 0 {
				if year < oldestYear {
					oldestYear = year
					oldestBirth = p.BirthDate
				}
				if year > newestYear {
					newestYear = year
					newestBirth = p.BirthDate
				}
			}
		}
	}

	dateRange := "(no dates recorded)"
	if oldestYear != 9999 && newestYear != 0 {
		dateRange = fmt.Sprintf("%s to %s (%d years)", oldestBirth, newestBirth, newestYear-oldestYear)
	}

	// Calculate generation depth (max distance from oldest to youngest)
	generationDepth := 0
	if oldestYear != 9999 && newestYear != 0 {
		generationDepth = (newestYear - oldestYear) / 25 // Rough estimate: 25 years per generation
	}

	// Build statistics display
	content := container.NewVBox()

	title := widget.NewLabelWithStyle("📊 Database Statistics", fyne.TextAlignCenter, fyne.TextStyle{Bold: true})
	title.TextStyle.Bold = true
	content.Add(title)
	content.Add(widget.NewSeparator())

	// People statistics
	peopleSection := widget.NewLabelWithStyle("👥 People", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	content.Add(peopleSection)
	content.Add(widget.NewLabel(fmt.Sprintf("  Total People: %d", totalPeople)))
	content.Add(widget.NewLabel(fmt.Sprintf("  Living: %d (%.1f%%)", living, float64(living)/float64(totalPeople)*100)))
	content.Add(widget.NewLabel(fmt.Sprintf("  Deceased: %d (%.1f%%)", deceased, float64(deceased)/float64(totalPeople)*100)))
	content.Add(widget.NewLabel(""))

	// Relationship statistics
	relSection := widget.NewLabelWithStyle("🔗 Relationships", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	content.Add(relSection)
	content.Add(widget.NewLabel(fmt.Sprintf("  Total Relationships: %d", relationshipCount)))
	content.Add(widget.NewLabel(fmt.Sprintf("  Marriages/Partnerships: %d", marriages)))
	content.Add(widget.NewLabel(""))

	// Data quality statistics
	qualitySection := widget.NewLabelWithStyle("✓ Data Quality", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	content.Add(qualitySection)
	content.Add(widget.NewLabel(fmt.Sprintf("  Complete Records: %d (%.1f%%)", complete, completionRate)))
	content.Add(widget.NewLabel(fmt.Sprintf("  Incomplete Records: %d (%.1f%%)", incomplete, float64(incomplete)/float64(totalPeople)*100)))
	content.Add(widget.NewLabel(""))

	// Media statistics
	mediaSection := widget.NewLabelWithStyle("📷 Media", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	content.Add(mediaSection)
	content.Add(widget.NewLabel(fmt.Sprintf("  Total Media Files: %d", mediaCount)))
	content.Add(widget.NewLabel(""))

	// Surname statistics
	surnameSection := widget.NewLabelWithStyle("📝 Most Common Surnames", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	content.Add(surnameSection)
	topSurnamesLabel := widget.NewLabel(topSurnames)
	topSurnamesLabel.TextStyle.Monospace = true
	content.Add(topSurnamesLabel)
	content.Add(widget.NewLabel(""))

	// Age & Lifespan statistics
	ageSection := widget.NewLabelWithStyle("👴 Age & Lifespan", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	content.Add(ageSection)

	// Find oldest living person
	var oldestLivingPerson *store.Person
	oldestLivingAge := 0
	for i, p := range people {
		if p.IsLiving && p.BirthDate != "" {
			birthYear := parseBirthYear(p.BirthDate)
			if birthYear > 0 {
				age := time.Now().Year() - birthYear
				if age > oldestLivingAge {
					oldestLivingAge = age
					oldestLivingPerson = &people[i]
				}
			}
		}
	}

	if oldestLivingPerson != nil {
		content.Add(widget.NewLabel(fmt.Sprintf("  Oldest Living: %s (age %d, born %s)",
			formatPersonName(*oldestLivingPerson), oldestLivingAge, oldestLivingPerson.BirthDate)))
	} else {
		content.Add(widget.NewLabel("  Oldest Living: (none recorded)"))
	}

	// Find oldest deceased person (by age at death)
	var oldestDeceasedPerson *store.Person
	oldestDeceasedAge := 0
	totalLifespanYears := 0
	lifespanCount := 0

	for i, p := range people {
		if !p.IsLiving && p.BirthDate != "" && p.DeathDate != "" {
			birthYear := parseBirthYear(p.BirthDate)
			deathYear := parseBirthYear(p.DeathDate)
			if birthYear > 0 && deathYear > 0 {
				ageAtDeath := deathYear - birthYear
				if ageAtDeath > 0 && ageAtDeath < 150 { // Sanity check
					totalLifespanYears += ageAtDeath
					lifespanCount++

					if ageAtDeath > oldestDeceasedAge {
						oldestDeceasedAge = ageAtDeath
						oldestDeceasedPerson = &people[i]
					}
				}
			}
		}
	}

	if oldestDeceasedPerson != nil {
		content.Add(widget.NewLabel(fmt.Sprintf("  Oldest Deceased: %s (age %d, %s - %s)",
			formatPersonName(*oldestDeceasedPerson), oldestDeceasedAge,
			oldestDeceasedPerson.BirthDate, oldestDeceasedPerson.DeathDate)))
	} else {
		content.Add(widget.NewLabel("  Oldest Deceased: (none recorded)"))
	}

	// Calculate average lifespan
	if lifespanCount > 0 {
		avgLifespan := float64(totalLifespanYears) / float64(lifespanCount)
		content.Add(widget.NewLabel(fmt.Sprintf("  Average Lifespan: %.1f years (based on %d deceased with dates)",
			avgLifespan, lifespanCount)))
	} else {
		content.Add(widget.NewLabel("  Average Lifespan: (insufficient data)"))
	}
	content.Add(widget.NewLabel(""))

	// Timeline statistics
	timelineSection := widget.NewLabelWithStyle("📅 Timeline", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	content.Add(timelineSection)
	content.Add(widget.NewLabel(fmt.Sprintf("  Date Range: %s", dateRange)))
	content.Add(widget.NewLabel(fmt.Sprintf("  Estimated Generations: ~%d", generationDepth)))

	scroll := container.NewScroll(content)
	scroll.SetMinSize(fyne.NewSize(600, 500))

	statisticsDialog = dialog.NewCustom("Statistics Dashboard", "Close", scroll, w)
	statisticsDialog.SetOnClosed(func() {
		statisticsDialog = nil
	})
	statisticsDialog.Show()
}

// showLivingStatusReport displays a report analyzing ages and living status
func showLivingStatusReport(w fyne.Window, s *store.Store, navigateFunc func(int64)) {
	// Check if already open
	if livingStatusDialog != nil {
		livingStatusDialog.Show()
		return
	}

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

	for _, rec := range issues {
		p := rec.person // Capture for closure

		displayName := fmt.Sprintf("%s %s", p.GivenName, p.Surname)
		if strings.TrimSpace(displayName) == "" {
			displayName = "(no name)"
		}

		issueText := fmt.Sprintf("%s (ID: %d)", displayName, p.ID)

		btn := widget.NewButton(issueText, func() {
			if livingStatusDialog != nil {
				livingStatusDialog.Hide()
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

	livingStatusDialog = dialog.NewCustom("Living Status Report", "Close", scroll, w)
	livingStatusDialog.SetOnClosed(func() {
		livingStatusDialog = nil
	})
	livingStatusDialog.Show()
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

// showKeyboardShortcutsDialog displays a dialog for customizing keyboard shortcuts
func showKeyboardShortcutsDialog(w fyne.Window, cfg *config.Config) {
	content := container.NewVBox()

	title := widget.NewLabelWithStyle("Customize Keyboard Shortcuts", fyne.TextAlignCenter, fyne.TextStyle{Bold: true})
	content.Add(title)
	content.Add(widget.NewLabel("Click on a shortcut key to change it. Use Cmd (Mac) or Ctrl (Windows/Linux) + key."))
	content.Add(widget.NewSeparator())

	// Map of action name -> display name
	shortcuts := []struct {
		Action  string
		Display string
	}{
		{"AddPerson", "Add Person"},
		{"DeletePerson", "Delete Person"},
		{"EditPerson", "Edit Person"},
		{"FocusSearch", "Focus Search Box"},
		{"GoToFocusPerson", "Go to Focus Person"},
		{"OpenDatabase", "Open Database"},
		{"BackupDatabase", "Backup Database"},
		{"MediaLibrary", "Media Library"},
		{"Settings", "Settings (Primary)"},
		{"SettingsAlt", "Settings (Alternative)"},
		{"KeyboardShortcuts", "Keyboard Shortcuts"},
		{"About", "About"},
		{"CheckUpdate", "Check for Updates"},
		{"Help", "Help"},
		{"Quit", "Quit Application"},
		{"Statistics", "Statistics Dashboard"},
		{"DataQuality", "Data Quality Report"},
		{"SwitchToFamily", "Switch to Family View"},
		{"SwitchToPedigree", "Switch to Pedigree View"},
		{"SwitchToIndividual", "Switch to Individual View"},
	}

	// Track if any changes were made
	changesMap := make(map[string]string) // action -> new key

	// Create maps for shortcuts
	actionToDisplay := make(map[string]string) // action -> display name
	for _, sc := range shortcuts {
		actionToDisplay[sc.Action] = sc.Display
	}

	for _, sc := range shortcuts {
		// Capture loop variables properly
		currentAction := sc.Action
		currentDisplayName := sc.Display

		currentKey := cfg.GetShortcut(currentAction)
		if currentKey == "" {
			currentKey = config.DefaultKeyboardShortcuts()[currentAction]
		}

		label := widget.NewLabel(currentDisplayName + ":")

		// Use a regular label instead of disabled entry for better readability
		keyLabel := widget.NewLabel("Cmd/Ctrl+" + currentKey)
		keyLabel.TextStyle = fyne.TextStyle{Bold: true}

		// Change button to edit this shortcut
		editBtn := widget.NewButton("Change...", func() {
			// Capture for this closure
			action := currentAction
			displayName := currentDisplayName

			currentKeyVal := cfg.GetShortcut(action)
			if currentKeyVal == "" {
				currentKeyVal = config.DefaultKeyboardShortcuts()[action]
			}

			// Show dialog to enter new key
			showEditShortcutDialog(w, cfg, action, displayName, currentKeyVal, func(newKey string) bool {
				// Check for conflicts
				conflictAction := ""
				conflictDisplay := ""

				for otherAction, otherDisplay := range actionToDisplay {
					if otherAction == action {
						continue
					}
					otherKey := cfg.GetShortcut(otherAction)
					if otherKey == "" {
						otherKey = config.DefaultKeyboardShortcuts()[otherAction]
					}
					// Check pending changes too
					if pendingKey, ok := changesMap[otherAction]; ok {
						otherKey = pendingKey
					}

					if otherKey == newKey {
						conflictAction = otherAction
						conflictDisplay = otherDisplay
						break
					}
				}

				if conflictAction != "" {
					dialog.ShowError(fmt.Errorf("Key '%s' is already assigned to:\n\n%s\n\nPlease choose a different key.", newKey, conflictDisplay), w)
					return false // Don't close dialog, there was a conflict
				}

				// Update the display
				keyLabel.SetText("Cmd/Ctrl+" + newKey)
				changesMap[action] = newKey
				return true // Success, close dialog
			})
		})

		row := container.NewBorder(nil, nil, label, editBtn, keyLabel)
		content.Add(row)
	}

	content.Add(widget.NewSeparator())

	// Save and Reset buttons
	saveBtn := widget.NewButton("Save Changes", func() {
		if len(changesMap) == 0 {
			dialog.ShowInformation("No Changes", "No shortcuts have been changed.", w)
			return
		}

		// Apply all changes
		for action, newKey := range changesMap {
			cfg.SetShortcut(action, newKey)
		}

		if err := cfg.Save(); err != nil {
			dialog.ShowError(fmt.Errorf("Failed to save configuration: %w", err), w)
			return
		}

		dialog.ShowInformation("Success",
			fmt.Sprintf("Saved %d shortcut change(s).\n\nRestart the application for changes to take effect.", len(changesMap)), w)

		// Clear changes map
		changesMap = make(map[string]string)
	})

	resetBtn := widget.NewButton("Reset to Defaults", func() {
		dialog.ShowConfirm("Reset Shortcuts",
			"Are you sure you want to reset all keyboard shortcuts to their default values?",
			func(confirmed bool) {
				if confirmed {
					cfg.ResetShortcutsToDefaults()
					if err := cfg.Save(); err != nil {
						dialog.ShowError(fmt.Errorf("Failed to save configuration: %w", err), w)
						return
					}

					dialog.ShowInformation("Success",
						"Keyboard shortcuts have been reset to defaults.\n\nRestart the application for changes to take effect.", w)
				}
			}, w)
	})

	buttonBox := container.NewHBox(saveBtn, resetBtn)
	content.Add(buttonBox)

	scroll := container.NewScroll(content)
	scroll.SetMinSize(fyne.NewSize(550, 600))

	d := dialog.NewCustom("Keyboard Shortcuts", "Close", scroll, w)
	d.Show()
}

// showEditShortcutDialog shows a dialog to edit a single keyboard shortcut
func showEditShortcutDialog(parent fyne.Window, cfg *config.Config, action, displayName, currentKey string, onSave func(string) bool) {
	content := container.NewVBox()

	content.Add(widget.NewLabel(fmt.Sprintf("Change shortcut for: %s", displayName)))
	content.Add(widget.NewLabel(fmt.Sprintf("Current: Cmd/Ctrl+%s", currentKey)))
	content.Add(widget.NewSeparator())

	content.Add(widget.NewLabel("Enter the new key (letter, number, or symbol):"))
	content.Add(widget.NewLabel("Letters/Numbers: A-Z, 0-9"))
	content.Add(widget.NewLabel("Symbols: , . / ; - ="))

	keyEntry := widget.NewEntry()
	keyEntry.SetPlaceHolder("Type: N, T, 5, or , . / etc.")
	content.Add(keyEntry)

	var d dialog.Dialog

	saveBtn := widget.NewButton("Save", func() {
		newKey := strings.ToUpper(strings.TrimSpace(keyEntry.Text))
		if newKey == "" {
			dialog.ShowError(fmt.Errorf("Please enter a key"), parent)
			return
		}

		// Convert symbols to their word equivalents
		symbolMap := map[string]string{
			",":  "Comma",
			".":  "Period",
			"/":  "Slash",
			"\\": "Backslash",
			";":  "Semicolon",
			"-":  "Minus",
			"=":  "Equal",
			"!":  "1", // Shift+1
			"@":  "2", // Shift+2
			"#":  "3", // Shift+3
			"$":  "4", // Shift+4
			"%":  "5", // Shift+5
			"^":  "6", // Shift+6
			"&":  "7", // Shift+7
			"*":  "8", // Shift+8
			"(":  "9", // Shift+9
			")":  "0", // Shift+0
		}

		if wordKey, ok := symbolMap[newKey]; ok {
			newKey = wordKey
		}

		// Validate the key
		if config.StringToKeyName(newKey) == fyne.KeyUnknown {
			dialog.ShowError(fmt.Errorf("Invalid key: %s\n\nPlease use:\n- Letters: A-Z\n- Numbers: 0-9\n- Special: , (comma)  . (period)  / (slash)  ; (semicolon)  - (minus)  = (equal)", newKey), parent)
			return
		}

		// Call onSave and only hide if successful (no conflict)
		if onSave(newKey) {
			d.Hide()
		}
	})

	cancelBtn := widget.NewButton("Cancel", func() {
		d.Hide()
	})

	// Enter key to save
	keyEntry.OnSubmitted = func(s string) {
		saveBtn.OnTapped()
	}

	buttons := container.NewHBox(saveBtn, cancelBtn)
	content.Add(buttons)

	d = dialog.NewCustom("Edit Shortcut", "", content, parent)
	d.Resize(fyne.NewSize(400, 250))
	d.Show()

	// Focus the entry
	parent.Canvas().Focus(keyEntry)
}

// showSettingsDialog displays application settings including Focus User preference
func showSettingsDialog(w fyne.Window, cfg *config.Config, dbPath string, s *store.Store, onSave func()) {
	// Check if already open
	if settingsDialog != nil {
		settingsDialog.Show()
		return
	}

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
	settingsDialog = dialog.NewCustom("Settings", "Close", scroll, w)
	settingsDialog.SetOnClosed(func() {
		onSave()
		settingsDialog = nil
	})
	settingsDialog.Show()
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

	// Check if toID is a cousin of my parent (making them my cousin once removed)
	myParentsAgain, _ := s.GetRelatedPeople(fromID, "parent")
	for _, parent := range myParentsAgain {
		parentToTargetRel := getCousinInfo(s, parent.ID, toID)
		if parentToTargetRel != "" {
			// My parent's 1st cousin is my 1st cousin once removed
			// My parent's 2nd cousin is my 2nd cousin once removed, etc.
			if parentToTargetRel == "1st cousin" {
				return "1st cousin once removed"
			} else if parentToTargetRel == "2nd cousin" {
				return "2nd cousin once removed"
			} else if parentToTargetRel == "3rd cousin" {
				return "3rd cousin once removed"
			} else if parentToTargetRel == "1st cousin once removed" {
				return "1st cousin twice removed"
			} else if parentToTargetRel == "2nd cousin once removed" {
				return "2nd cousin twice removed"
			}
		}

		// Check if toID is spouse of parent's cousin
		parentCousins := getCousinsList(s, parent.ID)
		for _, cousin := range parentCousins {
			cousinSpouses, _ := s.GetSpouses(cousin.ID)
			for _, sp := range cousinSpouses {
				if sp.Person.ID == toID {
					cousinName := fmt.Sprintf("%s %s", cousin.GivenName, cousin.Surname)
					parentName := fmt.Sprintf("%s %s", parent.GivenName, parent.Surname)
					parentToCousinRel := getCousinInfo(s, parent.ID, cousin.ID)
					if sp.Person.Gender == "M" {
						return fmt.Sprintf("husband of %s, a %s of %s", cousinName, parentToCousinRel, parentName)
					}
					return fmt.Sprintf("wife of %s, a %s of %s", cousinName, parentToCousinRel, parentName)
				}
			}
		}
	}

	// Check if toID is a cousin of my child (making them my child's cousin once removed)
	myChildrenForCousins, _ := s.GetRelatedPeople(fromID, "child")
	for _, child := range myChildrenForCousins {
		childToTargetRel := getCousinInfo(s, child.ID, toID)
		if childToTargetRel != "" {
			// My child's 1st cousin is my nephew/niece's child OR my 1st cousin's child
			// This is the reverse check
			if childToTargetRel == "1st cousin" {
				return "1st cousin once removed"
			} else if childToTargetRel == "2nd cousin" {
				return "2nd cousin once removed"
			} else if childToTargetRel == "1st cousin once removed" {
				return "1st cousin twice removed"
			}
		}
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
							cousinName := fmt.Sprintf("%s %s", cousin.GivenName, cousin.Surname)
							// Get the fromID person's name for context
							fromPerson, _ := s.GetPersonByID(fromID)
							fromName := "you"
							if fromPerson != nil {
								fromName = fmt.Sprintf("%s %s", fromPerson.GivenName, fromPerson.Surname)
							}
							if cousinSp.Person.Gender == "M" {
								return fmt.Sprintf("husband of %s, a 1st cousin of %s", cousinName, fromName)
							}
							return fmt.Sprintf("wife of %s, a 1st cousin of %s", cousinName, fromName)
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

		// Check for second cousins (children of parent's first cousins)
		// Get great-grandparents
		for _, grandparent := range grandparents {
			greatGrandparents, _ := s.GetRelatedPeople(grandparent.ID, "parent")

			for _, greatGrandparent := range greatGrandparents {
				// Get all children of great-grandparent
				grandAuntsUncles, _ := s.GetRelatedPeople(greatGrandparent.ID, "child")

				for _, grandAuntUncle := range grandAuntsUncles {
					if grandAuntUncle.ID == grandparent.ID {
						continue // Skip my grandparent
					}

					// Get their children (parent's first cousins)
					parentsFirstCousins, _ := s.GetRelatedPeople(grandAuntUncle.ID, "child")

					for _, parentsFirstCousin := range parentsFirstCousins {
						// Get their children (my second cousins)
						secondCousins, _ := s.GetRelatedPeople(parentsFirstCousin.ID, "child")

						for _, secondCousin := range secondCousins {
							if secondCousin.ID == toID {
								return "2nd cousin"
							}

							// Check for 2nd cousin once removed (their children)
							secondCousinChildren, _ := s.GetRelatedPeople(secondCousin.ID, "child")
							for _, secondCousinChild := range secondCousinChildren {
								if secondCousinChild.ID == toID {
									return "2nd cousin once removed"
								}

								// Check for 2nd cousin twice removed
								secondCousinGrandchildren, _ := s.GetRelatedPeople(secondCousinChild.ID, "child")
								for _, secondCousinGrandchild := range secondCousinGrandchildren {
									if secondCousinGrandchild.ID == toID {
										return "2nd cousin twice removed"
									}
								}
							}
						}
					}
				}
			}
		}
	}

	return ""
}

// getCousinsList returns all cousins of a person (for spouse checking)
func getCousinsList(s *store.Store, personID int64) []store.Person {
	var cousins []store.Person
	visited := make(map[int64]bool)

	parents, _ := s.GetRelatedPeople(personID, "parent")
	for _, parent := range parents {
		grandparents, _ := s.GetRelatedPeople(parent.ID, "parent")
		for _, grandparent := range grandparents {
			auntsUncles, _ := s.GetRelatedPeople(grandparent.ID, "child")
			for _, auntUncle := range auntsUncles {
				if auntUncle.ID == parent.ID {
					continue
				}
				firstCousins, _ := s.GetRelatedPeople(auntUncle.ID, "child")
				for _, cousin := range firstCousins {
					if !visited[cousin.ID] {
						cousins = append(cousins, cousin)
						visited[cousin.ID] = true
					}
				}
			}
		}
	}

	return cousins
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
// showImportOptionsDialog displays a dialog to choose import destination
func showImportOptionsDialog(w fyne.Window, cfg *config.Config, dbPath string, getStore func() *store.Store,
	reloadWithDatabase func(string, ...bool), refreshAll func(), fileExt string, formatName string,
	importFunc func(string, *store.Store) error) {

	importDestination := "current" // "current" or "new"

	radio := widget.NewRadioGroup([]string{
		"Import into current database",
		"Create new database and import",
	}, func(selected string) {
		if selected == "Import into current database" {
			importDestination = "current"
		} else {
			importDestination = "new"
		}
	})
	radio.Selected = "Import into current database"
	radio.Horizontal = false

	content := container.NewVBox(
		widget.NewLabel(fmt.Sprintf("Where would you like to import this %s file?", formatName)),
		widget.NewSeparator(),
		radio,
	)

	dialog.ShowCustomConfirm("Import Options", "Continue", "Cancel", content, func(proceed bool) {
		if !proceed {
			return
		}

		if importDestination == "new" {
			// First, prompt for the file to import
			importFd := dialog.NewFileOpen(func(r fyne.URIReadCloser, importErr error) {
				if importErr != nil || r == nil {
					return
				}
				importPath := r.URI().Path()
				r.Close()

				// Suggest database name based on import file name
				importFileName := filepath.Base(importPath)
				suggestedDBName := strings.TrimSuffix(importFileName, filepath.Ext(importFileName)) + ".db"

				// Now prompt for new database location
				dbFd := dialog.NewFileSave(func(uc fyne.URIWriteCloser, err error) {
					if err != nil || uc == nil {
						return
					}
					newDBPath := uc.URI().Path()
					uc.Close()

					// Ensure .db extension
					if !strings.HasSuffix(newDBPath, ".db") {
						newDBPath += ".db"
					}

					// Create new database
					newStore, err := store.Open(newDBPath)
					if err != nil {
						dialog.ShowError(fmt.Errorf("Failed to create new database: %w", err), w)
						return
					}

					// Initialize schema
					if err := newStore.InitSchema(); err != nil {
						newStore.Close()
						dialog.ShowError(fmt.Errorf("Failed to initialize database: %w", err), w)
						return
					}

					// Run migrations
					if err := newStore.MigrateSchema(); err != nil {
						newStore.Close()
						dialog.ShowError(fmt.Errorf("Failed to migrate database: %w", err), w)
						return
					}

					// Import into the NEW database (before switching to it)
					if err := importFunc(importPath, newStore); err != nil {
						newStore.Close()
						dialog.ShowError(fmt.Errorf("%s import failed: %w", formatName, err), w)
						return
					}

					// Close the new store before reloading
					newStore.Close()

					// NOW switch to the new database (which now has imported data)
					// Pass true to suppress the "Database Loaded" dialog
					reloadWithDatabase(newDBPath, true)

					// Show import success message
					dialog.ShowInformation("Import Complete",
						fmt.Sprintf("✅ New database created and %s file imported successfully!\n\nDatabase: %s\n\nRecords imported - no sample data needed.", formatName, filepath.Base(newDBPath)), w)
				}, w)

				dbFd.SetFileName(suggestedDBName)
				dbFd.Show()
			}, w)
			importFd.SetFilter(storageFilter{ext: fileExt})
			importFd.Show()
		} else {
			// Import into current database
			fd := dialog.NewFileOpen(func(r fyne.URIReadCloser, err error) {
				if err != nil || r == nil {
					return
				}
				importPath := r.URI().Path()
				r.Close()

				// Show progress for Gramps (can be slow)
				if formatName == "Gramps" {
					dialog.ShowInformation("Importing", "Importing from Gramps database...\nThis may take a moment.", w)
				}

				if err := importFunc(importPath, getStore()); err != nil {
					dialog.ShowError(fmt.Errorf("%s import failed: %w", formatName, err), w)
					return
				}
				dialog.ShowInformation("Import Complete", fmt.Sprintf("%s file imported successfully!", formatName), w)
				refreshAll()
			}, w)
			fd.SetFilter(storageFilter{ext: fileExt})
			fd.Show()
		}
	}, w)
}

// showLoadDemoDialog loads the demo database for new users to explore
func showLoadDemoDialog(w fyne.Window, cfg *config.Config, reloadWithDatabase func(string, ...bool)) {
	// Show information about what the demo contains
	content := widget.NewLabel(
		"Generate a demo database?\n\n" +
			"The demo database contains:\n" +
			"  • 50+ people across 5-6 generations\n" +
			"  • Multiple marriages & divorces\n" +
			"  • Living people with contact info\n" +
			"  • Preferred names and nicknames\n" +
			"  • International locations (USA + Latvia)\n" +
			"  • Example data quality issues\n\n" +
			"📍 Focus Person: Michael Harrison\n" +
			"   Browse his marriages and explore his\n" +
			"   children's families for surprises!\n\n" +
			"This will help you explore all the features\n" +
			"of KrankyBear Genealogy!\n\n" +
			"Click OK to choose where to save the demo database.")
	content.Wrapping = fyne.TextWrapWord

	dialog.ShowCustomConfirm("Load Demo Database", "OK", "Cancel", content, func(proceed bool) {
		if !proceed {
			return
		}

		// Show save file dialog
		saveDialog := dialog.NewFileSave(func(writer fyne.URIWriteCloser, err error) {
			if err != nil || writer == nil {
				return
			}
			defer writer.Close()

			demoDestPath := writer.URI().Path()

			// Remove old demo if it exists
			os.Remove(demoDestPath)

			// Create new demo database
			demoStore, err := store.Open(demoDestPath)
			if err != nil {
				dialog.ShowError(fmt.Errorf("Failed to create demo database: %w", err), w)
				return
			}

			// Initialize schema
			if err := demoStore.InitSchema(); err != nil {
				demoStore.Close()
				dialog.ShowError(fmt.Errorf("Failed to initialize demo database: %w", err), w)
				return
			}

			// Run migrations (adds marriage-related columns to relationships table)
			if err := demoStore.MigrateSchema(); err != nil {
				demoStore.Close()
				dialog.ShowError(fmt.Errorf("Failed to migrate demo database: %w", err), w)
				return
			}

			// Generate demo data
			focusPersonID, err := demo.GenerateDemoData(demoStore)
			if err != nil {
				demoStore.Close()
				dialog.ShowError(fmt.Errorf("Failed to generate demo data: %w", err), w)
				return
			}

			// Close the demo database
			demoStore.Close()

			// Set Michael Harrison as the focus person for this database
			cfg.SetFocusUserForDatabase(demoDestPath, focusPersonID)
			cfg.OpenWithFocusUser = true
			cfg.Save()

			// Switch to the demo database (silently, we'll show our own message)
			reloadWithDatabase(demoDestPath, true)

			// Show welcome message
			welcomeMsg := fmt.Sprintf(
				"✅ Demo database generated!\n\n"+
					"📍 Location: %s\n\n"+
					"👋 Welcome! This database contains 50+ people across\n"+
					"   5-6 generations with international locations.\n\n"+
					"👤 Focus Person: Michael Harrison\n"+
					"   (Notice he has 2 marriages!)\n"+
					"   Explore his children's spouses for interesting discoveries...\n\n"+
					"📷 Note: Media files are not included in this generated demo.\n"+
					"   To get the full demo with photos & documents, download\n"+
					"   demo.db from the GitHub repository.\n\n"+
					"🎯 Try these features:\n"+
					"  • Browse the family tree in all 3 views\n"+
					"  • Try Reports → Data Quality Report\n"+
					"  • Check Reports → Conflicts Report\n"+
					"  • Use Reports → Statistics Dashboard\n"+
					"  • Press G to return to Michael Harrison\n"+
					"  • Use Add Media to attach your own photos\n\n"+
					"💡 This demo showcases all the features you can use\n"+
					"for your own family history!",
				demoDestPath)

			dialog.ShowInformation("Demo Loaded", welcomeMsg, w)
		}, w)

		// Set suggested filename
		saveDialog.SetFileName("demo.db")

		// Set filter to show .db files
		saveDialog.SetFilter(storage.NewExtensionFileFilter([]string{".db"}))

		saveDialog.Show()
	}, w)
}

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
func showBackupDialog(w fyne.Window, dbPath string, getStore func() *store.Store) {
	s := getStore()

	// Calculate database-stored media size
	dbStoredCount, totalDBMediaSize := calculateDatabaseMediaSize(s)

	// Calculate external media count and size
	externalCount, totalExternalSize, externalPaths := calculateExternalMediaSize(s)

	// Format sizes for display
	dbMediaSizeStr := formatFileSize(totalDBMediaSize)
	externalMediaSizeStr := formatFileSize(totalExternalSize)

	// Generate backup filename with timestamp
	baseName := strings.TrimSuffix(filepath.Base(dbPath), filepath.Ext(dbPath))
	timestamp := time.Now().Format("01-02-2006") // MM-DD-YYYY
	defaultFileName := fmt.Sprintf("%s-%s.zip", baseName, timestamp)

	// Create checkboxes for backup options
	includeDBMedia := false
	includeExternalMedia := false

	// Database-stored media checkbox
	dbMediaCheck := widget.NewCheck(
		fmt.Sprintf("Extract Database-Stored Media (separate ZIP, %d files, ~%s)", dbStoredCount, dbMediaSizeStr),
		func(checked bool) {
			includeDBMedia = checked
		})

	// External media checkbox
	externalMediaCheck := widget.NewCheck(
		fmt.Sprintf("Include External Media Files (separate ZIP, %d files, ~%s)", externalCount, externalMediaSizeStr),
		func(checked bool) {
			includeExternalMedia = checked
		})

	// Warning labels
	warningLabel := widget.NewLabel("")
	warningLabel.Wrapping = fyne.TextWrapWord

	if totalDBMediaSize > 100*1024*1024 { // > 100MB
		warningLabel.SetText("⚠️  Database media is large. Extraction may take time.")
		warningLabel.Importance = widget.WarningImportance
	}

	externalWarningLabel := widget.NewLabel("")
	externalWarningLabel.Wrapping = fyne.TextWrapWord

	if totalExternalSize > 500*1024*1024 { // > 500MB
		externalWarningLabel.SetText("⚠️  External media is very large. Backup will take significant time.")
		externalWarningLabel.Importance = widget.WarningImportance
	} else if externalCount > 0 {
		externalWarningLabel.SetText("External files will be copied from their current locations. Missing files will be skipped with warnings.")
	}

	infoLabel := widget.NewLabel("Database backup includes all data (including stored media BLOBs).\nOptional: Extract media to separate ZIPs for easier access.")
	infoLabel.Wrapping = fyne.TextWrapWord

	content := container.NewVBox(
		infoLabel,
		widget.NewSeparator(),
		dbMediaCheck,
		warningLabel,
		widget.NewSeparator(),
		externalMediaCheck,
		externalWarningLabel,
	)

	dialog.ShowCustomConfirm("Backup Database", "Backup", "Cancel", content, func(ok bool) {
		if !ok {
			return
		}

		fd := dialog.NewFileSave(func(uc fyne.URIWriteCloser, err error) {
			if err != nil || uc == nil {
				return
			}
			backupPath := uc.URI().Path()
			uc.Close()

			// Backup database
			if err := backupDatabase(dbPath, backupPath); err != nil {
				dialog.ShowError(fmt.Errorf("Failed to backup database: %w", err), w)
				return
			}

			resultMsg := fmt.Sprintf("✅ Database backed up to:\n%s", backupPath)

			// Backup database-stored media if requested
			if includeDBMedia && dbStoredCount > 0 {
				dbMediaBackupPath := strings.TrimSuffix(backupPath, filepath.Ext(backupPath)) + "-media-database.zip"
				if err := backupDatabaseMedia(s, dbMediaBackupPath); err != nil {
					dialog.ShowError(fmt.Errorf("Database backup succeeded, but media extraction failed: %w", err), w)
					return
				}
				resultMsg += fmt.Sprintf("\n\n✅ Database media extracted to:\n%s", dbMediaBackupPath)
			}

			// Backup external media if requested
			if includeExternalMedia && externalCount > 0 {
				externalMediaBackupPath := strings.TrimSuffix(backupPath, filepath.Ext(backupPath)) + "-media-external.zip"
				skipped, copyErr := backupExternalMedia(s, externalMediaBackupPath, externalPaths)
				if copyErr != nil {
					dialog.ShowError(fmt.Errorf("Database backup succeeded, but external media backup failed: %w", copyErr), w)
					return
				}
				if skipped > 0 {
					resultMsg += fmt.Sprintf("\n\n✅ External media backed up to:\n%s\n⚠️  %d files were missing/skipped", externalMediaBackupPath, skipped)
				} else {
					resultMsg += fmt.Sprintf("\n\n✅ External media backed up to:\n%s", externalMediaBackupPath)
				}
			}

			dialog.ShowInformation("Backup Complete", resultMsg, w)
		}, w)

		fd.SetFileName(defaultFileName)
		fd.Show()
	}, w)
}

// backupDatabase creates a zip backup of the database file
func backupDatabase(dbPath, backupPath string) error {
	// Create zip file
	zipFile, err := os.Create(backupPath)
	if err != nil {
		return fmt.Errorf("failed to create backup file: %w", err)
	}
	defer zipFile.Close()

	zipWriter := zip.NewWriter(zipFile)
	defer zipWriter.Close()

	// Open the database file
	dbFile, err := os.Open(dbPath)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer dbFile.Close()

	// Get file info
	dbFileInfo, err := dbFile.Stat()
	if err != nil {
		return fmt.Errorf("failed to get database info: %w", err)
	}

	// Create zip entry header
	header, err := zip.FileInfoHeader(dbFileInfo)
	if err != nil {
		return fmt.Errorf("failed to create zip header: %w", err)
	}
	header.Name = filepath.Base(dbPath)
	header.Method = zip.Deflate

	// Create writer for this file in the zip
	writer, err := zipWriter.CreateHeader(header)
	if err != nil {
		return fmt.Errorf("failed to create zip entry: %w", err)
	}

	// Copy database to zip
	_, err = io.Copy(writer, dbFile)
	if err != nil {
		return fmt.Errorf("failed to write database to backup: %w", err)
	}

	return nil
}

// backupDatabaseMedia creates a separate zip backup of all media files stored in the database as BLOBs
func backupDatabaseMedia(s *store.Store, mediaBackupPath string) error {
	// Query to get database-stored media with full_image BLOB data
	rows, err := s.DB.Query(`
		SELECT id, media_type, mime_type, full_image
		FROM media
		WHERE is_external = 0 AND full_image IS NOT NULL AND LENGTH(full_image) > 0
		ORDER BY id
	`)
	if err != nil {
		return fmt.Errorf("failed to query media: %w", err)
	}
	defer rows.Close()

	// Create media zip file
	zipFile, err := os.Create(mediaBackupPath)
	if err != nil {
		return fmt.Errorf("failed to create media backup file: %w", err)
	}
	defer zipFile.Close()

	zipWriter := zip.NewWriter(zipFile)
	defer zipWriter.Close()

	// Process each media file
	for rows.Next() {
		var mediaID int64
		var mediaType, mimeType string
		var fullImage []byte

		if err := rows.Scan(&mediaID, &mediaType, &mimeType, &fullImage); err != nil {
			return fmt.Errorf("failed to scan media row: %w", err)
		}

		// Create a safe filename - use MimeType to determine extension
		ext := ""
		switch {
		case strings.Contains(mimeType, "jpeg") || strings.Contains(mimeType, "jpg"):
			ext = ".jpg"
		case strings.Contains(mimeType, "png"):
			ext = ".png"
		case strings.Contains(mimeType, "gif"):
			ext = ".gif"
		case strings.Contains(mimeType, "pdf"):
			ext = ".pdf"
		case strings.Contains(mimeType, "video"):
			ext = ".mp4"
		case strings.Contains(mimeType, "word"):
			ext = ".docx"
		case strings.Contains(mimeType, "excel") || strings.Contains(mimeType, "spreadsheet"):
			ext = ".xlsx"
		case strings.Contains(mimeType, "powerpoint") || strings.Contains(mimeType, "presentation"):
			ext = ".pptx"
		default:
			// Fallback to media type
			switch mediaType {
			case "image":
				ext = ".jpg"
			case "pdf":
				ext = ".pdf"
			case "video":
				ext = ".mp4"
			case "document":
				ext = ".doc"
			default:
				ext = ".dat"
			}
		}
		safeFileName := fmt.Sprintf("media_%d%s", mediaID, ext)

		// Create zip entry
		header := &zip.FileHeader{
			Name:   safeFileName,
			Method: zip.Deflate,
		}

		writer, err := zipWriter.CreateHeader(header)
		if err != nil {
			return fmt.Errorf("failed to create zip entry for %s: %w", safeFileName, err)
		}

		// Write media data to zip
		_, err = writer.Write(fullImage)
		if err != nil {
			return fmt.Errorf("failed to write media %s to backup: %w", safeFileName, err)
		}
	}

	if err := rows.Err(); err != nil {
		return fmt.Errorf("error iterating media rows: %w", err)
	}

	return nil
}

// backupExternalMedia creates a separate zip backup of all external media files
// Returns the number of files skipped (missing/inaccessible) and any fatal error
func backupExternalMedia(s *store.Store, mediaBackupPath string, externalPaths []string) (skippedCount int, err error) {
	// Create media zip file
	zipFile, err := os.Create(mediaBackupPath)
	if err != nil {
		return 0, fmt.Errorf("failed to create external media backup file: %w", err)
	}
	defer zipFile.Close()

	zipWriter := zip.NewWriter(zipFile)
	defer zipWriter.Close()

	skippedCount = 0

	// Query external media with their IDs and paths
	rows, queryErr := s.DB.Query(`
		SELECT id, external_path, media_type, mime_type
		FROM media
		WHERE is_external = 1 AND external_path IS NOT NULL AND external_path != ''
		ORDER BY id
	`)
	if queryErr != nil {
		return 0, fmt.Errorf("failed to query external media: %w", queryErr)
	}
	defer rows.Close()

	for rows.Next() {
		var mediaID int64
		var externalPath, mediaType, mimeType string

		if err := rows.Scan(&mediaID, &externalPath, &mediaType, &mimeType); err != nil {
			return skippedCount, fmt.Errorf("failed to scan external media row: %w", err)
		}

		// Check if file exists
		fileInfo, statErr := os.Stat(externalPath)
		if statErr != nil {
			// File doesn't exist or is inaccessible - skip with warning
			skippedCount++
			continue
		}

		// Read the external file
		fileData, readErr := os.ReadFile(externalPath)
		if readErr != nil {
			// Can't read file - skip with warning
			skippedCount++
			continue
		}

		// Create a safe filename preserving original extension
		originalFileName := filepath.Base(externalPath)
		ext := filepath.Ext(originalFileName)
		if ext == "" {
			// Determine extension from mime type if not in filename
			switch {
			case strings.Contains(mimeType, "jpeg") || strings.Contains(mimeType, "jpg"):
				ext = ".jpg"
			case strings.Contains(mimeType, "png"):
				ext = ".png"
			case strings.Contains(mimeType, "gif"):
				ext = ".gif"
			case strings.Contains(mimeType, "pdf"):
				ext = ".pdf"
			case strings.Contains(mimeType, "video"):
				ext = ".mp4"
			case strings.Contains(mimeType, "word"):
				ext = ".docx"
			case strings.Contains(mimeType, "excel") || strings.Contains(mimeType, "spreadsheet"):
				ext = ".xlsx"
			case strings.Contains(mimeType, "powerpoint") || strings.Contains(mimeType, "presentation"):
				ext = ".pptx"
			default:
				ext = ".dat"
			}
		}

		safeFileName := fmt.Sprintf("external_%d_%s%s", mediaID, strings.TrimSuffix(originalFileName, ext), ext)

		// Create zip entry with file modification time
		header := &zip.FileHeader{
			Name:     safeFileName,
			Method:   zip.Deflate,
			Modified: fileInfo.ModTime(),
		}

		writer, createErr := zipWriter.CreateHeader(header)
		if createErr != nil {
			return skippedCount, fmt.Errorf("failed to create zip entry for %s: %w", safeFileName, createErr)
		}

		// Write file data to zip
		_, writeErr := writer.Write(fileData)
		if writeErr != nil {
			return skippedCount, fmt.Errorf("failed to write external media %s to backup: %w", safeFileName, writeErr)
		}
	}

	if err := rows.Err(); err != nil {
		return skippedCount, fmt.Errorf("error iterating external media rows: %w", err)
	}

	return skippedCount, nil
}

// calculateDatabaseMediaSize calculates the total size and count of database-stored media
// without loading all the BLOB data into memory
func calculateDatabaseMediaSize(s *store.Store) (count int, totalSize int64) {
	// Query to count and sum the size of database-stored media (not external)
	// We use LENGTH(full_image) to get the size of the BLOB without loading it
	row := s.DB.QueryRow(`
		SELECT COUNT(*), COALESCE(SUM(LENGTH(full_image)), 0)
		FROM media
		WHERE is_external = 0 AND full_image IS NOT NULL AND LENGTH(full_image) > 0
	`)

	err := row.Scan(&count, &totalSize)
	if err != nil {
		// If error, return 0,0 - backup will still work, just won't show accurate size
		return 0, 0
	}

	return count, totalSize
}

// calculateExternalMediaSize calculates the total size and count of external media files
func calculateExternalMediaSize(s *store.Store) (count int, totalSize int64, paths []string) {
	// Query external media paths
	rows, err := s.DB.Query(`
		SELECT external_path
		FROM media
		WHERE is_external = 1 AND external_path IS NOT NULL AND external_path != ''
		ORDER BY id
	`)
	if err != nil {
		return 0, 0, nil
	}
	defer rows.Close()

	paths = []string{}
	for rows.Next() {
		var path string
		if err := rows.Scan(&path); err != nil {
			continue
		}

		// Check if file exists and get size
		if info, err := os.Stat(path); err == nil {
			totalSize += info.Size()
			count++
			paths = append(paths, path)
		}
		// If file doesn't exist, we don't count it (will be skipped with warning during backup)
	}

	return count, totalSize, paths
}

// formatFileSize formats bytes into human-readable sizes
func formatFileSize(bytes int64) string {
	const (
		KB = 1024
		MB = 1024 * KB
		GB = 1024 * MB
	)

	switch {
	case bytes >= GB:
		return fmt.Sprintf("%.2f GB", float64(bytes)/float64(GB))
	case bytes >= MB:
		return fmt.Sprintf("%.2f MB", float64(bytes)/float64(MB))
	case bytes >= KB:
		return fmt.Sprintf("%.2f KB", float64(bytes)/float64(KB))
	default:
		return fmt.Sprintf("%d bytes", bytes)
	}
}

// showRestoreDialog restores a database from a zip backup
func showRestoreDialog(w fyne.Window, currentDBPath string, reloadFunc func(string, ...bool)) {
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

// formatPersonName formats a person's full name.
func formatPersonName(p store.Person) string {
	// If preferred name is set, show it in parentheses: "Ivan (Allan) Marillier"
	// Otherwise just show: "Ivan Marillier"
	if p.PreferredName != "" {
		name := fmt.Sprintf("%s (%s) %s", p.GivenName, p.PreferredName, p.Surname)
		return strings.TrimSpace(name)
	}
	name := fmt.Sprintf("%s %s", p.GivenName, p.Surname)
	return strings.TrimSpace(name)
}

// Conflict types
type Conflict struct {
	PersonID    int64
	PersonName  string
	Type        string
	Description string
	Severity    string // "Critical", "Warning", "Info"
}

var conflictsDialog fyne.Window

func showConflictsReport(w fyne.Window, s *store.Store, navigateFunc func(int64)) {
	// Check if already open
	if conflictsDialog != nil {
		conflictsDialog.RequestFocus()
		conflictsDialog.Show()
		return
	}

	conflicts := detectConflicts(s)

	// Filter checkbox state
	showReviewed := false

	// Function to rebuild the content
	var content *fyne.Container
	var scroll *container.Scroll
	var rebuildContent func() // Declare first to allow recursive reference

	rebuildContent = func() {
		// Filter conflicts based on review status
		filteredConflicts := []Conflict{}
		reviewedCount := 0

		for _, c := range conflicts {
			isReviewed, _, _ := s.IsItemReviewed("conflict", c.PersonID, nil, c.Type)
			if isReviewed {
				reviewedCount++
				if showReviewed {
					filteredConflicts = append(filteredConflicts, c)
				}
			} else {
				filteredConflicts = append(filteredConflicts, c)
			}
		}

		// Group by severity
		critical := []Conflict{}
		warnings := []Conflict{}

		for _, c := range filteredConflicts {
			if c.Severity == "Critical" {
				critical = append(critical, c)
			} else {
				warnings = append(warnings, c)
			}
		}

		// Build content
		content.Objects = nil // Clear existing content

		if len(conflicts) == 0 {
			content.Add(widget.NewLabel("✅ No data conflicts detected!"))
			content.Add(widget.NewLabel("Your genealogy data is consistent."))
		} else {
			// Summary
			summary := widget.NewLabelWithStyle(
				fmt.Sprintf("Found %d conflicts (%d critical, %d warnings, %d reviewed)",
					len(conflicts), len(critical)+len(warnings), len(filteredConflicts)-len(critical), reviewedCount),
				fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
			content.Add(summary)
			content.Add(widget.NewSeparator())

			if len(filteredConflicts) == 0 {
				content.Add(widget.NewLabel("All conflicts have been reviewed."))
				content.Add(widget.NewLabel("Check 'Show Reviewed Items' to see them."))
			} else {
				// Critical issues
				if len(critical) > 0 {
					criticalLabel := widget.NewLabelWithStyle("🚨 Critical Issues:",
						fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
					content.Add(criticalLabel)

					for _, c := range critical {
						addConflictItem(content, s, conflictsDialog, c, navigateFunc, rebuildContent)
					}
				}

				// Warnings
				if len(warnings) > 0 {
					warningLabel := widget.NewLabelWithStyle("⚠️  Warnings:",
						fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
					content.Add(warningLabel)

					for _, c := range warnings {
						addConflictItem(content, s, conflictsDialog, c, navigateFunc, rebuildContent)
					}
				}
			}
		}

		content.Refresh()
		if scroll != nil {
			scroll.Refresh()
		}
	}

	// Initialize content container
	content = container.NewVBox()

	// Create filter checkbox
	showReviewedCheck := widget.NewCheck("Show Reviewed Items", func(checked bool) {
		showReviewed = checked
		rebuildContent()
	})

	// Initial build
	rebuildContent()

	scroll = container.NewVScroll(content)
	scroll.SetMinSize(fyne.NewSize(600, 400))

	mainContent := container.NewBorder(
		container.NewVBox(showReviewedCheck, widget.NewSeparator()),
		nil, nil, nil,
		scroll,
	)

	conflictsDialog = fyne.CurrentApp().NewWindow("Conflicts Report")
	conflictsDialog.SetContent(mainContent)
	conflictsDialog.Resize(fyne.NewSize(750, 550))
	conflictsDialog.SetOnClosed(func() {
		conflictsDialog = nil
	})
	conflictsDialog.Show()
}

func addConflictItem(content *fyne.Container, s *store.Store, reportWindow fyne.Window, c Conflict, navigateFunc func(int64), refreshFunc func()) {
	conflictCopy := c

	// Check if reviewed
	isReviewed, validatedItem, _ := s.IsItemReviewed("conflict", c.PersonID, nil, c.Type)

	personBtn := widget.NewButton(c.PersonName, func() {
		navigateFunc(conflictCopy.PersonID)
	})

	typeLabel := widget.NewLabel(fmt.Sprintf("  %s: %s", c.Type, c.Description))

	if isReviewed && validatedItem != nil {
		// Show review info with visual distinction
		reviewLabel := widget.NewLabelWithStyle("✅ REVIEWED",
			fyne.TextAlignLeading, fyne.TextStyle{Bold: true})

		reviewInfo := widget.NewLabel(fmt.Sprintf("  Reviewed by %s: %s",
			validatedItem.ReviewedBy, validatedItem.ValidationNote))
		reviewInfo.TextStyle = fyne.TextStyle{Italic: true}

		unmarkBtn := widget.NewButton("Remove Review Status", func() {
			dialog.ShowConfirm("Remove Review Status",
				"Are you sure you want to remove the review marking? This conflict will appear in reports again.",
				func(confirmed bool) {
					if confirmed {
						if err := s.UnmarkAsReviewed("conflict", conflictCopy.PersonID, nil, conflictCopy.Type); err != nil {
							dialog.ShowError(fmt.Errorf("Failed to unmark: %w", err), reportWindow)
							return
						}
						dialog.ShowInformation("Review Removed", "The review status has been removed.", reportWindow)
						refreshFunc()
					}
				}, reportWindow)
		})

		// Create a visually distinct container with background color
		itemBox := container.NewVBox(
			reviewLabel,
			personBtn,
			typeLabel,
			reviewInfo,
			unmarkBtn,
		)

		content.Add(itemBox)
	} else {
		// Show Mark as Reviewed button
		reviewBtn := widget.NewButton("✓ Mark as Reviewed", func() {
			showMarkAsReviewedDialog(reportWindow, s, "conflict", conflictCopy.PersonID, nil, conflictCopy.Type, refreshFunc)
		})

		itemBox := container.NewVBox(
			personBtn,
			typeLabel,
			reviewBtn,
		)
		content.Add(itemBox)
	}

	content.Add(widget.NewSeparator())
}

func showMarkAsReviewedDialog(w fyne.Window, s *store.Store, itemType string, personID int64, relatedPersonID *int64, conflictType string, refreshFunc func()) {
	// Prevent multiple dialogs from opening
	if markAsReviewedDialogOpen {
		return
	}
	markAsReviewedDialogOpen = true

	nameEntry := widget.NewEntry()
	nameEntry.SetPlaceHolder("Your name")

	noteEntry := widget.NewMultiLineEntry()
	noteEntry.SetPlaceHolder("Notes/explanation (e.g., 'Verified from parish records', 'Mother was indeed 14 years old')")
	noteEntry.SetMinRowsVisible(3)

	form := container.NewVBox(
		widget.NewLabel("Mark this item as reviewed:"),
		widget.NewSeparator(),
		widget.NewLabel("Reviewed by:"),
		nameEntry,
		widget.NewLabel("Notes:"),
		noteEntry,
	)

	dialog.ShowCustomConfirm("Mark as Reviewed", "Save", "Cancel", form, func(confirmed bool) {
		markAsReviewedDialogOpen = false // Reset flag when dialog closes

		if confirmed {
			reviewedBy := strings.TrimSpace(nameEntry.Text)
			note := strings.TrimSpace(noteEntry.Text)

			if reviewedBy == "" {
				dialog.ShowError(fmt.Errorf("Please enter your name"), w)
				return
			}

			if err := s.MarkAsReviewed(itemType, personID, relatedPersonID, conflictType, note, reviewedBy); err != nil {
				dialog.ShowError(fmt.Errorf("Failed to mark as reviewed: %w", err), w)
				return
			}

			dialog.ShowInformation("Marked as Reviewed", "This item has been marked as reviewed and will be hidden by default in future reports.", w)
			refreshFunc()
		}
	}, w)
}

func detectConflicts(s *store.Store) []Conflict {
	conflicts := []Conflict{}

	people, err := s.GetPeople()
	if err != nil {
		return conflicts
	}

	for _, person := range people {
		// Death before birth
		if person.BirthDate != "" && person.DeathDate != "" {
			birthTime, bErr := parseDate(person.BirthDate)
			deathTime, dErr := parseDate(person.DeathDate)
			if bErr == nil && dErr == nil && deathTime.Before(birthTime) {
				conflicts = append(conflicts, Conflict{
					PersonID:    person.ID,
					PersonName:  formatPersonName(person),
					Type:        "Death Before Birth",
					Description: fmt.Sprintf("Died %s before birth %s", person.DeathDate, person.BirthDate),
					Severity:    "Critical",
				})
			}
		}

		// Check relationships
		rels, err := s.GetRelationshipsForPerson(person.ID)
		if err != nil {
			continue
		}

		for _, rel := range rels {
			// Only check relationships where person is the subject
			if rel.SubjectID != person.ID {
				continue
			}

			// Marriage before birth
			if rel.Type == "spouse" && rel.MarriageDate != "" && person.BirthDate != "" {
				marriageTime, mErr := parseDate(rel.MarriageDate)
				birthTime, bErr := parseDate(person.BirthDate)
				if mErr == nil && bErr == nil && marriageTime.Before(birthTime) {
					conflicts = append(conflicts, Conflict{
						PersonID:    person.ID,
						PersonName:  formatPersonName(person),
						Type:        "Married Before Birth",
						Description: fmt.Sprintf("Married %s before birth %s", rel.MarriageDate, person.BirthDate),
						Severity:    "Critical",
					})
				}

				// Married too young (< 13 years old)
				if mErr == nil && bErr == nil {
					age := int(marriageTime.Sub(birthTime).Hours() / 24 / 365.25)
					if age < 13 {
						conflicts = append(conflicts, Conflict{
							PersonID:    person.ID,
							PersonName:  formatPersonName(person),
							Type:        "Married Too Young",
							Description: fmt.Sprintf("Married at age %d (less than 13)", age),
							Severity:    "Warning",
						})
					}
				}
			}

			// Parent-child relationships
			// Pattern: Type="child", SubjectID=parent, ObjectID=child (same as GetRelatedPeople)
			if rel.Type != "child" || rel.SubjectID != person.ID {
				continue
			}

			child, err := s.GetPersonByID(rel.ObjectID)
			if err != nil || child == nil {
				continue
			}

			// Now we have the child, check for conflicts
			{

				// Child born before parent
				if person.BirthDate != "" && child.BirthDate != "" {
					parentBirth, pErr := parseDate(person.BirthDate)
					childBirth, cErr := parseDate(child.BirthDate)
					if pErr == nil && cErr == nil && childBirth.Before(parentBirth) {
						conflicts = append(conflicts, Conflict{
							PersonID:   person.ID,
							PersonName: formatPersonName(person),
							Type:       "Child Born Before Parent",
							Description: fmt.Sprintf("Child %s born %s before parent birth %s",
								formatPersonName(*child), child.BirthDate, person.BirthDate),
							Severity: "Critical",
						})
					}

					// Parent too young (< 13)
					if pErr == nil && cErr == nil {
						parentAge := int(childBirth.Sub(parentBirth).Hours() / 24 / 365.25)
						if parentAge < 13 {
							conflicts = append(conflicts, Conflict{
								PersonID:   person.ID,
								PersonName: formatPersonName(person),
								Type:       "Parent Too Young",
								Description: fmt.Sprintf("Had child %s at age %d (less than 13)",
									formatPersonName(*child), parentAge),
								Severity: "Warning",
							})
						}

						// Parent too old (women > 60, men > 80)
						if (person.Gender == "Female" || person.Gender == "F") && parentAge > 60 {
							conflicts = append(conflicts, Conflict{
								PersonID:   person.ID,
								PersonName: formatPersonName(person),
								Type:       "Parent Too Old",
								Description: fmt.Sprintf("Had child %s at age %d (female > 60)",
									formatPersonName(*child), parentAge),
								Severity: "Warning",
							})
						} else if (person.Gender == "Male" || person.Gender == "M") && parentAge > 80 {
							conflicts = append(conflicts, Conflict{
								PersonID:   person.ID,
								PersonName: formatPersonName(person),
								Type:       "Parent Too Old",
								Description: fmt.Sprintf("Had child %s at age %d (male > 80)",
									formatPersonName(*child), parentAge),
								Severity: "Warning",
							})
						}
					}
				}

				// Child born after parent died
				if person.DeathDate != "" && child.BirthDate != "" {
					parentDeath, pErr := parseDate(person.DeathDate)
					childBirth, cErr := parseDate(child.BirthDate)
					if pErr == nil && cErr == nil {
						// Allow 9 months posthumous birth for fathers
						maxPosthumous := time.Duration(0)
						if person.Gender == "Male" || person.Gender == "M" {
							maxPosthumous = 280 * 24 * time.Hour // ~9 months
						}

						if childBirth.After(parentDeath.Add(maxPosthumous)) {
							conflicts = append(conflicts, Conflict{
								PersonID:   person.ID,
								PersonName: formatPersonName(person),
								Type:       "Child Born After Parent Death",
								Description: fmt.Sprintf("Child %s born %s after parent died %s",
									formatPersonName(*child), child.BirthDate, person.DeathDate),
								Severity: "Warning",
							})
						}
					}
				}
			} // end of parent-child relationship checks block
		}
	}

	return conflicts
}

func parseDate(dateStr string) (time.Time, error) {
	formats := []string{
		"2006-01-02",
		"2006-01",
		"2006",
		"02 Jan 2006",
		"January 2006",
		"Jan 2006",
	}

	for _, format := range formats {
		if t, err := time.Parse(format, dateStr); err == nil {
			return t, nil
		}
	}

	return time.Time{}, fmt.Errorf("unable to parse date: %s", dateStr)
}

// Duplicate pair
type DuplicatePair struct {
	Person1      store.Person
	Person2      store.Person
	NameScore    float64 // 0-100
	DateScore    float64 // 0-100
	OverallScore float64 // 0-100
}

var duplicatesDialog fyne.Window

func showDuplicateDetectionReport(w fyne.Window, s *store.Store, navigateFunc func(int64), refreshFunc func()) {
	// Check if already open
	if duplicatesDialog != nil {
		duplicatesDialog.RequestFocus()
		duplicatesDialog.Show()
		return
	}

	duplicates := detectDuplicates(s)

	// Filter checkbox state
	showReviewed := false

	// Function to rebuild the content
	var content *fyne.Container
	var scroll *container.Scroll
	var rebuildContent func() // Declare first to allow recursive reference

	rebuildContent = func() {
		// Filter duplicates based on review status
		filteredDuplicates := []DuplicatePair{}
		reviewedCount := 0

		for _, dup := range duplicates {
			relatedID := dup.Person2.ID
			isReviewed, _, _ := s.IsItemReviewed("duplicate", dup.Person1.ID, &relatedID, "Duplicate")
			if isReviewed {
				reviewedCount++
				if showReviewed {
					filteredDuplicates = append(filteredDuplicates, dup)
				}
			} else {
				filteredDuplicates = append(filteredDuplicates, dup)
			}
		}

		// Build content
		content.Objects = nil // Clear existing content

		if len(duplicates) == 0 {
			content.Add(widget.NewLabel("✅ No potential duplicates detected!"))
			content.Add(widget.NewLabel("Your database appears to have no duplicate records."))
		} else {
			// Summary
			summary := widget.NewLabelWithStyle(
				fmt.Sprintf("Found %d potential duplicate pairs (%d reviewed)",
					len(duplicates), reviewedCount),
				fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
			content.Add(summary)
			content.Add(widget.NewLabel("Click 'Compare' to view details and merge if needed."))
			content.Add(widget.NewSeparator())

			if len(filteredDuplicates) == 0 {
				content.Add(widget.NewLabel("All duplicates have been reviewed."))
				content.Add(widget.NewLabel("Check 'Show Reviewed Items' to see them."))
			} else {
				// Sort by overall score (highest first)
				sort.Slice(filteredDuplicates, func(i, j int) bool {
					return filteredDuplicates[i].OverallScore > filteredDuplicates[j].OverallScore
				})

				// Show each duplicate pair
				for i, dup := range filteredDuplicates {
					dupCopy := dup // Capture for closure
					relatedID := dup.Person2.ID

					scoreLabel := widget.NewLabelWithStyle(
						fmt.Sprintf("Match Score: %.0f%%", dup.OverallScore),
						fyne.TextAlignLeading, fyne.TextStyle{Bold: true})

					person1Label := widget.NewLabel(fmt.Sprintf("  1. %s (ID: %d) - Born: %s",
						formatPersonName(dup.Person1), dup.Person1.ID, dup.Person1.BirthDate))
					person2Label := widget.NewLabel(fmt.Sprintf("  2. %s (ID: %d) - Born: %s",
						formatPersonName(dup.Person2), dup.Person2.ID, dup.Person2.BirthDate))

					// Check if reviewed
					isReviewed, validatedItem, _ := s.IsItemReviewed("duplicate", dup.Person1.ID, &relatedID, "Duplicate")

					var buttonBox *fyne.Container
					if isReviewed && validatedItem != nil {
						// Show review info with visual distinction
						reviewLabel := widget.NewLabelWithStyle("✅ REVIEWED - Not a Duplicate",
							fyne.TextAlignLeading, fyne.TextStyle{Bold: true})

						reviewInfo := widget.NewLabel(fmt.Sprintf("  Reviewed by %s: %s",
							validatedItem.ReviewedBy, validatedItem.ValidationNote))
						reviewInfo.TextStyle = fyne.TextStyle{Italic: true}

						viewBtn := widget.NewButton("View Details", func() {
							showDuplicateComparisonDialog(w, s, dupCopy, navigateFunc, refreshFunc, func() {
								duplicatesDialog.Close()
								showDuplicateDetectionReport(w, s, navigateFunc, refreshFunc)
							})
						})

						unmarkBtn := widget.NewButton("Remove Review Status", func() {
							dialog.ShowConfirm("Remove Review Status",
								"Are you sure you want to remove the review marking? This duplicate pair will appear in reports again.",
								func(confirmed bool) {
									if confirmed {
										if err := s.UnmarkAsReviewed("duplicate", dupCopy.Person1.ID, &relatedID, "Duplicate"); err != nil {
											dialog.ShowError(fmt.Errorf("Failed to unmark: %w", err), duplicatesDialog)
											return
										}
										dialog.ShowInformation("Review Removed", "The review status has been removed.", duplicatesDialog)
										rebuildContent()
									}
								}, duplicatesDialog)
						})

						buttonBox = container.NewVBox(
							reviewLabel,
							reviewInfo,
							container.NewHBox(viewBtn, unmarkBtn),
						)
					} else {
						compareBtn := widget.NewButton("Compare & Merge", func() {
							showDuplicateComparisonDialog(w, s, dupCopy, navigateFunc, refreshFunc, func() {
								duplicatesDialog.Close()
								showDuplicateDetectionReport(w, s, navigateFunc, refreshFunc)
							})
						})

						view1Btn := widget.NewButton("View #1", func() {
							navigateFunc(dupCopy.Person1.ID)
						})
						view2Btn := widget.NewButton("View #2", func() {
							navigateFunc(dupCopy.Person2.ID)
						})

						reviewBtn := widget.NewButton("✓ Not a Duplicate", func() {
							showMarkAsReviewedDialog(duplicatesDialog, s, "duplicate", dupCopy.Person1.ID, &relatedID, "Duplicate", rebuildContent)
						})

						buttonBox = container.NewVBox(
							container.NewHBox(compareBtn, view1Btn, view2Btn),
							reviewBtn,
						)
					}

					pairBox := container.NewVBox(
						scoreLabel,
						person1Label,
						person2Label,
						buttonBox,
					)

					content.Add(pairBox)
					if i < len(filteredDuplicates)-1 {
						content.Add(widget.NewSeparator())
					}
				}
			}
		}

		content.Refresh()
		if scroll != nil {
			scroll.Refresh()
		}
	}

	// Initialize content container
	content = container.NewVBox()

	// Create filter checkbox
	showReviewedCheck := widget.NewCheck("Show Reviewed Items", func(checked bool) {
		showReviewed = checked
		rebuildContent()
	})

	// Initial build
	rebuildContent()

	scroll = container.NewVScroll(content)
	scroll.SetMinSize(fyne.NewSize(700, 400))

	mainContent := container.NewBorder(
		container.NewVBox(showReviewedCheck, widget.NewSeparator()),
		nil, nil, nil,
		scroll,
	)

	duplicatesDialog = fyne.CurrentApp().NewWindow("Duplicate Detection")
	duplicatesDialog.SetContent(mainContent)
	duplicatesDialog.Resize(fyne.NewSize(850, 650))
	duplicatesDialog.SetOnClosed(func() {
		duplicatesDialog = nil
	})
	duplicatesDialog.Show()
}

func detectDuplicates(s *store.Store) []DuplicatePair {
	duplicates := []DuplicatePair{}

	people, err := s.GetPeople()
	if err != nil {
		return duplicates
	}

	// Compare each pair
	for i := 0; i < len(people); i++ {
		for j := i + 1; j < len(people); j++ {
			p1 := people[i]
			p2 := people[j]

			nameScore := calculateNameSimilarity(p1, p2)
			dateScore := calculateDateSimilarity(p1, p2)

			// Overall score: weighted average (name 60%, date 40%)
			overallScore := (nameScore * 0.6) + (dateScore * 0.4)

			// Only report if score > 70%
			if overallScore > 70 {
				duplicates = append(duplicates, DuplicatePair{
					Person1:      p1,
					Person2:      p2,
					NameScore:    nameScore,
					DateScore:    dateScore,
					OverallScore: overallScore,
				})
			}
		}
	}

	return duplicates
}

func calculateNameSimilarity(p1, p2 store.Person) float64 {
	// Normalize names (lowercase, trim)
	name1 := strings.ToLower(strings.TrimSpace(p1.GivenName + " " + p1.Surname))
	name2 := strings.ToLower(strings.TrimSpace(p2.GivenName + " " + p2.Surname))

	// Exact match
	if name1 == name2 {
		return 100.0
	}

	// Levenshtein distance-based similarity
	distance := levenshteinDistance(name1, name2)
	maxLen := float64(max(len(name1), len(name2)))
	if maxLen == 0 {
		return 0
	}

	similarity := (1.0 - float64(distance)/maxLen) * 100.0
	return math.Max(0, similarity)
}

func calculateDateSimilarity(p1, p2 store.Person) float64 {
	// If both have no birth date, score 50 (neutral)
	if p1.BirthDate == "" && p2.BirthDate == "" {
		return 50.0
	}

	// If one has birth date and other doesn't, score 20 (low)
	if p1.BirthDate == "" || p2.BirthDate == "" {
		return 20.0
	}

	// Parse dates
	t1, err1 := parseDate(p1.BirthDate)
	t2, err2 := parseDate(p2.BirthDate)

	if err1 != nil || err2 != nil {
		return 20.0
	}

	// Exact match
	if t1.Equal(t2) {
		return 100.0
	}

	// Calculate day difference
	daysDiff := math.Abs(t1.Sub(t2).Hours() / 24)

	// Score based on proximity
	if daysDiff < 1 {
		return 100.0
	} else if daysDiff < 7 {
		return 90.0
	} else if daysDiff < 30 {
		return 80.0
	} else if daysDiff < 365 {
		return 70.0
	} else if daysDiff < 365*2 {
		return 50.0
	} else if daysDiff < 365*5 {
		return 30.0
	} else {
		return 10.0
	}
}

func levenshteinDistance(s1, s2 string) int {
	if len(s1) == 0 {
		return len(s2)
	}
	if len(s2) == 0 {
		return len(s1)
	}

	// Create matrix
	matrix := make([][]int, len(s1)+1)
	for i := range matrix {
		matrix[i] = make([]int, len(s2)+1)
		matrix[i][0] = i
	}
	for j := 0; j <= len(s2); j++ {
		matrix[0][j] = j
	}

	// Fill matrix
	for i := 1; i <= len(s1); i++ {
		for j := 1; j <= len(s2); j++ {
			cost := 1
			if s1[i-1] == s2[j-1] {
				cost = 0
			}
			matrix[i][j] = min(
				matrix[i-1][j]+1,      // deletion
				matrix[i][j-1]+1,      // insertion
				matrix[i-1][j-1]+cost, // substitution
			)
		}
	}

	return matrix[len(s1)][len(s2)]
}

func showDuplicateComparisonDialog(w fyne.Window, s *store.Store, dup DuplicatePair,
	navigateFunc func(int64), refreshFunc func(), closeParentFunc func()) {

	p1 := dup.Person1
	p2 := dup.Person2

	title := widget.NewLabelWithStyle("Compare Potential Duplicates",
		fyne.TextAlignCenter, fyne.TextStyle{Bold: true})

	// Person 1 details
	p1Box := container.NewVBox(
		widget.NewLabelWithStyle(fmt.Sprintf("Person #1 (ID: %d)", p1.ID),
			fyne.TextAlignCenter, fyne.TextStyle{Bold: true}),
		widget.NewLabel(fmt.Sprintf("Name: %s %s", p1.GivenName, p1.Surname)),
		widget.NewLabel(fmt.Sprintf("Birth: %s %s", p1.BirthDate, p1.BirthPlace)),
		widget.NewLabel(fmt.Sprintf("Death: %s %s", p1.DeathDate, p1.DeathPlace)),
		widget.NewLabel(fmt.Sprintf("Gender: %s", p1.Gender)),
	)

	// Person 2 details
	p2Box := container.NewVBox(
		widget.NewLabelWithStyle(fmt.Sprintf("Person #2 (ID: %d)", p2.ID),
			fyne.TextAlignCenter, fyne.TextStyle{Bold: true}),
		widget.NewLabel(fmt.Sprintf("Name: %s %s", p2.GivenName, p2.Surname)),
		widget.NewLabel(fmt.Sprintf("Birth: %s %s", p2.BirthDate, p2.BirthPlace)),
		widget.NewLabel(fmt.Sprintf("Death: %s %s", p2.DeathDate, p2.DeathPlace)),
		widget.NewLabel(fmt.Sprintf("Gender: %s", p2.Gender)),
	)

	comparison := container.NewHBox(
		container.NewVBox(p1Box),
		widget.NewSeparator(),
		container.NewVBox(p2Box),
	)

	// Scores
	scoresBox := container.NewVBox(
		widget.NewLabel(fmt.Sprintf("Name Similarity: %.0f%%", dup.NameScore)),
		widget.NewLabel(fmt.Sprintf("Date Similarity: %.0f%%", dup.DateScore)),
		widget.NewLabelWithStyle(fmt.Sprintf("Overall Match: %.0f%%", dup.OverallScore),
			fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
	)

	// Action buttons
	mergeP1Btn := widget.NewButton("Keep #1, Delete #2", func() {
		dialog.ShowConfirm("Merge Records",
			fmt.Sprintf("This will:\n1. Keep %s (ID: %d)\n2. Transfer all relationships from #2 to #1\n3. Delete %s (ID: %d)\n\nContinue?",
				formatPersonName(p1), p1.ID, formatPersonName(p2), p2.ID),
			func(confirmed bool) {
				if confirmed {
					if err := mergePersons(s, p1.ID, p2.ID); err != nil {
						dialog.ShowError(fmt.Errorf("Merge failed: %w", err), w)
					} else {
						dialog.ShowInformation("Merge Complete",
							fmt.Sprintf("Successfully merged records. Kept %s (ID: %d)",
								formatPersonName(p1), p1.ID), w)
						refreshFunc()
						closeParentFunc()
					}
				}
			}, w)
	})

	mergeP2Btn := widget.NewButton("Keep #2, Delete #1", func() {
		dialog.ShowConfirm("Merge Records",
			fmt.Sprintf("This will:\n1. Keep %s (ID: %d)\n2. Transfer all relationships from #1 to #2\n3. Delete %s (ID: %d)\n\nContinue?",
				formatPersonName(p2), p2.ID, formatPersonName(p1), p1.ID),
			func(confirmed bool) {
				if confirmed {
					if err := mergePersons(s, p2.ID, p1.ID); err != nil {
						dialog.ShowError(fmt.Errorf("Merge failed: %w", err), w)
					} else {
						dialog.ShowInformation("Merge Complete",
							fmt.Sprintf("Successfully merged records. Kept %s (ID: %d)",
								formatPersonName(p2), p2.ID), w)
						refreshFunc()
						closeParentFunc()
					}
				}
			}, w)
	})

	notDuplicateBtn := widget.NewButton("Not Duplicates", func() {
		// Just close the dialog
	})

	buttons := container.NewHBox(mergeP1Btn, mergeP2Btn, notDuplicateBtn)

	content := container.NewVBox(
		title,
		widget.NewSeparator(),
		comparison,
		widget.NewSeparator(),
		scoresBox,
		widget.NewSeparator(),
		buttons,
	)

	dialog.ShowCustom("Compare Duplicates", "Close", content, w)
}

func mergePersons(s *store.Store, keepID, deleteID int64) error {
	// 1. Get all relationships for the person to be deleted
	rels, err := s.GetRelationshipsForPerson(deleteID)
	if err != nil {
		return fmt.Errorf("failed to get relationships: %w", err)
	}

	// 2. Transfer relationships to the kept person
	for _, rel := range rels {
		// Create new relationship with keepID
		var newSubjectID, newObjectID int64
		if rel.SubjectID == deleteID {
			newSubjectID = keepID
			newObjectID = rel.ObjectID
		} else {
			newSubjectID = rel.SubjectID
			newObjectID = keepID
		}

		// Check if relationship already exists to avoid duplicates
		existingRels, _ := s.GetRelationshipsForPerson(keepID)
		exists := false
		for _, existing := range existingRels {
			if (existing.SubjectID == newSubjectID && existing.ObjectID == newObjectID && existing.Type == rel.Type) ||
				(existing.SubjectID == newObjectID && existing.ObjectID == newSubjectID && existing.Type == rel.Type) {
				exists = true
				break
			}
		}

		if !exists {
			newRel := &store.Relationship{
				SubjectID:      newSubjectID,
				ObjectID:       newObjectID,
				Type:           rel.Type,
				MarriageDate:   rel.MarriageDate,
				MarriagePlace:  rel.MarriagePlace,
				DivorceDate:    rel.DivorceDate,
				SeparationDate: rel.SeparationDate,
				EndReason:      rel.EndReason,
			}
			if err := s.CreateRelationship(newRel); err != nil {
				return fmt.Errorf("failed to transfer relationship: %w", err)
			}
		}

		// Delete old relationship
		if err := s.DeleteRelationship(rel.SubjectID, rel.ObjectID, rel.Type); err != nil {
			// Ignore errors for already deleted relationships
			continue
		}
	}

	// 3. Transfer media links
	media, err := s.GetMediaForPerson(deleteID)
	if err == nil {
		for _, m := range media {
			// Link to keepID
			_ = s.LinkMediaToPerson(m.ID, keepID)
			// Unlink from deleteID
			_ = s.UnlinkMediaFromPerson(m.ID, deleteID)
		}
	}

	// 4. Delete the duplicate person
	if err := s.DeletePerson(deleteID); err != nil {
		return fmt.Errorf("failed to delete person: %w", err)
	}

	return nil
}

// Descendant Report
var descendantDialog fyne.Window

func showDescendantReport(w fyne.Window, s *store.Store, rootPersonID int64, navigateFunc func(int64)) {
	// Check if already open
	if descendantDialog != nil {
		descendantDialog.RequestFocus()
		descendantDialog.Show()
		return
	}

	if rootPersonID <= 0 {
		dialog.ShowInformation("Descendant (Pedigree) Report", "Please select a person first", w)
		return
	}

	rootPerson, err := s.GetPersonByID(rootPersonID)
	if err != nil {
		dialog.ShowError(fmt.Errorf("Failed to load person: %w", err), w)
		return
	}

	// Build descendant tree
	descendants := collectDescendants(s, rootPersonID, 0)

	// Build content
	content := container.NewVBox()

	title := widget.NewLabelWithStyle(
		fmt.Sprintf("Descendants of %s", formatPersonName(*rootPerson)),
		fyne.TextAlignCenter, fyne.TextStyle{Bold: true})
	content.Add(title)

	if len(descendants) == 0 {
		content.Add(widget.NewLabel("No descendants found."))
	} else {
		// Summary
		summary := widget.NewLabel(fmt.Sprintf("Total descendants: %d", len(descendants)))
		content.Add(summary)

		// Count by generation
		genCounts := make(map[int]int)
		for _, d := range descendants {
			genCounts[d.Generation]++
		}
		maxGen := 0
		for gen := range genCounts {
			if gen > maxGen {
				maxGen = gen
			}
		}
		genSummary := "Generations: "
		for i := 1; i <= maxGen; i++ {
			if i > 1 {
				genSummary += ", "
			}
			genSummary += fmt.Sprintf("Gen %d: %d", i, genCounts[i])
		}
		content.Add(widget.NewLabel(genSummary))
		content.Add(widget.NewSeparator())

		// Display descendants by generation
		for gen := 1; gen <= maxGen; gen++ {
			genLabel := widget.NewLabelWithStyle(
				fmt.Sprintf("Generation %d:", gen),
				fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
			content.Add(genLabel)

			for _, d := range descendants {
				if d.Generation == gen {
					dCopy := d
					indent := strings.Repeat("  ", gen-1)
					personBtn := widget.NewButton(
						fmt.Sprintf("%s%s (b. %s)", indent, formatPersonName(d.Person), d.Person.BirthDate),
						func() {
							navigateFunc(dCopy.Person.ID)
						})
					content.Add(personBtn)
				}
			}
			content.Add(widget.NewSeparator())
		}
	}

	scroll := container.NewVScroll(content)
	scroll.SetMinSize(fyne.NewSize(600, 400))

	descendantDialog = fyne.CurrentApp().NewWindow("Descendant (Pedigree) Report")
	
	// Export buttons - created after dialog window so we can pass it
	exportHTMLBtn := widget.NewButton("Export to HTML", func() {
		exportDescendantReportHTML(descendantDialog, s, rootPersonID)
	})
	exportPDFBtn := widget.NewButton("Export to PDF", func() {
		exportDescendantReportPDF(descendantDialog, s, rootPersonID)
	})
	exportButtons := container.NewHBox(exportHTMLBtn, exportPDFBtn)

	// Main content with export buttons at bottom
	mainContent := container.NewBorder(nil, exportButtons, nil, nil, scroll)
	descendantDialog.SetContent(mainContent)
	descendantDialog.Resize(fyne.NewSize(700, 600))
	descendantDialog.SetOnClosed(func() {
		descendantDialog = nil
	})
	descendantDialog.Show()
}

type DescendantNode struct {
	Person     store.Person
	Generation int
}

func collectDescendants(s *store.Store, personID int64, generation int) []DescendantNode {
	descendants := []DescendantNode{}

	rels, err := s.GetRelationshipsForPerson(personID)
	if err != nil {
		return descendants
	}

	for _, rel := range rels {
		if rel.Type == "child" && rel.SubjectID == personID {
			child, err := s.GetPersonByID(rel.ObjectID)
			if err != nil {
				continue
			}

			descendants = append(descendants, DescendantNode{
				Person:     *child,
				Generation: generation + 1,
			})

			// Recursively collect children's descendants
			childDescendants := collectDescendants(s, child.ID, generation+1)
			descendants = append(descendants, childDescendants...)
		}
	}

	return descendants
}

// Ancestor Report (Ahnentafel format)
var ancestorDialog fyne.Window

func showAncestorReport(w fyne.Window, s *store.Store, rootPersonID int64, navigateFunc func(int64)) {
	// Check if already open
	if ancestorDialog != nil {
		ancestorDialog.RequestFocus()
		ancestorDialog.Show()
		return
	}

	if rootPersonID <= 0 {
		dialog.ShowInformation("Ancestor (Ahnentafel) Report", "Please select a person first", w)
		return
	}

	rootPerson, err := s.GetPersonByID(rootPersonID)
	if err != nil {
		dialog.ShowError(fmt.Errorf("Failed to load person: %w", err), w)
		return
	}

	// Build ancestor tree in ahnentafel format
	ancestors := collectAncestors(s, rootPersonID, 1)

	// Build content
	content := container.NewVBox()

	title := widget.NewLabelWithStyle(
		fmt.Sprintf("Ancestors of %s (Ahnentafel Numbering)", formatPersonName(*rootPerson)),
		fyne.TextAlignCenter, fyne.TextStyle{Bold: true})
	content.Add(title)

	if len(ancestors) == 0 {
		content.Add(widget.NewLabel("No ancestors found."))
	} else {
		// Summary
		summary := widget.NewLabel(fmt.Sprintf("Total ancestors: %d", len(ancestors)))
		content.Add(summary)

		// Pedigree collapse detection
		uniqueIDs := make(map[int64]bool)
		collapseCount := 0
		for _, a := range ancestors {
			if uniqueIDs[a.Person.ID] {
				collapseCount++
			}
			uniqueIDs[a.Person.ID] = true
		}
		if collapseCount > 0 {
			collapseLabel := widget.NewLabelWithStyle(
				fmt.Sprintf("⚠️ Pedigree collapse detected: %d ancestors appear multiple times", collapseCount),
				fyne.TextAlignLeading, fyne.TextStyle{Italic: true})
			content.Add(collapseLabel)
		}

		content.Add(widget.NewSeparator())

		// Display ancestors in ahnentafel order
		// Sort by ahnentafel number
		sort.Slice(ancestors, func(i, j int) bool {
			return ancestors[i].AhnentafelNumber < ancestors[j].AhnentafelNumber
		})

		for _, a := range ancestors {
			aCopy := a
			generation := int(math.Log2(float64(a.AhnentafelNumber))) + 1
			relationship := getAhnentafelRelationship(a.AhnentafelNumber)

			personBtn := widget.NewButton(
				fmt.Sprintf("%d. %s (b. %s) - %s [Gen %d]",
					a.AhnentafelNumber, formatPersonName(a.Person), a.Person.BirthDate, relationship, generation),
				func() {
					navigateFunc(aCopy.Person.ID)
				})
			content.Add(personBtn)
		}
	}

	scroll := container.NewVScroll(content)
	scroll.SetMinSize(fyne.NewSize(700, 400))

	ancestorDialog = fyne.CurrentApp().NewWindow("Ancestor (Ahnentafel) Report")
	
	// Export buttons - created after dialog window so we can pass it
	exportHTMLBtn := widget.NewButton("Export to HTML", func() {
		exportAncestorReportHTML(ancestorDialog, s, rootPersonID)
	})
	exportPDFBtn := widget.NewButton("Export to PDF", func() {
		exportAncestorReportPDF(ancestorDialog, s, rootPersonID)
	})
	exportButtons := container.NewHBox(exportHTMLBtn, exportPDFBtn)

	// Main content with export buttons at bottom
	mainContent := container.NewBorder(nil, exportButtons, nil, nil, scroll)
	ancestorDialog.SetContent(mainContent)
	ancestorDialog.Resize(fyne.NewSize(800, 600))
	ancestorDialog.SetOnClosed(func() {
		ancestorDialog = nil
	})
	ancestorDialog.Show()
}

type AncestorNode struct {
	Person           store.Person
	AhnentafelNumber int
}

func collectAncestors(s *store.Store, personID int64, ahnentafelNum int) []AncestorNode {
	ancestors := []AncestorNode{}

	// Use the same method that Family View uses - GetRelatedPeople
	parents, err := s.GetRelatedPeople(personID, "parent")
	if err != nil {
		return ancestors
	}

	var father, mother *store.Person

	// Separate parents by gender (check both "M"/"Male" and "F"/"Female")
	for i := range parents {
		parent := &parents[i]

		if (parent.Gender == "Male" || parent.Gender == "M") && father == nil {
			father = parent
		} else if (parent.Gender == "Female" || parent.Gender == "F") && mother == nil {
			mother = parent
		}
	}

	// Father has ahnentafel number 2*n
	if father != nil {
		fatherNum := ahnentafelNum * 2
		ancestors = append(ancestors, AncestorNode{
			Person:           *father,
			AhnentafelNumber: fatherNum,
		})
		// Recursively collect father's ancestors
		fatherAncestors := collectAncestors(s, father.ID, fatherNum)
		ancestors = append(ancestors, fatherAncestors...)
	}

	// Mother has ahnentafel number 2*n + 1
	if mother != nil {
		motherNum := ahnentafelNum*2 + 1
		ancestors = append(ancestors, AncestorNode{
			Person:           *mother,
			AhnentafelNumber: motherNum,
		})
		// Recursively collect mother's ancestors
		motherAncestors := collectAncestors(s, mother.ID, motherNum)
		ancestors = append(ancestors, motherAncestors...)
	}

	return ancestors
}

func getAhnentafelRelationship(num int) string {
	if num == 1 {
		return "Self"
	} else if num == 2 {
		return "Father"
	} else if num == 3 {
		return "Mother"
	} else if num >= 4 && num <= 7 {
		if num%2 == 0 {
			return "Paternal Grandfather/Grandmother"
		}
		return "Maternal Grandfather/Grandmother"
	} else if num >= 8 && num <= 15 {
		return "Great-Grandparent"
	} else if num >= 16 && num <= 31 {
		return "2nd Great-Grandparent"
	} else if num >= 32 && num <= 63 {
		return "3rd Great-Grandparent"
	}
	generations := int(math.Log2(float64(num)))
	return fmt.Sprintf("%dth Great-Grandparent", generations-2)
}

// Family Group Sheet
var familyGroupDialog fyne.Window

func showFamilyGroupSheet(w fyne.Window, s *store.Store, personID int64, navigateFunc func(int64)) {
	// Check if already open
	if familyGroupDialog != nil {
		familyGroupDialog.RequestFocus()
		familyGroupDialog.Show()
		return
	}

	if personID <= 0 {
		dialog.ShowInformation("Family Group Sheet", "Please select a person first", w)
		return
	}

	person, err := s.GetPersonByID(personID)
	if err != nil {
		dialog.ShowError(fmt.Errorf("Failed to load person: %w", err), w)
		return
	}

	// Build content
	content := container.NewVBox()

	title := widget.NewLabelWithStyle(
		fmt.Sprintf("Family Group Sheet - %s", formatPersonName(*person)),
		fyne.TextAlignCenter, fyne.TextStyle{Bold: true})
	content.Add(title)
	content.Add(widget.NewSeparator())

	// Get spouse(s) with marriage information
	spouseInfos, _ := s.GetSpouses(personID)
	
	if len(spouseInfos) > 0 {
		// Sort spouses chronologically by marriage date (earliest first)
		sort.Slice(spouseInfos, func(i, j int) bool {
			return spouseInfos[i].MarriageDate < spouseInfos[j].MarriageDate
		})
		
		// Show as married couple with children
		for _, spouseInfo := range spouseInfos {
			spouseCopy := spouseInfo.Spouse
			renderMarriedFamily(content, s, person, &spouseCopy, navigateFunc)
			content.Add(widget.NewSeparator())
		}
	} else {
		// Show person's parents and siblings (person is the child)
		parents, _ := s.GetRelatedPeople(personID, "parent")
		renderParentalFamily(content, s, person, parents, navigateFunc)
	}

	scroll := container.NewVScroll(content)
	scroll.SetMinSize(fyne.NewSize(700, 400))

	familyGroupDialog = fyne.CurrentApp().NewWindow("Family Group Sheet")
	
	// Export buttons - created after dialog window so we can pass it
	exportHTMLBtn := widget.NewButton("Export to HTML", func() {
		exportFamilyGroupSheetHTML(familyGroupDialog, s, personID)
	})
	exportPDFBtn := widget.NewButton("Export to PDF", func() {
		exportFamilyGroupSheetPDF(familyGroupDialog, s, personID)
	})
	exportButtons := container.NewHBox(exportHTMLBtn, exportPDFBtn)

	// Main content with export buttons at bottom
	mainContent := container.NewBorder(nil, exportButtons, nil, nil, scroll)
	familyGroupDialog.SetContent(mainContent)
	familyGroupDialog.Resize(fyne.NewSize(800, 600))
	familyGroupDialog.SetOnClosed(func() {
		familyGroupDialog = nil
	})
	familyGroupDialog.Show()
}

// HTMLExportOptions holds privacy settings for HTML exports
type HTMLExportOptions struct {
	HideLiving      bool
	LimitLivingInfo bool
}

// showHTMLExportOptionsDialog shows privacy options before exporting HTML
func showHTMLExportOptionsDialog(w fyne.Window, person *store.Person, callback func(HTMLExportOptions)) {
	options := HTMLExportOptions{
		HideLiving:      false,
		LimitLivingInfo: false,
	}

	// Create checkboxes
	hideLivingCheck := widget.NewCheck("Hide all living people", func(checked bool) {
		options.HideLiving = checked
		// If hiding living people, disable limit option
		if checked {
			options.LimitLivingInfo = false
		}
	})

	limitInfoCheck := widget.NewCheck("Show living people with limited information (name only)", func(checked bool) {
		options.LimitLivingInfo = checked
		// If limiting info, disable hide option
		if checked {
			options.HideLiving = false
		}
	})

	// Explanation text
	explanation := widget.NewLabel("Privacy Options:\n\n" +
		"• Hide all living people: Completely excludes living individuals from the report\n" +
		"• Limited information: Shows only names for living people (no dates, places, marriage details, or other information)\n" +
		"• If neither is selected, all information is included\n\n" +
		"These options help protect privacy when sharing reports publicly.")
	explanation.Wrapping = fyne.TextWrapWord

	content := container.NewVBox(
		widget.NewLabelWithStyle("HTML Export Privacy Settings", fyne.TextAlignCenter, fyne.TextStyle{Bold: true}),
		widget.NewSeparator(),
		explanation,
		widget.NewSeparator(),
		hideLivingCheck,
		limitInfoCheck,
	)

	d := dialog.NewCustomConfirm("Export Options", "Continue", "Cancel", content, func(proceed bool) {
		if proceed {
			callback(options)
		}
	}, w)
	d.Resize(fyne.NewSize(500, 350))
	d.Show()
}

// exportCompleteWebsite generates a multi-page HTML website for the entire genealogy
func exportCompleteWebsite(w fyne.Window, s *store.Store) {
	// Show privacy options dialog
	showHTMLExportOptionsDialog(w, nil, func(options HTMLExportOptions) {
		// Step 1: Show dialog to get folder name
		defaultFolderName := "FamilyWebsite_" + time.Now().Format("2006-01-02")
		folderNameEntry := widget.NewEntry()
		folderNameEntry.SetText(defaultFolderName)
		folderNameEntry.SetPlaceHolder("Enter folder name")
		
		folderDialog := dialog.NewCustomConfirm(
			"Website Folder Name",
			"Next",
			"Cancel",
			container.NewVBox(
				widget.NewLabel("Choose a name for your website folder:"),
				folderNameEntry,
				widget.NewLabel("\nYou'll select where to create it in the next step."),
			),
			func(proceed bool) {
				if !proceed {
					return
				}
				
				folderName := strings.TrimSpace(folderNameEntry.Text)
				if folderName == "" {
					folderName = defaultFolderName
				}
				
				// Step 2: Show folder selection dialog for parent directory
				dialog.ShowFolderOpen(func(uri fyne.ListableURI, err error) {
					if err != nil || uri == nil {
						return
					}

					parentPath := uri.Path()
					fullFolderPath := filepath.Join(parentPath, folderName)
					
					// Check if folder already exists
					if _, err := os.Stat(fullFolderPath); err == nil {
						dialog.ShowConfirm(
							"Folder Exists",
							fmt.Sprintf("The folder '%s' already exists.\n\nDo you want to overwrite its contents?", folderName),
							func(overwrite bool) {
								if overwrite {
									generateWebsiteInFolder(w, s, fullFolderPath, options)
								}
							},
							w,
						)
						return
					}
					
					// Create the folder
					if err := os.MkdirAll(fullFolderPath, 0755); err != nil {
						dialog.ShowError(fmt.Errorf("Failed to create folder: %w", err), w)
						return
					}
					
					// Generate website
					generateWebsiteInFolder(w, s, fullFolderPath, options)
				}, w)
			},
			w,
		)
		folderDialog.Resize(fyne.NewSize(500, 200))
		folderDialog.Show()
	})
}

// generateWebsiteInFolder handles the actual website generation with progress indicator
func generateWebsiteInFolder(w fyne.Window, s *store.Store, folderPath string, options HTMLExportOptions) {
	// Generate website with progress indicator
	progressDialog := dialog.NewCustom("Generating Website", "Close", 
		widget.NewLabel("Generating family website...\nThis may take a moment."), w)
	progressDialog.Show()

	go func() {
		err := generateCompleteWebsite(s, folderPath, options)
		
		fyne.Do(func() {
			progressDialog.Hide()
			
			if err != nil {
				dialog.ShowError(fmt.Errorf("Failed to generate website: %w", err), w)
				return
			}

			dialog.ShowInformation("Website Generated Successfully",
				fmt.Sprintf("Family website created in:\n%s\n\nOpen index.html in your browser to view.", folderPath), w)
		})
	}()
}

// generateCompleteWebsite creates a multi-page HTML genealogy website
func generateCompleteWebsite(s *store.Store, outputDir string, options HTMLExportOptions) error {
	// Get all people from database
	allPeople, err := s.GetAllPeople()
	if err != nil {
		return fmt.Errorf("failed to get people: %w", err)
	}

	// Filter based on privacy settings
	var people []store.Person
	for _, p := range allPeople {
		if shouldShowPerson(&p, options) {
			people = append(people, p)
		}
	}

	if len(people) == 0 {
		return fmt.Errorf("no people to export after privacy filtering")
	}

	// Create shared CSS file
	cssPath := filepath.Join(outputDir, "style.css")
	if err := os.WriteFile(cssPath, []byte(getWebsiteCSS()), 0644); err != nil {
		return fmt.Errorf("failed to write CSS: %w", err)
	}

	// Generate index page
	indexPath := filepath.Join(outputDir, "index.html")
	indexHTML := generateIndexPage(s, people, options)
	if err := os.WriteFile(indexPath, []byte(indexHTML), 0644); err != nil {
		return fmt.Errorf("failed to write index.html: %w", err)
	}

	// Generate surname index
	surnamesPath := filepath.Join(outputDir, "surnames.html")
	surnamesHTML := generateSurnameIndexPage(s, people, options)
	if err := os.WriteFile(surnamesPath, []byte(surnamesHTML), 0644); err != nil {
		return fmt.Errorf("failed to write surnames.html: %w", err)
	}

	// Generate places index
	placesPath := filepath.Join(outputDir, "places.html")
	placesHTML := generatePlacesIndexPage(s, people, options)
	if err := os.WriteFile(placesPath, []byte(placesHTML), 0644); err != nil {
		return fmt.Errorf("failed to write places.html: %w", err)
	}

	// Generate timeline page
	timelinePath := filepath.Join(outputDir, "timeline.html")
	timelineHTML := generateTimelinePage(s, people, options)
	if err := os.WriteFile(timelinePath, []byte(timelineHTML), 0644); err != nil {
		return fmt.Errorf("failed to write timeline.html: %w", err)
	}

	// Generate statistics page
	statsPath := filepath.Join(outputDir, "statistics.html")
	statsHTML := generateStatisticsPage(s, allPeople, people, options)
	if err := os.WriteFile(statsPath, []byte(statsHTML), 0644); err != nil {
		return fmt.Errorf("failed to write statistics.html: %w", err)
	}

	// Generate sources page
	sourcesPath := filepath.Join(outputDir, "sources.html")
	sourcesHTML := generateSourcesPage(s, people, options)
	if err := os.WriteFile(sourcesPath, []byte(sourcesHTML), 0644); err != nil {
		return fmt.Errorf("failed to write sources.html: %w", err)
	}

	// Generate people JSON for search
	peopleJSON := generatePeopleJSON(people, options)

	// Generate search page with embedded data
	searchPath := filepath.Join(outputDir, "search.html")
	searchHTML := generateSearchPage(peopleJSON)
	if err := os.WriteFile(searchPath, []byte(searchHTML), 0644); err != nil {
		return fmt.Errorf("failed to write search.html: %w", err)
	}
	
	// Also save standalone JSON file for reference/API use
	peopleJSONPath := filepath.Join(outputDir, "people.json")
	if err := os.WriteFile(peopleJSONPath, []byte(peopleJSON), 0644); err != nil {
		return fmt.Errorf("failed to write people.json: %w", err)
	}

	// Generate individual person pages
	for _, person := range people {
		personPath := filepath.Join(outputDir, fmt.Sprintf("person_%d.html", person.ID))
		personHTML := generatePersonPage(s, &person, options)
		if err := os.WriteFile(personPath, []byte(personHTML), 0644); err != nil {
			return fmt.Errorf("failed to write person_%d.html: %w", person.ID, err)
		}
	}

	return nil
}

// getWebsiteCSS returns the shared CSS for the genealogy website
func getWebsiteCSS() string {
	return `/* KrankyBear Genealogy Website Styles */
* {
    margin: 0;
    padding: 0;
    box-sizing: border-box;
}

body {
    font-family: 'Segoe UI', Tahoma, Geneva, Verdana, sans-serif;
    line-height: 1.6;
    color: #333;
    background-color: #f5f5f5;
}

.header {
    background-color: #2c3e50;
    color: white;
    padding: 20px;
    box-shadow: 0 2px 5px rgba(0,0,0,0.1);
}

.header h1 {
    font-size: 2em;
    margin-bottom: 10px;
}

.nav {
    background-color: #34495e;
    padding: 15px 20px;
    box-shadow: 0 2px 5px rgba(0,0,0,0.1);
}

.nav a {
    color: white;
    text-decoration: none;
    padding: 8px 15px;
    margin-right: 10px;
    border-radius: 3px;
    transition: background-color 0.3s;
}

.nav a:hover {
    background-color: #2c3e50;
}

.container {
    max-width: 1200px;
    margin: 20px auto;
    padding: 30px;
    background-color: white;
    box-shadow: 0 2px 10px rgba(0,0,0,0.1);
    border-radius: 8px;
}

.person-card {
    background-color: #fafafa;
    border-left: 4px solid #3498db;
    padding: 15px;
    margin: 15px 0;
    border-radius: 3px;
}

.person-name {
    font-size: 1.3em;
    font-weight: bold;
    color: #2c3e50;
    margin-bottom: 8px;
}

.person-details {
    color: #555;
    margin-left: 20px;
}

.detail-row {
    margin: 5px 0;
}

.detail-label {
    font-weight: 600;
    color: #34495e;
    display: inline-block;
    min-width: 120px;
}

.section {
    margin: 30px 0;
}

.section h2 {
    color: #34495e;
    border-bottom: 2px solid #3498db;
    padding-bottom: 10px;
    margin-bottom: 20px;
}

.surname-group {
    margin: 20px 0;
}

.surname-group h3 {
    color: #2c3e50;
    background-color: #ecf0f1;
    padding: 10px;
    border-left: 4px solid #3498db;
}

.person-list {
    list-style: none;
    margin: 10px 0 10px 20px;
}

.person-list li {
    padding: 8px;
    margin: 5px 0;
    background-color: #fafafa;
    border-radius: 3px;
}

.person-list a {
    color: #3498db;
    text-decoration: none;
    font-weight: 500;
}

.person-list a:hover {
    text-decoration: underline;
}

.stats-grid {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
    gap: 20px;
    margin: 20px 0;
}

.stat-card {
    background-color: #ecf0f1;
    padding: 20px;
    border-radius: 5px;
    text-align: center;
}

.stat-number {
    font-size: 2.5em;
    font-weight: bold;
    color: #3498db;
}

.stat-label {
    color: #7f8c8d;
    margin-top: 5px;
}

.footer {
    margin-top: 40px;
    padding-top: 20px;
    border-top: 1px solid #ecf0f1;
    text-align: center;
    color: #7f8c8d;
    font-size: 0.9em;
}

.privacy-note {
    background-color: #fff3cd;
    border-left: 4px solid #ffc107;
    padding: 12px;
    margin: 20px 0;
    border-radius: 3px;
    font-style: italic;
}

.stat-bars {
    margin-top: 20px;
}

.stat-bar-row {
    display: grid;
    grid-template-columns: 200px 1fr 60px;
    gap: 15px;
    align-items: center;
    margin-bottom: 15px;
}

.stat-bar-label {
    font-weight: 600;
    color: #2c3e50;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
}

.stat-bar-label a {
    color: #2c3e50;
    text-decoration: none;
}

.stat-bar-label a:hover {
    color: #3498db;
    text-decoration: underline;
}

.stat-card a {
    display: block;
    transition: transform 0.2s;
}

.stat-card a:hover {
    transform: scale(1.05);
}

.stat-bar-container {
    background-color: #ecf0f1;
    border-radius: 10px;
    height: 30px;
    position: relative;
    overflow: hidden;
}

.stat-bar-fill {
    background: linear-gradient(90deg, #3498db, #2980b9);
    height: 100%;
    border-radius: 10px;
    transition: width 0.3s ease;
}

.stat-bar-value {
    text-align: right;
    font-weight: 600;
    color: #3498db;
}

.source-card {
    background-color: white;
    padding: 20px;
    margin-bottom: 20px;
    border-radius: 5px;
    border-left: 4px solid #3498db;
    box-shadow: 0 2px 4px rgba(0,0,0,0.1);
}

.source-card h3 {
    color: #2c3e50;
    margin-bottom: 15px;
    font-size: 1.3em;
}

.source-card p {
    margin: 8px 0;
    line-height: 1.6;
}

.citation-list {
    margin-top: 15px;
    padding-top: 15px;
    border-top: 1px solid #ecf0f1;
}

.citation-item {
    padding: 8px 0;
    display: flex;
    gap: 10px;
    align-items: center;
    flex-wrap: wrap;
}

.citation-item a {
    color: #3498db;
    text-decoration: none;
    font-weight: 500;
}

.citation-item a:hover {
    text-decoration: underline;
}

.citation-detail {
    color: #7f8c8d;
    font-size: 0.9em;
    font-style: italic;
}

.citation-confidence {
    padding: 2px 8px;
    border-radius: 3px;
    font-size: 0.8em;
    font-weight: 600;
    text-transform: uppercase;
}

.confidence-high {
    background-color: #d4edda;
    color: #155724;
}

.confidence-medium {
    background-color: #fff3cd;
    color: #856404;
}

.confidence-low {
    background-color: #f8d7da;
    color: #721c24;
}

.person-citations {
    margin-top: 15px;
}

.person-citation-item {
    background-color: #f8f9fa;
    padding: 15px;
    margin-bottom: 15px;
    border-radius: 5px;
    border-left: 3px solid #3498db;
}

.citation-source-title {
    font-weight: 600;
    color: #2c3e50;
    margin-bottom: 8px;
    font-size: 1.1em;
}

.citation-transcription {
    font-style: italic;
    color: #555;
    margin: 10px 0;
    padding: 10px;
    background-color: white;
    border-left: 3px solid #95a5a6;
}

.citation-notes {
    color: #7f8c8d;
    font-size: 0.9em;
    margin-top: 8px;
}

.photo-gallery {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(200px, 1fr));
    gap: 20px;
    margin: 20px 0;
}

.photo-item {
    background-color: #fafafa;
    border-radius: 5px;
    overflow: hidden;
    box-shadow: 0 2px 5px rgba(0,0,0,0.1);
}

.photo-item img {
    width: 100%;
    height: 200px;
    object-fit: cover;
    display: block;
}

.photo-caption {
    padding: 10px;
    font-weight: 600;
    color: #2c3e50;
}

.photo-date {
    padding: 0 10px 10px 10px;
    color: #7f8c8d;
    font-size: 0.9em;
}

.breadcrumb {
    max-width: 1200px;
    margin: 0 auto;
    padding: 15px 20px;
    background-color: #ecf0f1;
    font-size: 0.9em;
    border-radius: 5px;
    margin-top: 10px;
}

.breadcrumb a {
    color: #3498db;
    text-decoration: none;
}

.breadcrumb a:hover {
    text-decoration: underline;
}

.breadcrumb span {
    color: #7f8c8d;
}

@media print {
    body {
        background-color: white;
    }
    .container {
        box-shadow: none;
        max-width: 100%;
    }
    .nav {
        display: none;
    }
    .breadcrumb {
        display: none;
    }
    .photo-gallery {
        grid-template-columns: repeat(2, 1fr);
    }
    .photo-item img {
        height: 150px;
    }
}`
}

// generateIndexPage creates the home page for the website
func generateIndexPage(s *store.Store, people []store.Person, options HTMLExportOptions) string {
	var html strings.Builder

	// Count statistics
	totalPeople := len(people)
	livingCount := 0
	deceasedCount := 0
	surnameMap := make(map[string]int)

	for _, p := range people {
		if p.IsLiving {
			livingCount++
		} else {
			deceasedCount++
		}
		if p.Surname != "" {
			surnameMap[p.Surname]++
		}
	}

	html.WriteString(`<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Family Genealogy - Home</title>
    <link rel="stylesheet" href="style.css">
</head>
<body>
    <div class="header">
        <h1>Family Genealogy Website</h1>
        <p>Explore your family history</p>
    </div>
    <nav class="nav">
        <a href="index.html">Home</a>
        <a href="search.html">Search</a>
        <a href="surnames.html">Surnames</a>
        <a href="places.html">Places</a>
        <a href="timeline.html">Timeline</a>
        <a href="statistics.html">Statistics</a>
        <a href="sources.html">Sources</a>
    </nav>
    <div class="container">
        <div class="section">
            <h2>Welcome to the Family Tree</h2>
            <p>This website contains information about ` + fmt.Sprintf("%d", totalPeople) + ` individuals across ` + fmt.Sprintf("%d", len(surnameMap)) + ` different surnames.</p>
        </div>
`)

	// Privacy note
	if options.HideLiving || options.LimitLivingInfo {
		html.WriteString(`        <div class="privacy-note">`)
		if options.HideLiving {
			html.WriteString(`            <strong>Privacy Note:</strong> Information about living individuals has been excluded from this website to protect their privacy.`)
		} else {
			html.WriteString(`            <strong>Privacy Note:</strong> Information about living individuals is limited to names only to protect their privacy.`)
		}
		html.WriteString(`        </div>
`)
	}

	// Statistics
	html.WriteString(`        <div class="section">
            <h2>Statistics</h2>
            <div class="stats-grid">
                <div class="stat-card">
                    <div class="stat-number">` + fmt.Sprintf("%d", totalPeople) + `</div>
                    <div class="stat-label">Total People</div>
                </div>
                <div class="stat-card">
                    <div class="stat-number">` + fmt.Sprintf("%d", len(surnameMap)) + `</div>
                    <div class="stat-label">Surnames</div>
                </div>
                <div class="stat-card">
                    <div class="stat-number">` + fmt.Sprintf("%d", deceasedCount) + `</div>
                    <div class="stat-label">Deceased</div>
                </div>
`)
	if !options.HideLiving {
		html.WriteString(`                <div class="stat-card">
                    <div class="stat-number">` + fmt.Sprintf("%d", livingCount) + `</div>
                    <div class="stat-label">Living</div>
                </div>
`)
	}
	html.WriteString(`            </div>
        </div>

        <div class="section">
            <h2>Explore Your Family History</h2>
            <div style="display: grid; grid-template-columns: repeat(auto-fit, minmax(250px, 1fr)); gap: 20px; margin-top: 20px;">
                <div style="background-color: #ecf0f1; padding: 20px; border-radius: 5px;">
                    <h3 style="color: #2c3e50; margin-bottom: 10px;">🔍 Search</h3>
                    <p>Search for people by name, date, or place</p>
                    <a href="search.html" style="color: #3498db; font-weight: 600;">Search →</a>
                </div>
                <div style="background-color: #ecf0f1; padding: 20px; border-radius: 5px;">
                    <h3 style="color: #2c3e50; margin-bottom: 10px;">📋 Surnames</h3>
                    <p>Browse all individuals organized by family name</p>
                    <a href="surnames.html" style="color: #3498db; font-weight: 600;">View Surnames →</a>
                </div>
                <div style="background-color: #ecf0f1; padding: 20px; border-radius: 5px;">
                    <h3 style="color: #2c3e50; margin-bottom: 10px;">📍 Places</h3>
                    <p>Explore family history by geographic location</p>
                    <a href="places.html" style="color: #3498db; font-weight: 600;">View Places →</a>
                </div>
                <div style="background-color: #ecf0f1; padding: 20px; border-radius: 5px;">
                    <h3 style="color: #2c3e50; margin-bottom: 10px;">📅 Timeline</h3>
                    <p>Chronological view of all family events</p>
                    <a href="timeline.html" style="color: #3498db; font-weight: 600;">View Timeline →</a>
                </div>
                <div style="background-color: #ecf0f1; padding: 20px; border-radius: 5px;">
                    <h3 style="color: #2c3e50; margin-bottom: 10px;">📊 Statistics</h3>
                    <p>Detailed family history statistics and insights</p>
                    <a href="statistics.html" style="color: #3498db; font-weight: 600;">View Statistics →</a>
                </div>
                <div style="background-color: #ecf0f1; padding: 20px; border-radius: 5px;">
                    <h3 style="color: #2c3e50; margin-bottom: 10px;">📚 Sources</h3>
                    <p>Research sources and citations documentation</p>
                    <a href="sources.html" style="color: #3498db; font-weight: 600;">View Sources →</a>
                </div>
            </div>
        </div>

        <div class="footer">
            <p>Generated by KrankyBear Genealogy on ` + time.Now().Format("January 2, 2006") + `</p>
        </div>
    </div>
</body>
</html>`)

	return html.String()
}

// generateSurnameIndexPage creates the surname index page
func generateSurnameIndexPage(s *store.Store, people []store.Person, options HTMLExportOptions) string {
	var html strings.Builder

	// Group people by surname
	surnameMap := make(map[string][]store.Person)
	for _, p := range people {
		surname := p.Surname
		if surname == "" {
			surname = "(No surname)"
		}
		surnameMap[surname] = append(surnameMap[surname], p)
	}

	// Sort surnames
	var surnames []string
	for surname := range surnameMap {
		surnames = append(surnames, surname)
	}
	sort.Strings(surnames)

	html.WriteString(`<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Family Genealogy - Surnames</title>
    <link rel="stylesheet" href="style.css">
</head>
<body>
    <div class="header">
        <h1>Surname Index</h1>
        <p>Browse all individuals by surname</p>
    </div>
    <nav class="nav">
        <a href="index.html">Home</a>
        <a href="search.html">Search</a>
        <a href="surnames.html">Surnames</a>
        <a href="places.html">Places</a>
        <a href="timeline.html">Timeline</a>
        <a href="statistics.html">Statistics</a>
        <a href="sources.html">Sources</a>
    </nav>
    <div class="container">
`)

	// Generate surname groups
	for _, surname := range surnames {
		peopleInSurname := surnameMap[surname]
		// Sort by given name
		sort.Slice(peopleInSurname, func(i, j int) bool {
			return peopleInSurname[i].GivenName < peopleInSurname[j].GivenName
		})

		html.WriteString(`        <div class="surname-group">
            <h3>` + surname + ` (` + fmt.Sprintf("%d", len(peopleInSurname)) + `)</h3>
            <ul class="person-list">
`)
		for _, person := range peopleInSurname {
			dates := ""
			if !options.LimitLivingInfo || !person.IsLiving {
				if person.BirthDate != "" {
					dates = fmt.Sprintf(" (b. %s", formatFGSDate(person.BirthDate))
					if !person.IsLiving && person.DeathDate != "" {
						dates += fmt.Sprintf(" - d. %s", formatFGSDate(person.DeathDate))
					}
					dates += ")"
				}
			} else if person.IsLiving {
				dates = " (Living)"
			}
			html.WriteString(`                <li><a href="person_` + fmt.Sprintf("%d", person.ID) + `.html">` + formatPersonName(person) + `</a>` + dates + `</li>
`)
		}
		html.WriteString(`            </ul>
        </div>
`)
	}

	html.WriteString(`        <div class="footer">
            <p>Generated by KrankyBear Genealogy on ` + time.Now().Format("January 2, 2006") + `</p>
        </div>
    </div>
</body>
</html>`)

	return html.String()
}

// generatePlacesIndexPage creates the places index page
func generatePlacesIndexPage(s *store.Store, people []store.Person, options HTMLExportOptions) string {
	var html strings.Builder

	// Collect all places
	birthPlaces := make(map[string][]store.Person)
	deathPlaces := make(map[string][]store.Person)
	
	for _, p := range people {
		if p.BirthPlace != "" && (!options.LimitLivingInfo || !p.IsLiving) {
			birthPlaces[p.BirthPlace] = append(birthPlaces[p.BirthPlace], p)
		}
		if p.DeathPlace != "" && !p.IsLiving {
			deathPlaces[p.DeathPlace] = append(deathPlaces[p.DeathPlace], p)
		}
	}

	// Get unique sorted places
	placeSet := make(map[string]bool)
	for place := range birthPlaces {
		placeSet[place] = true
	}
	for place := range deathPlaces {
		placeSet[place] = true
	}
	
	var places []string
	for place := range placeSet {
		places = append(places, place)
	}
	sort.Strings(places)

	html.WriteString(`<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Family Genealogy - Places</title>
    <link rel="stylesheet" href="style.css">
</head>
<body>
    <div class="header">
        <h1>Places Index</h1>
        <p>Browse family history by location</p>
    </div>
    <nav class="nav">
        <a href="index.html">Home</a>
        <a href="search.html">Search</a>
        <a href="surnames.html">Surnames</a>
        <a href="places.html">Places</a>
        <a href="timeline.html">Timeline</a>
        <a href="statistics.html">Statistics</a>
        <a href="sources.html">Sources</a>
    </nav>
    <div class="container">
        <div class="section">
            <p>` + fmt.Sprintf("%d locations", len(places)) + ` recorded in family history</p>
        </div>
`)

	for _, place := range places {
		births := birthPlaces[place]
		deaths := deathPlaces[place]
		totalEvents := len(births) + len(deaths)
		
		html.WriteString(`        <div class="surname-group">
            <h3>` + place + ` (` + fmt.Sprintf("%d events", totalEvents) + `)</h3>
`)
		
		if len(births) > 0 {
			html.WriteString(`            <h4 style="margin: 10px 0 5px 20px; color: #2c3e50;">Births (` + fmt.Sprintf("%d", len(births)) + `)</h4>
            <ul class="person-list">
`)
			// Sort births by date
			sort.Slice(births, func(i, j int) bool {
				return births[i].BirthDate < births[j].BirthDate
			})
			for _, person := range births {
				dateStr := ""
				if person.BirthDate != "" {
					dateStr = " (" + formatFGSDate(person.BirthDate) + ")"
				}
				html.WriteString(`                <li><a href="person_` + fmt.Sprintf("%d", person.ID) + `.html">` + formatPersonName(person) + `</a>` + dateStr + `</li>
`)
			}
			html.WriteString(`            </ul>
`)
		}
		
		if len(deaths) > 0 {
			html.WriteString(`            <h4 style="margin: 10px 0 5px 20px; color: #2c3e50;">Deaths (` + fmt.Sprintf("%d", len(deaths)) + `)</h4>
            <ul class="person-list">
`)
			// Sort deaths by date
			sort.Slice(deaths, func(i, j int) bool {
				return deaths[i].DeathDate < deaths[j].DeathDate
			})
			for _, person := range deaths {
				dateStr := ""
				if person.DeathDate != "" {
					dateStr = " (" + formatFGSDate(person.DeathDate) + ")"
				}
				html.WriteString(`                <li><a href="person_` + fmt.Sprintf("%d", person.ID) + `.html">` + formatPersonName(person) + `</a>` + dateStr + `</li>
`)
			}
			html.WriteString(`            </ul>
`)
		}
		
		html.WriteString(`        </div>
`)
	}

	html.WriteString(`        <div class="footer">
            <p>Generated by KrankyBear Genealogy on ` + time.Now().Format("January 2, 2006") + `</p>
        </div>
    </div>
</body>
</html>`)

	return html.String()
}

// generateTimelinePage creates a chronological timeline page
func generateTimelinePage(s *store.Store, people []store.Person, options HTMLExportOptions) string {
	var html strings.Builder

	// Collect all events
	type WebTimelineEvent struct {
		Date   string
		Year   string
		Type   string // "birth", "death"
		Person store.Person
		Place  string
	}
	
	var events []WebTimelineEvent
	
	// Add births
	for _, p := range people {
		// Show living people on timeline, but hide details if LimitLivingInfo is set
		if p.BirthDate != "" {
			if options.LimitLivingInfo && p.IsLiving {
				// Add to timeline but without date/place details
				events = append(events, WebTimelineEvent{
					Date:   "", // Empty date will show as "Living"
					Year:   "",
					Type:   "birth",
					Person: p,
					Place:  "",
				})
			} else {
				// Show full details
				year := extractYear(p.BirthDate)
				events = append(events, WebTimelineEvent{
					Date:   p.BirthDate,
					Year:   year,
					Type:   "birth",
					Person: p,
					Place:  p.BirthPlace,
				})
			}
		}
	}
	
	// Add deaths
	for _, p := range people {
		if p.DeathDate != "" && !p.IsLiving {
			year := extractYear(p.DeathDate)
			events = append(events, WebTimelineEvent{
				Date:   p.DeathDate,
				Year:   year,
				Type:   "death",
				Person: p,
				Place:  p.DeathPlace,
			})
		}
	}
	
	// Sort by date chronologically, then alphabetically by name for living people
	sort.Slice(events, func(i, j int) bool {
		// Parse dates for proper chronological comparison
		dateI := normalizeDateForSort(events[i].Date)
		dateJ := normalizeDateForSort(events[j].Date)
		
		// If dates are the same (both empty = living people), sort by name
		if dateI == dateJ {
			nameI := formatPersonName(events[i].Person)
			nameJ := formatPersonName(events[j].Person)
			return nameI < nameJ
		}
		
		return dateI < dateJ
	})

	html.WriteString(`<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Family Genealogy - Timeline</title>
    <link rel="stylesheet" href="style.css">
</head>
<body>
    <div class="header">
        <h1>Family Timeline</h1>
        <p>Chronological view of family events</p>
    </div>
    <nav class="nav">
        <a href="index.html">Home</a>
        <a href="search.html">Search</a>
        <a href="surnames.html">Surnames</a>
        <a href="places.html">Places</a>
        <a href="timeline.html">Timeline</a>
        <a href="statistics.html">Statistics</a>
        <a href="sources.html">Sources</a>
    </nav>
    <div class="container">
        <div class="section">
            <p>` + fmt.Sprintf("%d events", len(events)) + ` in chronological order</p>
        </div>
`)

	// Group by century, with living people at the end
	currentCentury := ""
	for _, event := range events {
		// Extract century from year string
		centuryStr := ""
		if len(event.Year) >= 4 {
			if yearInt, err := strconv.Atoi(event.Year); err == nil {
				century := (yearInt / 100) * 100
				centuryStr = fmt.Sprintf("%d", century)
			}
		} else if event.Date == "" && event.Person.IsLiving {
			// Living people with hidden dates go in "Living" section
			centuryStr = "Living"
		}
		
		if centuryStr != "" && centuryStr != currentCentury {
			if currentCentury != "" {
				html.WriteString(`            </ul>
        </div>
`)
			}
			currentCentury = centuryStr
			sectionTitle := centuryStr
			if centuryStr != "Living" {
				sectionTitle = centuryStr + "s"
			}
			html.WriteString(`        <div class="surname-group">
            <h3>` + sectionTitle + `</h3>
            <ul class="person-list">
`)
		}
		
		eventIcon := "📅"
		if event.Type == "birth" {
			eventIcon = "👶"
		} else if event.Type == "death" {
			eventIcon = "✝️"
		}
		
		eventType := event.Type
		if event.Type == "birth" {
			eventType = "Born"
		} else if event.Type == "death" {
			eventType = "Died"
		}
		
		// Format date and place
		dateStr := ""
		if event.Date != "" {
			dateStr = `<strong>` + formatFGSDate(event.Date) + `</strong> - `
		}
		
		placeStr := ""
		if event.Place != "" {
			placeStr = " in " + event.Place
		} else if event.Date == "" && event.Person.IsLiving {
			placeStr = " (birth information withheld for privacy)"
		}
		
		html.WriteString(`                <li>` + eventIcon + ` ` + dateStr + eventType + `: <a href="person_` + fmt.Sprintf("%d", event.Person.ID) + `.html">` + formatPersonName(event.Person) + `</a>` + placeStr + `</li>
`)
	}
	
	if len(events) > 0 {
		html.WriteString(`            </ul>
        </div>
`)
	}

	html.WriteString(`        <div class="footer">
            <p>Generated by KrankyBear Genealogy on ` + time.Now().Format("January 2, 2006") + `</p>
        </div>
    </div>
</body>
</html>`)

	return html.String()
}

// generateStatisticsPage creates the statistics dashboard page
func generateStatisticsPage(s *store.Store, allPeople []store.Person, visiblePeople []store.Person, options HTMLExportOptions) string {
	var html strings.Builder

	// Calculate statistics
	totalPeople := len(visiblePeople)
	livingCount := 0
	deceasedCount := 0
	maleCount := 0
	femaleCount := 0
	unknownGenderCount := 0
	
	birthCount := 0
	deathCount := 0
	marriageCount := 0
	
	var earliestBirth, latestBirth, earliestDeath, latestDeath string
	
	surnameMap := make(map[string]int)
	birthPlaceMap := make(map[string]int)
	deathPlaceMap := make(map[string]int)
	
	for _, p := range visiblePeople {
		// Living/Deceased
		if p.IsLiving {
			livingCount++
		} else {
			deceasedCount++
		}
		
		// Gender
		switch p.Gender {
		case "M":
			maleCount++
		case "F":
			femaleCount++
		default:
			unknownGenderCount++
		}
		
		// Surnames
		if p.Surname != "" {
			surnameMap[p.Surname]++
		}
		
		// Birth statistics
		if p.BirthDate != "" && (!options.LimitLivingInfo || !p.IsLiving) {
			birthCount++
			if earliestBirth == "" || p.BirthDate < earliestBirth {
				earliestBirth = p.BirthDate
			}
			if latestBirth == "" || p.BirthDate > latestBirth {
				latestBirth = p.BirthDate
			}
			
			if p.BirthPlace != "" {
				birthPlaceMap[p.BirthPlace]++
			}
		}
		
		// Death statistics
		if p.DeathDate != "" {
			deathCount++
			if earliestDeath == "" || p.DeathDate < earliestDeath {
				earliestDeath = p.DeathDate
			}
			if latestDeath == "" || p.DeathDate > latestDeath {
				latestDeath = p.DeathDate
			}
			
			if p.DeathPlace != "" {
				deathPlaceMap[p.DeathPlace]++
			}
		}
	}
	
	// Count marriages (only for visible people)
	for _, p := range visiblePeople {
		spouses, err := s.GetSpouses(p.ID)
		if err != nil {
			continue
		}
		for _, sp := range spouses {
			// Only count if both spouses are visible and this is the "first" one (to avoid double-counting)
			if p.ID < sp.Spouse.ID {
				spouseVisible := false
				for _, vp := range visiblePeople {
					if vp.ID == sp.Spouse.ID {
						spouseVisible = true
						break
					}
				}
				if spouseVisible {
					marriageCount++
				}
			}
		}
	}
	
	// Sort surnames by frequency
	type SurnameCount struct {
		Name  string
		Count int
	}
	var surnames []SurnameCount
	for name, count := range surnameMap {
		surnames = append(surnames, SurnameCount{Name: name, Count: count})
	}
	sort.Slice(surnames, func(i, j int) bool {
		if surnames[i].Count == surnames[j].Count {
			return surnames[i].Name < surnames[j].Name
		}
		return surnames[i].Count > surnames[j].Count
	})
	
	// Sort places by frequency
	type PlaceCount struct {
		Name  string
		Count int
	}
	var birthPlaces []PlaceCount
	for name, count := range birthPlaceMap {
		birthPlaces = append(birthPlaces, PlaceCount{Name: name, Count: count})
	}
	sort.Slice(birthPlaces, func(i, j int) bool {
		if birthPlaces[i].Count == birthPlaces[j].Count {
			return birthPlaces[i].Name < birthPlaces[j].Name
		}
		return birthPlaces[i].Count > birthPlaces[j].Count
	})
	
	var deathPlaces []PlaceCount
	for name, count := range deathPlaceMap {
		deathPlaces = append(deathPlaces, PlaceCount{Name: name, Count: count})
	}
	sort.Slice(deathPlaces, func(i, j int) bool {
		if deathPlaces[i].Count == deathPlaces[j].Count {
			return deathPlaces[i].Name < deathPlaces[j].Name
		}
		return deathPlaces[i].Count > deathPlaces[j].Count
	})

	// Start HTML
	html.WriteString(`<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Family Genealogy - Statistics</title>
    <link rel="stylesheet" href="style.css">
</head>
<body>
    <div class="header">
        <h1>Family Statistics</h1>
        <p>Overview and insights about the family tree</p>
    </div>
    <nav class="nav">
        <a href="index.html">Home</a>
        <a href="search.html">Search</a>
        <a href="surnames.html">Surnames</a>
        <a href="places.html">Places</a>
        <a href="timeline.html">Timeline</a>
        <a href="statistics.html">Statistics</a>
        <a href="sources.html">Sources</a>
    </nav>
    <div class="container">
`)

	// Privacy note
	if options.HideLiving || options.LimitLivingInfo {
		html.WriteString(`        <div class="privacy-note">`)
		if options.HideLiving {
			html.WriteString(`            <strong>Privacy Note:</strong> Living individuals are excluded from these statistics.`)
		} else {
			html.WriteString(`            <strong>Privacy Note:</strong> Birth information for living individuals is excluded from date range and place statistics.`)
		}
		html.WriteString(`        </div>
`)
	}

	// Overall statistics
	html.WriteString(`        <div class="section">
            <h2>Overall Statistics</h2>
            <div class="stats-grid">
                <div class="stat-card">
                    <div class="stat-number">` + fmt.Sprintf("%d", totalPeople) + `</div>
                    <div class="stat-label">Total People</div>
                </div>
                <div class="stat-card">
                    <div class="stat-number">` + fmt.Sprintf("%d", deceasedCount) + `</div>
                    <div class="stat-label">Deceased</div>
                </div>
`)
	if !options.HideLiving {
		html.WriteString(`                <div class="stat-card">
                    <div class="stat-number">` + fmt.Sprintf("%d", livingCount) + `</div>
                    <div class="stat-label">Living</div>
                </div>
`)
	}
	html.WriteString(`                <div class="stat-card">
                    <a href="surnames.html" style="text-decoration: none; color: inherit;">
                        <div class="stat-number">` + fmt.Sprintf("%d", len(surnameMap)) + `</div>
                        <div class="stat-label">Surnames</div>
                    </a>
                </div>
                <div class="stat-card">
                    <div class="stat-number">` + fmt.Sprintf("%d", marriageCount) + `</div>
                    <div class="stat-label">Marriages</div>
                </div>
            </div>
        </div>

        <div class="section">
            <h2>Gender Distribution</h2>
            <div class="stats-grid">
                <div class="stat-card">
                    <div class="stat-number">` + fmt.Sprintf("%d", maleCount) + `</div>
                    <div class="stat-label">Male</div>
                </div>
                <div class="stat-card">
                    <div class="stat-number">` + fmt.Sprintf("%d", femaleCount) + `</div>
                    <div class="stat-label">Female</div>
                </div>
`)
	if unknownGenderCount > 0 {
		html.WriteString(`                <div class="stat-card">
                    <div class="stat-number">` + fmt.Sprintf("%d", unknownGenderCount) + `</div>
                    <div class="stat-label">Unknown</div>
                </div>
`)
	}
	html.WriteString(`            </div>
        </div>
`)

	// Date ranges
	if earliestBirth != "" || latestBirth != "" || earliestDeath != "" || latestDeath != "" {
		html.WriteString(`        <div class="section">
            <h2>Date Ranges</h2>
            <div class="stats-grid">
`)
		if earliestBirth != "" {
			html.WriteString(`                <div class="stat-card">
                    <div class="stat-number" style="font-size: 1.5rem;">` + formatFGSDate(earliestBirth) + `</div>
                    <div class="stat-label">Earliest Birth</div>
                </div>
`)
		}
		if latestBirth != "" {
			html.WriteString(`                <div class="stat-card">
                    <div class="stat-number" style="font-size: 1.5rem;">` + formatFGSDate(latestBirth) + `</div>
                    <div class="stat-label">Latest Birth</div>
                </div>
`)
		}
		if earliestDeath != "" {
			html.WriteString(`                <div class="stat-card">
                    <div class="stat-number" style="font-size: 1.5rem;">` + formatFGSDate(earliestDeath) + `</div>
                    <div class="stat-label">Earliest Death</div>
                </div>
`)
		}
		if latestDeath != "" {
			html.WriteString(`                <div class="stat-card">
                    <div class="stat-number" style="font-size: 1.5rem;">` + formatFGSDate(latestDeath) + `</div>
                    <div class="stat-label">Latest Death</div>
                </div>
`)
		}
		html.WriteString(`            </div>
        </div>
`)
	}

	// Top 10 Surnames
	if len(surnames) > 0 {
		html.WriteString(`        <div class="section">
            <h2>Most Common Surnames</h2>
            <div class="stat-bars">
`)
		maxCount := surnames[0].Count
		displayCount := 10
		if len(surnames) < displayCount {
			displayCount = len(surnames)
		}
		
		for i := 0; i < displayCount; i++ {
			sn := surnames[i]
			percentage := float64(sn.Count) / float64(maxCount) * 100
			html.WriteString(`                <div class="stat-bar-row">
                    <div class="stat-bar-label"><a href="surnames.html#` + sn.Name + `">` + sn.Name + `</a></div>
                    <div class="stat-bar-container">
                        <div class="stat-bar-fill" style="width: ` + fmt.Sprintf("%.1f", percentage) + `%;"></div>
                    </div>
                    <div class="stat-bar-value">` + fmt.Sprintf("%d", sn.Count) + `</div>
                </div>
`)
		}
		html.WriteString(`            </div>
        </div>
`)
	}

	// Top 10 Birth Places
	if len(birthPlaces) > 0 {
		html.WriteString(`        <div class="section">
            <h2>Most Common Birth Places</h2>
            <div class="stat-bars">
`)
		maxCount := birthPlaces[0].Count
		displayCount := 10
		if len(birthPlaces) < displayCount {
			displayCount = len(birthPlaces)
		}
		
		for i := 0; i < displayCount; i++ {
			pl := birthPlaces[i]
			percentage := float64(pl.Count) / float64(maxCount) * 100
			html.WriteString(`                <div class="stat-bar-row">
                    <div class="stat-bar-label"><a href="places.html">` + pl.Name + `</a></div>
                    <div class="stat-bar-container">
                        <div class="stat-bar-fill" style="width: ` + fmt.Sprintf("%.1f", percentage) + `%;"></div>
                    </div>
                    <div class="stat-bar-value">` + fmt.Sprintf("%d", pl.Count) + `</div>
                </div>
`)
		}
		html.WriteString(`            </div>
        </div>
`)
	}

	// Top 10 Death Places
	if len(deathPlaces) > 0 {
		html.WriteString(`        <div class="section">
            <h2>Most Common Death Places</h2>
            <div class="stat-bars">
`)
		maxCount := deathPlaces[0].Count
		displayCount := 10
		if len(deathPlaces) < displayCount {
			displayCount = len(deathPlaces)
		}
		
		for i := 0; i < displayCount; i++ {
			pl := deathPlaces[i]
			percentage := float64(pl.Count) / float64(maxCount) * 100
			html.WriteString(`                <div class="stat-bar-row">
                    <div class="stat-bar-label"><a href="places.html">` + pl.Name + `</a></div>
                    <div class="stat-bar-container">
                        <div class="stat-bar-fill" style="width: ` + fmt.Sprintf("%.1f", percentage) + `%;"></div>
                    </div>
                    <div class="stat-bar-value">` + fmt.Sprintf("%d", pl.Count) + `</div>
                </div>
`)
		}
		html.WriteString(`            </div>
        </div>
`)
	}

	html.WriteString(`        <div class="footer">
            <p>Generated by KrankyBear Genealogy on ` + time.Now().Format("January 2, 2006") + `</p>
        </div>
    </div>
</body>
</html>`)

	return html.String()
}

// generateSourcesPage creates the sources and citations page
func generateSourcesPage(s *store.Store, visiblePeople []store.Person, options HTMLExportOptions) string {
	var html strings.Builder

	// Get all sources
	allSources, err := s.GetAllSources()
	if err != nil {
		allSources = []store.Source{}
	}

	// Create a map of visible person IDs for quick lookup
	visiblePersonIDs := make(map[int64]bool)
	for _, p := range visiblePeople {
		visiblePersonIDs[p.ID] = true
	}

	// Group sources by type and collect citation counts
	type SourceWithCitations struct {
		Source      store.Source
		Citations   []store.Citation
		PeopleCount int
	}
	
	sourcesByType := make(map[string][]SourceWithCitations)
	
	for _, source := range allSources {
		citations, err := s.GetCitationsForSource(source.ID)
		if err != nil {
			continue
		}
		
		// Filter citations to only include visible people
		var visibleCitations []store.Citation
		for _, cit := range citations {
			if visiblePersonIDs[cit.PersonID] {
				visibleCitations = append(visibleCitations, cit)
			}
		}
		
		// Only include sources that have visible citations
		if len(visibleCitations) > 0 {
			sourceType := source.SourceType
			if sourceType == "" {
				sourceType = "other"
			}
			
			sourcesByType[sourceType] = append(sourcesByType[sourceType], SourceWithCitations{
				Source:      source,
				Citations:   visibleCitations,
				PeopleCount: len(visibleCitations),
			})
		}
	}

	// Source type labels
	sourceTypeLabels := map[string]string{
		"vital_record": "Vital Records",
		"census":       "Census Records",
		"church":       "Church Records",
		"book":         "Books & Publications",
		"website":      "Websites & Online Resources",
		"military":     "Military Records",
		"land":         "Land & Property Records",
		"probate":      "Probate & Court Records",
		"newspaper":    "Newspapers",
		"other":        "Other Sources",
	}

	// Start HTML
	html.WriteString(`<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Family Genealogy - Sources</title>
    <link rel="stylesheet" href="style.css">
</head>
<body>
    <div class="header">
        <h1>Research Sources</h1>
        <p>Documentation and citations for family history research</p>
    </div>
    <nav class="nav">
        <a href="index.html">Home</a>
        <a href="search.html">Search</a>
        <a href="surnames.html">Surnames</a>
        <a href="places.html">Places</a>
        <a href="timeline.html">Timeline</a>
        <a href="statistics.html">Statistics</a>
        <a href="sources.html">Sources</a>
    </nav>
    <div class="container">
`)

	if len(sourcesByType) == 0 {
		html.WriteString(`        <div class="section">
            <p>No sources have been added to the database yet.</p>
        </div>
`)
	} else {
		// Count totals
		totalSources := len(allSources)
		totalCitations := 0
		for _, sources := range sourcesByType {
			for _, swc := range sources {
				totalCitations += len(swc.Citations)
			}
		}

		html.WriteString(`        <div class="section">
            <div class="stats-grid">
                <div class="stat-card">
                    <div class="stat-number">` + fmt.Sprintf("%d", totalSources) + `</div>
                    <div class="stat-label">Total Sources</div>
                </div>
                <div class="stat-card">
                    <div class="stat-number">` + fmt.Sprintf("%d", totalCitations) + `</div>
                    <div class="stat-label">Total Citations</div>
                </div>
                <div class="stat-card">
                    <div class="stat-number">` + fmt.Sprintf("%d", len(sourcesByType)) + `</div>
                    <div class="stat-label">Source Types</div>
                </div>
            </div>
        </div>
`)

		// Display sources grouped by type
		// Sort types for consistent display
		var sortedTypes []string
		for sourceType := range sourcesByType {
			sortedTypes = append(sortedTypes, sourceType)
		}
		sort.Strings(sortedTypes)

		for _, sourceType := range sortedTypes {
			sources := sourcesByType[sourceType]
			typeLabel := sourceTypeLabels[sourceType]
			if typeLabel == "" {
				typeLabel = sourceType
			}

			html.WriteString(`        <div class="section">
            <h2>` + typeLabel + ` (` + fmt.Sprintf("%d", len(sources)) + `)</h2>
`)

			// Sort sources by title
			sort.Slice(sources, func(i, j int) bool {
				return sources[i].Source.Title < sources[j].Source.Title
			})

			for _, swc := range sources {
				src := swc.Source
				
				html.WriteString(`            <div class="source-card">
                <h3>` + src.Title + `</h3>
`)

				if src.Author != "" {
					html.WriteString(`                <p><strong>Author:</strong> ` + src.Author + `</p>
`)
				}
				if src.Publication != "" {
					html.WriteString(`                <p><strong>Publication:</strong> ` + src.Publication + `</p>
`)
				}
				if src.Repository != "" {
					html.WriteString(`                <p><strong>Repository:</strong> ` + src.Repository + `</p>
`)
				}
				if src.CallNumber != "" {
					html.WriteString(`                <p><strong>Call Number:</strong> ` + src.CallNumber + `</p>
`)
				}
				if src.Notes != "" {
					html.WriteString(`                <p><strong>Notes:</strong> ` + src.Notes + `</p>
`)
				}

				html.WriteString(`                <p><strong>Citations:</strong> ` + fmt.Sprintf("%d", len(swc.Citations)) + ` people</p>
                <div class="citation-list">
`)

				// Show people who cite this source
				for _, cit := range swc.Citations {
					person, err := s.GetPersonByID(cit.PersonID)
					if err != nil {
						continue
					}
					
					html.WriteString(`                    <div class="citation-item">
                        <a href="person_` + fmt.Sprintf("%d", person.ID) + `.html">` + formatPersonName(*person) + `</a>
`)
					
					if cit.CitationDetail != "" {
						html.WriteString(`                        <span class="citation-detail">` + cit.CitationDetail + `</span>
`)
					}
					if cit.Confidence != "" {
						confidenceClass := "confidence-" + cit.Confidence
						html.WriteString(`                        <span class="citation-confidence ` + confidenceClass + `">` + cit.Confidence + `</span>
`)
					}
					
					html.WriteString(`                    </div>
`)
				}

				html.WriteString(`                </div>
            </div>
`)
			}

			html.WriteString(`        </div>
`)
		}
	}

	html.WriteString(`        <div class="footer">
            <p>Generated by KrankyBear Genealogy on ` + time.Now().Format("January 2, 2006") + `</p>
        </div>
    </div>
</body>
</html>`)

	return html.String()
}

// generatePeopleJSON creates a JSON file with searchable person data
func generatePeopleJSON(people []store.Person, options HTMLExportOptions) string {
	type PersonJSON struct {
		ID         int64  `json:"id"`
		Name       string `json:"name"`
		GivenName  string `json:"given_name"`
		Surname    string `json:"surname"`
		BirthDate  string `json:"birth_date,omitempty"`
		BirthPlace string `json:"birth_place,omitempty"`
		DeathDate  string `json:"death_date,omitempty"`
		DeathPlace string `json:"death_place,omitempty"`
		IsLiving   bool   `json:"is_living"`
	}

	jsonPeople := make([]PersonJSON, 0, len(people))
	for _, p := range people {
		personJSON := PersonJSON{
			ID:        p.ID,
			Name:      formatPersonName(p),
			GivenName: p.GivenName,
			Surname:   p.Surname,
			IsLiving:  p.IsLiving,
		}

		// Include birth/death info based on privacy settings
		if !options.LimitLivingInfo || !p.IsLiving {
			personJSON.BirthDate = p.BirthDate
			personJSON.BirthPlace = p.BirthPlace
		}
		if !p.IsLiving {
			personJSON.DeathDate = p.DeathDate
			personJSON.DeathPlace = p.DeathPlace
		}

		jsonPeople = append(jsonPeople, personJSON)
	}

	// Marshal to JSON with indentation for readability
	jsonBytes, err := json.MarshalIndent(jsonPeople, "", "  ")
	if err != nil {
		// Fallback to empty array
		fmt.Printf("Error marshaling people JSON: %v\n", err)
		return "[]"
	}
	return string(jsonBytes)
}

// generateSearchPage creates the search page with JavaScript functionality
func generateSearchPage(peopleJSON string) string {
	return `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Family Genealogy - Search</title>
    <link rel="stylesheet" href="style.css">
    <style>
        .search-container {
            max-width: 800px;
            margin: 0 auto;
        }
        .search-box {
            width: 100%;
            padding: 15px;
            font-size: 1.1em;
            border: 2px solid #3498db;
            border-radius: 5px;
            margin-bottom: 20px;
            box-sizing: border-box;
        }
        .search-box:focus {
            outline: none;
            border-color: #2980b9;
            box-shadow: 0 0 5px rgba(52, 152, 219, 0.5);
        }
        .search-results {
            margin-top: 20px;
        }
        .search-result-item {
            background-color: white;
            padding: 15px;
            margin-bottom: 10px;
            border-radius: 5px;
            border-left: 4px solid #3498db;
            box-shadow: 0 2px 4px rgba(0,0,0,0.1);
            transition: transform 0.2s;
        }
        .search-result-item:hover {
            transform: translateX(5px);
        }
        .search-result-name {
            font-size: 1.2em;
            font-weight: 600;
            color: #2c3e50;
            margin-bottom: 8px;
        }
        .search-result-name a {
            color: #3498db;
            text-decoration: none;
        }
        .search-result-name a:hover {
            text-decoration: underline;
        }
        .search-result-details {
            color: #7f8c8d;
            font-size: 0.95em;
        }
        .search-result-highlight {
            background-color: #fff3cd;
            padding: 2px 4px;
            border-radius: 2px;
        }
        .search-stats {
            color: #7f8c8d;
            margin-bottom: 15px;
            font-style: italic;
        }
        .no-results {
            text-align: center;
            color: #7f8c8d;
            padding: 40px;
            font-size: 1.1em;
        }
    </style>
</head>
<body>
    <div class="header">
        <h1>Search Family Tree</h1>
        <p>Search by name, date, or place</p>
    </div>
    <nav class="nav">
        <a href="index.html">Home</a>
        <a href="search.html">Search</a>
        <a href="surnames.html">Surnames</a>
        <a href="places.html">Places</a>
        <a href="timeline.html">Timeline</a>
        <a href="statistics.html">Statistics</a>
        <a href="sources.html">Sources</a>
    </nav>
    <div class="container">
        <div class="section search-container">
            <input type="text" id="searchBox" class="search-box" placeholder="Search for people by name, date, or place..." autofocus>
            <div id="searchStats" class="search-stats"></div>
            <div id="searchResults" class="search-results"></div>
        </div>
        <div class="footer">
            <p>Generated by KrankyBear Genealogy</p>
        </div>
    </div>

    <script>
        // Embedded people data (avoids CORS issues with local files)
        const allPeople = ` + peopleJSON + `;
        
        // Initialize on page load
        document.addEventListener('DOMContentLoaded', function() {
            console.log('Loaded ' + allPeople.length + ' people');
            document.getElementById('searchStats').textContent = 'Ready to search ' + allPeople.length + ' people';
        });

        // Search function
        function searchPeople(query) {
            if (!query || query.trim().length < 2) {
                document.getElementById('searchResults').innerHTML = '';
                document.getElementById('searchStats').textContent = 'Ready to search ' + allPeople.length + ' people';
                return;
            }

            query = query.toLowerCase().trim();
            const results = allPeople.filter(person => {
                return (
                    person.name.toLowerCase().includes(query) ||
                    person.given_name.toLowerCase().includes(query) ||
                    person.surname.toLowerCase().includes(query) ||
                    (person.birth_date && person.birth_date.toLowerCase().includes(query)) ||
                    (person.birth_place && person.birth_place.toLowerCase().includes(query)) ||
                    (person.death_date && person.death_date.toLowerCase().includes(query)) ||
                    (person.death_place && person.death_place.toLowerCase().includes(query))
                );
            });

            displayResults(results, query);
        }

        // Display results
        function displayResults(results, query) {
            const resultsDiv = document.getElementById('searchResults');
            const statsDiv = document.getElementById('searchStats');

            if (results.length === 0) {
                resultsDiv.innerHTML = '<div class="no-results">No results found for "' + escapeHtml(query) + '"</div>';
                statsDiv.textContent = '0 results';
                return;
            }

            statsDiv.textContent = results.length + ' result' + (results.length === 1 ? '' : 's') + ' found';

            let html = '';
            results.forEach(person => {
                let details = [];
                
                if (person.birth_date) {
                    details.push('Born: ' + person.birth_date);
                }
                if (person.birth_place) {
                    details.push(person.birth_place);
                }
                if (person.death_date) {
                    details.push('Died: ' + person.death_date);
                }
                if (person.death_place && person.death_place !== person.birth_place) {
                    details.push(person.death_place);
                }
                if (person.is_living) {
                    details.push('Living');
                }

                html += '<div class="search-result-item">';
                html += '<div class="search-result-name"><a href="person_' + person.id + '.html">' + escapeHtml(person.name) + '</a></div>';
                if (details.length > 0) {
                    html += '<div class="search-result-details">' + escapeHtml(details.join(' • ')) + '</div>';
                }
                html += '</div>';
            });

            resultsDiv.innerHTML = html;
        }

        // Escape HTML to prevent XSS
        function escapeHtml(text) {
            const div = document.createElement('div');
            div.textContent = text;
            return div.innerHTML;
        }

        // Add event listener for search box
        const searchBox = document.getElementById('searchBox');
        let searchTimeout;
        searchBox.addEventListener('input', function(e) {
            clearTimeout(searchTimeout);
            searchTimeout = setTimeout(() => {
                searchPeople(e.target.value);
            }, 300); // Debounce for 300ms
        });

        // Handle Enter key
        searchBox.addEventListener('keypress', function(e) {
            if (e.key === 'Enter') {
                clearTimeout(searchTimeout);
                searchPeople(e.target.value);
            }
        });
    </script>
</body>
</html>`
}

// normalizeDateForSort converts various date formats to YYYY-MM-DD for sorting
func normalizeDateForSort(dateStr string) string {
	if dateStr == "" {
		return "9999-99-99" // Put empty dates at end
	}
	
	// If already in YYYY-MM-DD format (or close), return as-is
	if len(dateStr) >= 10 && dateStr[4] == '-' && dateStr[7] == '-' {
		return dateStr
	}
	
	// Try parsing common formats
	layouts := []string{
		"2006-01-02",
		"02 Jan 2006",
		"Jan 2006",
		"2006",
	}
	
	for _, layout := range layouts {
		if t, err := time.Parse(layout, dateStr); err == nil {
			return t.Format("2006-01-02")
		}
	}
	
	// If we can't parse, return as-is
	return dateStr
}

// generatePersonPage creates an individual person page
func generatePersonPage(s *store.Store, person *store.Person, options HTMLExportOptions) string {
	var html strings.Builder

	html.WriteString(`<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>` + formatPersonName(*person) + `</title>
    <link rel="stylesheet" href="style.css">
</head>
<body>
    <div class="header">
        <h1>` + formatPersonName(*person) + `</h1>
    </div>
    <nav class="nav">
        <a href="index.html">Home</a>
        <a href="search.html">Search</a>
        <a href="surnames.html">Surnames</a>
        <a href="places.html">Places</a>
        <a href="timeline.html">Timeline</a>
        <a href="statistics.html">Statistics</a>
        <a href="sources.html">Sources</a>
    </nav>
    <div class="breadcrumb">
        <a href="index.html">Home</a> &raquo; 
        <a href="surnames.html">Surnames</a> &raquo; 
        <a href="surnames.html#` + person.Surname + `">` + person.Surname + `</a> &raquo; 
        <span>` + formatPersonName(*person) + `</span>
    </div>
    <div class="container">
`)

	// Personal Details
	html.WriteString(`        <div class="section">
            <h2>Personal Information</h2>
            <div class="person-details">
`)
	if options.LimitLivingInfo && person.IsLiving {
		html.WriteString(`                <div class="detail-row"><span class="detail-label">Status:</span> Living (limited information)</div>
`)
	} else {
		if person.BirthDate != "" {
			html.WriteString(`                <div class="detail-row"><span class="detail-label">Born:</span> ` + formatFGSDate(person.BirthDate) + `</div>
`)
		}
		if person.BirthPlace != "" {
			html.WriteString(`                <div class="detail-row"><span class="detail-label">Birth Place:</span> ` + person.BirthPlace + `</div>
`)
		}
		if !person.IsLiving {
			if person.DeathDate != "" {
				html.WriteString(`                <div class="detail-row"><span class="detail-label">Died:</span> ` + formatFGSDate(person.DeathDate) + `</div>
`)
			}
			if person.DeathPlace != "" {
				html.WriteString(`                <div class="detail-row"><span class="detail-label">Death Place:</span> ` + person.DeathPlace + `</div>
`)
			}
		}
	}
	html.WriteString(`            </div>
        </div>
`)

	// Photo Gallery
	photoGalleryHTML := generatePhotoGalleryHTML(s, person.ID)
	if photoGalleryHTML != "" {
		html.WriteString(`        <div class="section">
            <h2>Photos</h2>
`)
		html.WriteString(photoGalleryHTML)
		html.WriteString(`        </div>
`)
	}

	// Parents
	parents, _ := s.GetRelatedPeople(person.ID, "parent")
	if len(parents) > 0 {
		html.WriteString(`        <div class="section">
            <h2>Parents</h2>
            <ul class="person-list">
`)
		for _, parent := range parents {
			if shouldShowPerson(&parent, options) {
				html.WriteString(`                <li><a href="person_` + fmt.Sprintf("%d", parent.ID) + `.html">` + formatPersonName(parent) + `</a></li>
`)
			}
		}
		html.WriteString(`            </ul>
        </div>
`)
	}

	// Spouses
	spouses, _ := s.GetRelatedPeople(person.ID, "spouse")
	if len(spouses) > 0 {
		filteredSpouses := []store.Person{}
		for _, sp := range spouses {
			if shouldShowPerson(&sp, options) {
				filteredSpouses = append(filteredSpouses, sp)
			}
		}
		if len(filteredSpouses) > 0 {
			html.WriteString(`        <div class="section">
            <h2>Spouses</h2>
            <ul class="person-list">
`)
			for _, spouse := range filteredSpouses {
				html.WriteString(`                <li><a href="person_` + fmt.Sprintf("%d", spouse.ID) + `.html">` + formatPersonName(spouse) + `</a></li>
`)
			}
			html.WriteString(`            </ul>
        </div>
`)
		}
	}

	// Children
	children, _ := s.GetRelatedPeople(person.ID, "child")
	if len(children) > 0 {
		filteredChildren := []store.Person{}
		for _, child := range children {
			if shouldShowPerson(&child, options) {
				filteredChildren = append(filteredChildren, child)
			}
		}
		if len(filteredChildren) > 0 {
			html.WriteString(`        <div class="section">
            <h2>Children</h2>
            <ul class="person-list">
`)
			for _, child := range filteredChildren {
				html.WriteString(`                <li><a href="person_` + fmt.Sprintf("%d", child.ID) + `.html">` + formatPersonName(child) + `</a></li>
`)
			}
			html.WriteString(`            </ul>
        </div>
`)
		}
	}

	// Sources & Citations
	citations, err := s.GetCitationsForPerson(person.ID)
	if err == nil && len(citations) > 0 {
		html.WriteString(`        <div class="section">
            <h2>Sources & Citations</h2>
            <div class="person-citations">
`)
		for _, cit := range citations {
			source, err := s.GetSourceByID(cit.SourceID)
			if err != nil {
				continue
			}
			
			html.WriteString(`                <div class="person-citation-item">
                    <div class="citation-source-title">` + source.Title + `</div>
`)
			if cit.CitationDetail != "" {
				html.WriteString(`                    <div class="citation-detail">` + cit.CitationDetail + `</div>
`)
			}
			if cit.Transcription != "" {
				html.WriteString(`                    <div class="citation-transcription">"` + cit.Transcription + `"</div>
`)
			}
			if cit.Confidence != "" {
				confidenceClass := "confidence-" + cit.Confidence
				html.WriteString(`                    <span class="citation-confidence ` + confidenceClass + `">Confidence: ` + cit.Confidence + `</span>
`)
			}
			if cit.Notes != "" {
				html.WriteString(`                    <div class="citation-notes">` + cit.Notes + `</div>
`)
			}
			html.WriteString(`                </div>
`)
		}
		html.WriteString(`            </div>
        </div>
`)
	}

	html.WriteString(`        <div class="footer">
            <p>Generated by KrankyBear Genealogy on ` + time.Now().Format("January 2, 2006") + `</p>
        </div>
    </div>
</body>
</html>`)

	return html.String()
}

// exportFamilyGroupSheetPDF exports the Family Group Sheet to PDF format
func exportFamilyGroupSheetPDF(w fyne.Window, s *store.Store, personID int64) {
	if personID <= 0 {
		dialog.ShowInformation("Export Family Group Sheet", "Please select a person first", w)
		return
	}

	person, err := s.GetPersonByID(personID)
	if err != nil {
		dialog.ShowError(fmt.Errorf("Failed to load person: %w", err), w)
		return
	}

	// Show privacy options dialog
	showHTMLExportOptionsDialog(w, person, func(options HTMLExportOptions) {
		// Show file save dialog
		fd := dialog.NewFileSave(func(uc fyne.URIWriteCloser, err error) {
			if err != nil || uc == nil {
				return
			}
			defer uc.Close()

			// Generate PDF content with privacy options
			err = generateFamilyGroupSheetPDF(s, person, options, uc.URI().Path())
			if err != nil {
				dialog.ShowError(fmt.Errorf("Failed to generate PDF: %w", err), w)
				return
			}

			dialog.ShowInformation("Export Successful",
				fmt.Sprintf("Family Group Sheet exported to:\n%s", uc.URI().Path()), w)
		}, w)

		// Set default filename
		defaultName := fmt.Sprintf("FamilyGroup_%s_%s.pdf",
			strings.ReplaceAll(person.Surname, " ", "_"),
			strings.ReplaceAll(person.GivenName, " ", "_"))
		fd.SetFileName(defaultName)
		fd.Show()
	})
}

// generateFamilyGroupSheetPDF creates a PDF document for the family group sheet
func generateFamilyGroupSheetPDF(s *store.Store, person *store.Person, options HTMLExportOptions, filepath string) error {
	pdf := gofpdf.New("P", "mm", "Letter", "")
	pdf.AddPage()
	
	// Title
	pdf.SetFont("Arial", "B", 16)
	pdf.CellFormat(0, 10, "Family Group Sheet", "", 1, "C", false, 0, "")
	pdf.Ln(2)
	
	// Main person name
	pdf.SetFont("Arial", "B", 14)
	pdf.CellFormat(0, 8, formatPersonName(*person), "", 1, "C", false, 0, "")
	pdf.Ln(3)
	
	// Get spouse(s)
	spouseInfos, _ := s.GetSpouses(person.ID)
	
	if len(spouseInfos) > 0 {
		// Sort spouses chronologically
		sort.Slice(spouseInfos, func(i, j int) bool {
			return spouseInfos[i].MarriageDate < spouseInfos[j].MarriageDate
		})
		
		// Show each marriage
		for idx, spouseInfo := range spouseInfos {
			if idx > 0 {
				pdf.AddPage()
			}
			
			spouse := spouseInfo.Spouse
			
			// Husband section
			pdf.SetFont("Arial", "B", 12)
			pdf.SetFillColor(52, 152, 219) // Blue
			pdf.SetTextColor(255, 255, 255)
			pdf.CellFormat(0, 8, "Husband", "", 1, "L", true, 0, "")
			pdf.SetTextColor(0, 0, 0)
			pdf.SetFont("Arial", "", 10)
			
			husband := person
			wife := &spouse
			if person.Gender == "F" {
				husband = &spouse
				wife = person
			}
			
			addPersonDetailsToPDF(pdf, husband, options, "husband")
			pdf.Ln(2)
			
			// Wife section
			pdf.SetFont("Arial", "B", 12)
			pdf.SetFillColor(231, 76, 60) // Red/Pink
			pdf.SetTextColor(255, 255, 255)
			pdf.CellFormat(0, 8, "Wife", "", 1, "L", true, 0, "")
			pdf.SetTextColor(0, 0, 0)
			pdf.SetFont("Arial", "", 10)
			
			addPersonDetailsToPDF(pdf, wife, options, "wife")
			pdf.Ln(2)
			
			// Marriage information
			pdf.SetFont("Arial", "B", 12)
			pdf.SetFillColor(46, 204, 113) // Green
			pdf.SetTextColor(255, 255, 255)
			pdf.CellFormat(0, 8, "Marriage", "", 1, "L", true, 0, "")
			pdf.SetTextColor(0, 0, 0)
			pdf.SetFont("Arial", "", 10)
			
			// Check if both are living for privacy
			bothLiving := husband.IsLiving && wife.IsLiving
			if options.LimitLivingInfo && bothLiving {
				pdf.CellFormat(0, 6, "(Marriage information withheld for privacy - both spouses living)", "", 1, "L", false, 0, "")
			} else {
				if spouseInfo.MarriageDate != "" {
					pdf.CellFormat(60, 6, "Marriage Date:", "", 0, "L", false, 0, "")
					pdf.CellFormat(0, 6, formatFGSDate(spouseInfo.MarriageDate), "", 1, "L", false, 0, "")
				}
				if spouseInfo.MarriagePlace != "" {
					pdf.CellFormat(60, 6, "Marriage Place:", "", 0, "L", false, 0, "")
					pdf.MultiCell(0, 6, spouseInfo.MarriagePlace, "", "L", false)
				}
			}
			pdf.Ln(2)
			
			// Children section
			pdf.SetFont("Arial", "B", 12)
			pdf.SetFillColor(155, 89, 182) // Purple
			pdf.SetTextColor(255, 255, 255)
			pdf.CellFormat(0, 8, "Children", "", 1, "L", true, 0, "")
			pdf.SetTextColor(0, 0, 0)
			pdf.SetFont("Arial", "", 10)
			
			// Get children
			children, _ := s.GetRelatedPeople(person.ID, "child")
			
			// Filter children by this spouse
			var marriageChildren []store.Person
			for _, child := range children {
				parents, _ := s.GetRelatedPeople(child.ID, "parent")
				for _, parent := range parents {
					if parent.ID == spouse.ID {
						marriageChildren = append(marriageChildren, child)
						break
					}
				}
			}
			
			// Sort children by birth date
			sort.Slice(marriageChildren, func(i, j int) bool {
				return marriageChildren[i].BirthDate < marriageChildren[j].BirthDate
			})
			
			if len(marriageChildren) == 0 {
				pdf.CellFormat(0, 6, "No children recorded", "", 1, "L", false, 0, "")
			} else {
				for childIdx, child := range marriageChildren {
					if shouldShowPerson(&child, options) {
						pdf.SetFont("Arial", "B", 10)
						pdf.CellFormat(0, 6, fmt.Sprintf("%d. %s", childIdx+1, formatPersonName(child)), "", 1, "L", false, 0, "")
						pdf.SetFont("Arial", "", 9)
						
						details := formatPersonDetailsPDF(&child, options)
						if details != "" {
							pdf.CellFormat(10, 5, "", "", 0, "L", false, 0, "")
							pdf.MultiCell(0, 5, details, "", "L", false)
						}
					}
				}
			}
		}
	} else {
		// Show parental family
		parents, _ := s.GetRelatedPeople(person.ID, "parent")
		
		pdf.SetFont("Arial", "B", 12)
		pdf.SetFillColor(52, 152, 219)
		pdf.SetTextColor(255, 255, 255)
		pdf.CellFormat(0, 8, "Parents", "", 1, "L", true, 0, "")
		pdf.SetTextColor(0, 0, 0)
		pdf.SetFont("Arial", "", 10)
		
		if len(parents) == 0 {
			pdf.CellFormat(0, 6, "No parents recorded", "", 1, "L", false, 0, "")
		} else {
			for _, parent := range parents {
				if shouldShowPerson(&parent, options) {
					pdf.SetFont("Arial", "B", 10)
					pdf.CellFormat(0, 6, formatPersonName(parent), "", 1, "L", false, 0, "")
					pdf.SetFont("Arial", "", 9)
					details := formatPersonDetailsPDF(&parent, options)
					if details != "" {
						pdf.CellFormat(10, 5, "", "", 0, "L", false, 0, "")
						pdf.MultiCell(0, 5, details, "", "L", false)
					}
				}
			}
		}
		
		pdf.Ln(2)
		
		// Siblings
		siblings, _ := s.GetRelatedPeople(person.ID, "sibling")
		if len(siblings) > 0 {
			pdf.SetFont("Arial", "B", 12)
			pdf.SetFillColor(155, 89, 182)
			pdf.SetTextColor(255, 255, 255)
			pdf.CellFormat(0, 8, "Siblings", "", 1, "L", true, 0, "")
			pdf.SetTextColor(0, 0, 0)
			pdf.SetFont("Arial", "", 10)
			
			for _, sibling := range siblings {
				if shouldShowPerson(&sibling, options) {
					pdf.CellFormat(0, 6, formatPersonName(sibling), "", 1, "L", false, 0, "")
				}
			}
		}
	}
	
	// Save PDF
	return pdf.OutputFileAndClose(filepath)
}

// formatPersonDetailsPDF returns plain text person details for PDF
func formatPersonDetailsPDF(person *store.Person, options HTMLExportOptions) string {
	if options.LimitLivingInfo && person.IsLiving {
		return "(Living)"
	}
	
	var details []string
	if person.BirthDate != "" {
		birthStr := "Born: " + formatFGSDate(person.BirthDate)
		if person.BirthPlace != "" {
			birthStr += " in " + person.BirthPlace
		}
		details = append(details, birthStr)
	}
	
	if !person.IsLiving {
		if person.DeathDate != "" {
			deathStr := "Died: " + formatFGSDate(person.DeathDate)
			if person.DeathPlace != "" {
				deathStr += " in " + person.DeathPlace
			}
			details = append(details, deathStr)
		}
	}
	
	if len(details) == 0 && person.IsLiving {
		return "(Living)"
	}
	
	return strings.Join(details, "; ")
}

// addPersonDetailsToPDF adds person details to PDF with privacy handling
func addPersonDetailsToPDF(pdf *gofpdf.Fpdf, person *store.Person, options HTMLExportOptions, role string) {
	pdf.CellFormat(60, 6, "Full Name:", "", 0, "L", false, 0, "")
	pdf.CellFormat(0, 6, formatPersonName(*person), "", 1, "L", false, 0, "")
	
	if options.LimitLivingInfo && person.IsLiving {
		pdf.CellFormat(0, 6, "(Limited information - person is living)", "", 1, "L", false, 0, "")
	} else {
		if person.BirthDate != "" {
			pdf.CellFormat(60, 6, "Birth Date:", "", 0, "L", false, 0, "")
			pdf.CellFormat(0, 6, formatFGSDate(person.BirthDate), "", 1, "L", false, 0, "")
		}
		if person.BirthPlace != "" {
			pdf.CellFormat(60, 6, "Birth Place:", "", 0, "L", false, 0, "")
			pdf.MultiCell(0, 6, person.BirthPlace, "", "L", false)
		}
		if !person.IsLiving {
			if person.DeathDate != "" {
				pdf.CellFormat(60, 6, "Death Date:", "", 0, "L", false, 0, "")
				pdf.CellFormat(0, 6, formatFGSDate(person.DeathDate), "", 1, "L", false, 0, "")
			}
			if person.DeathPlace != "" {
				pdf.CellFormat(60, 6, "Death Place:", "", 0, "L", false, 0, "")
				pdf.MultiCell(0, 6, person.DeathPlace, "", "L", false)
			}
		}
	}
}

// exportFamilyGroupSheetHTML exports the Family Group Sheet to HTML format
func exportFamilyGroupSheetHTML(w fyne.Window, s *store.Store, personID int64) {
	if personID <= 0 {
		dialog.ShowInformation("Export Family Group Sheet", "Please select a person first", w)
		return
	}

	person, err := s.GetPersonByID(personID)
	if err != nil {
		dialog.ShowError(fmt.Errorf("Failed to load person: %w", err), w)
		return
	}

	// Show privacy options dialog
	showHTMLExportOptionsDialog(w, person, func(options HTMLExportOptions) {
		// Show file save dialog
		fd := dialog.NewFileSave(func(uc fyne.URIWriteCloser, err error) {
			if err != nil || uc == nil {
				return
			}
			defer uc.Close()

			// Generate HTML content with privacy options
			html := generateFamilyGroupSheetHTML(s, person, options)

			// Write to file
			_, err = uc.Write([]byte(html))
			if err != nil {
				dialog.ShowError(fmt.Errorf("Failed to write HTML file: %w", err), w)
				return
			}

			dialog.ShowInformation("Export Successful",
				fmt.Sprintf("Family Group Sheet exported to:\n%s", uc.URI().Path()), w)
		}, w)

		// Set default filename
		defaultName := fmt.Sprintf("FamilyGroup_%s_%s.html",
			strings.ReplaceAll(person.Surname, " ", "_"),
			strings.ReplaceAll(person.GivenName, " ", "_"))
		fd.SetFileName(defaultName)
		fd.Show()
	})
}

// shouldShowPerson returns true if the person should be included in export based on privacy options
func shouldShowPerson(person *store.Person, options HTMLExportOptions) bool {
	if options.HideLiving && person.IsLiving {
		return false
	}
	return true
}

// exportPeopleToCSV exports all people to a CSV file
func exportPeopleToCSV(w fyne.Window, s *store.Store) {
	// Get all people
	allPeople, err := s.GetAllPeople()
	if err != nil {
		dialog.ShowError(fmt.Errorf("Failed to get people: %w", err), w)
		return
	}

	// Show file save dialog
	fd := dialog.NewFileSave(func(uc fyne.URIWriteCloser, err error) {
		if err != nil || uc == nil {
			return
		}
		defer uc.Close()

		// Create CSV writer
		writer := csv.NewWriter(uc)
		defer writer.Flush()

		// Write header
		header := []string{"ID", "Given Name", "Surname", "Gender", "Birth Date", "Birth Place", "Death Date", "Death Place", "Is Living"}
		if err := writer.Write(header); err != nil {
			dialog.ShowError(fmt.Errorf("Failed to write CSV header: %w", err), w)
			return
		}

		// Write data
		for _, person := range allPeople {
			livingStr := "No"
			if person.IsLiving {
				livingStr = "Yes"
			}
			
			row := []string{
				fmt.Sprintf("%d", person.ID),
				person.GivenName,
				person.Surname,
				person.Gender,
				person.BirthDate,
				person.BirthPlace,
				person.DeathDate,
				person.DeathPlace,
				livingStr,
			}
			if err := writer.Write(row); err != nil {
				dialog.ShowError(fmt.Errorf("Failed to write CSV row: %w", err), w)
				return
			}
		}

		dialog.ShowInformation("Export Successful",
			fmt.Sprintf("Exported %d people to:\n%s", len(allPeople), uc.URI().Path()), w)
	}, w)

	fd.SetFileName("people_export.csv")
	fd.Show()
}

// exportTimelineToCSV exports timeline events to a CSV file
func exportTimelineToCSV(w fyne.Window, s *store.Store) {
	// Get all people
	allPeople, err := s.GetAllPeople()
	if err != nil {
		dialog.ShowError(fmt.Errorf("Failed to get people: %w", err), w)
		return
	}

	// Show file save dialog
	fd := dialog.NewFileSave(func(uc fyne.URIWriteCloser, err error) {
		if err != nil || uc == nil {
			return
		}
		defer uc.Close()

		// Create CSV writer
		writer := csv.NewWriter(uc)
		defer writer.Flush()

		// Write header
		header := []string{"Date", "Year", "Type", "Person ID", "Person Name", "Place"}
		if err := writer.Write(header); err != nil {
			dialog.ShowError(fmt.Errorf("Failed to write CSV header: %w", err), w)
			return
		}

		// Collect events
		eventCount := 0
		for _, p := range allPeople {
			// Birth events
			if p.BirthDate != "" {
				year := extractYear(p.BirthDate)
				row := []string{
					p.BirthDate,
					year,
					"Birth",
					fmt.Sprintf("%d", p.ID),
					formatPersonName(p),
					p.BirthPlace,
				}
				if err := writer.Write(row); err != nil {
					dialog.ShowError(fmt.Errorf("Failed to write CSV row: %w", err), w)
					return
				}
				eventCount++
			}
			
			// Death events
			if p.DeathDate != "" {
				year := extractYear(p.DeathDate)
				row := []string{
					p.DeathDate,
					year,
					"Death",
					fmt.Sprintf("%d", p.ID),
					formatPersonName(p),
					p.DeathPlace,
				}
				if err := writer.Write(row); err != nil {
					dialog.ShowError(fmt.Errorf("Failed to write CSV row: %w", err), w)
					return
				}
				eventCount++
			}
		}

		dialog.ShowInformation("Export Successful",
			fmt.Sprintf("Exported %d timeline events to:\n%s", eventCount, uc.URI().Path()), w)
	}, w)

	fd.SetFileName("timeline_export.csv")
	fd.Show()
}

// exportSurnamesToCSV exports surname statistics to a CSV file
func exportSurnamesToCSV(w fyne.Window, s *store.Store) {
	// Get all people
	allPeople, err := s.GetAllPeople()
	if err != nil {
		dialog.ShowError(fmt.Errorf("Failed to get people: %w", err), w)
		return
	}

	// Count surnames
	surnameMap := make(map[string]int)
	for _, p := range allPeople {
		if p.Surname != "" {
			surnameMap[p.Surname]++
		}
	}

	// Sort by count
	type SurnameCount struct {
		Name  string
		Count int
	}
	var surnames []SurnameCount
	for name, count := range surnameMap {
		surnames = append(surnames, SurnameCount{Name: name, Count: count})
	}
	sort.Slice(surnames, func(i, j int) bool {
		if surnames[i].Count == surnames[j].Count {
			return surnames[i].Name < surnames[j].Name
		}
		return surnames[i].Count > surnames[j].Count
	})

	// Show file save dialog
	fd := dialog.NewFileSave(func(uc fyne.URIWriteCloser, err error) {
		if err != nil || uc == nil {
			return
		}
		defer uc.Close()

		// Create CSV writer
		writer := csv.NewWriter(uc)
		defer writer.Flush()

		// Write header
		header := []string{"Surname", "Count"}
		if err := writer.Write(header); err != nil {
			dialog.ShowError(fmt.Errorf("Failed to write CSV header: %w", err), w)
			return
		}

		// Write data
		for _, sn := range surnames {
			row := []string{
				sn.Name,
				fmt.Sprintf("%d", sn.Count),
			}
			if err := writer.Write(row); err != nil {
				dialog.ShowError(fmt.Errorf("Failed to write CSV row: %w", err), w)
				return
			}
		}

		dialog.ShowInformation("Export Successful",
			fmt.Sprintf("Exported %d surnames to:\n%s", len(surnames), uc.URI().Path()), w)
	}, w)

	fd.SetFileName("surnames_export.csv")
	fd.Show()
}

// exportPlacesToCSV exports place statistics to a CSV file
func exportPlacesToCSV(w fyne.Window, s *store.Store) {
	// Get all people
	allPeople, err := s.GetAllPeople()
	if err != nil {
		dialog.ShowError(fmt.Errorf("Failed to get people: %w", err), w)
		return
	}

	// Count places
	placeMap := make(map[string]struct {
		BirthCount int
		DeathCount int
	})
	
	for _, p := range allPeople {
		if p.BirthPlace != "" {
			entry := placeMap[p.BirthPlace]
			entry.BirthCount++
			placeMap[p.BirthPlace] = entry
		}
		if p.DeathPlace != "" {
			entry := placeMap[p.DeathPlace]
			entry.DeathCount++
			placeMap[p.DeathPlace] = entry
		}
	}

	// Sort by total count
	type PlaceCount struct {
		Name       string
		BirthCount int
		DeathCount int
		TotalCount int
	}
	var places []PlaceCount
	for name, counts := range placeMap {
		places = append(places, PlaceCount{
			Name:       name,
			BirthCount: counts.BirthCount,
			DeathCount: counts.DeathCount,
			TotalCount: counts.BirthCount + counts.DeathCount,
		})
	}
	sort.Slice(places, func(i, j int) bool {
		if places[i].TotalCount == places[j].TotalCount {
			return places[i].Name < places[j].Name
		}
		return places[i].TotalCount > places[j].TotalCount
	})

	// Show file save dialog
	fd := dialog.NewFileSave(func(uc fyne.URIWriteCloser, err error) {
		if err != nil || uc == nil {
			return
		}
		defer uc.Close()

		// Create CSV writer
		writer := csv.NewWriter(uc)
		defer writer.Flush()

		// Write header
		header := []string{"Place", "Birth Count", "Death Count", "Total Count"}
		if err := writer.Write(header); err != nil {
			dialog.ShowError(fmt.Errorf("Failed to write CSV header: %w", err), w)
			return
		}

		// Write data
		for _, pl := range places {
			row := []string{
				pl.Name,
				fmt.Sprintf("%d", pl.BirthCount),
				fmt.Sprintf("%d", pl.DeathCount),
				fmt.Sprintf("%d", pl.TotalCount),
			}
			if err := writer.Write(row); err != nil {
				dialog.ShowError(fmt.Errorf("Failed to write CSV row: %w", err), w)
				return
			}
		}

		dialog.ShowInformation("Export Successful",
			fmt.Sprintf("Exported %d places to:\n%s", len(places), uc.URI().Path()), w)
	}, w)

	fd.SetFileName("places_export.csv")
	fd.Show()
}

// exportSourcesToCSV exports sources and citations to a CSV file
func exportSourcesToCSV(w fyne.Window, s *store.Store) {
	// Get all sources
	sources, err := s.GetAllSources()
	if err != nil {
		dialog.ShowError(fmt.Errorf("Failed to get sources: %w", err), w)
		return
	}

	// Show file save dialog
	fd := dialog.NewFileSave(func(uc fyne.URIWriteCloser, err error) {
		if err != nil || uc == nil {
			return
		}
		defer uc.Close()

		// Create CSV writer
		writer := csv.NewWriter(uc)
		defer writer.Flush()

		// Write header
		header := []string{"Source ID", "Title", "Author", "Publication", "Repository", "Call Number", "Type", "Citation Count"}
		if err := writer.Write(header); err != nil {
			dialog.ShowError(fmt.Errorf("Failed to write CSV header: %w", err), w)
			return
		}

		// Write data
		for _, source := range sources {
			// Get citation count
			citations, _ := s.GetCitationsForSource(source.ID)
			citationCount := len(citations)
			
			row := []string{
				fmt.Sprintf("%d", source.ID),
				source.Title,
				source.Author,
				source.Publication,
				source.Repository,
				source.CallNumber,
				source.SourceType,
				fmt.Sprintf("%d", citationCount),
			}
			if err := writer.Write(row); err != nil {
				dialog.ShowError(fmt.Errorf("Failed to write CSV row: %w", err), w)
				return
			}
		}

		dialog.ShowInformation("Export Successful",
			fmt.Sprintf("Exported %d sources to:\n%s", len(sources), uc.URI().Path()), w)
	}, w)

	fd.SetFileName("sources_export.csv")
	fd.Show()
}

// encodeImageAsDataURI converts image bytes to a base64 data URI
func encodeImageAsDataURI(imageData []byte, mimeType string) string {
	encoded := base64.StdEncoding.EncodeToString(imageData)
	return fmt.Sprintf("data:%s;base64,%s", mimeType, encoded)
}

// generatePhotoGalleryHTML creates HTML for a person's photo gallery
func generatePhotoGalleryHTML(s *store.Store, personID int64) string {
	// Get media list (without BLOB data for performance)
	mediaList, err := s.GetMediaForPerson(personID)
	if err != nil || len(mediaList) == 0 {
		return ""
	}

	// Filter to only images and load full data including BLOBs
	var images []store.Media
	for _, m := range mediaList {
		if m.MediaType == "image" {
			// Load the full media data including BLOB images
			fullMedia, err := s.GetMediaByID(m.ID)
			if err == nil && fullMedia != nil {
				images = append(images, *fullMedia)
			}
		}
	}

	if len(images) == 0 {
		return ""
	}

	var html strings.Builder
	html.WriteString(`<div class="photo-gallery">`)
	
	for _, img := range images {
		var imageData []byte
		var mimeType string
		
		if img.IsExternal {
			// Handle external images - read from file path
			if img.ExternalPath != "" {
				fileData, err := os.ReadFile(img.ExternalPath)
				if err == nil {
					imageData = fileData
					mimeType = img.MimeType
				}
			}
		} else {
			// Handle embedded images - use BLOB data
			if len(img.FullImage) > 0 {
				imageData = img.FullImage
				mimeType = img.MimeType
			} else if len(img.Thumbnail) > 0 {
				imageData = img.Thumbnail
				mimeType = img.MimeType
			}
		}
		
		if len(imageData) > 0 {
			dataURI := encodeImageAsDataURI(imageData, mimeType)
			html.WriteString(`<div class="photo-item">`)
			html.WriteString(`<img src="` + dataURI + `" alt="` + img.Title + `">`)
			if img.Title != "" {
				html.WriteString(`<div class="photo-caption">` + img.Title + `</div>`)
			}
			if img.DateTaken != "" {
				html.WriteString(`<div class="photo-date">` + formatFGSDate(img.DateTaken) + `</div>`)
			}
			html.WriteString(`</div>`)
		}
	}
	
	html.WriteString(`</div>`)
	return html.String()
}

// formatPersonDetailsHTML formats person details respecting privacy options
func formatPersonDetailsHTML(person *store.Person, options HTMLExportOptions, includeLabel bool) string {
	if !shouldShowPerson(person, options) {
		return ""
	}

	var html strings.Builder

	// If limiting info for living people, only show name
	if options.LimitLivingInfo && person.IsLiving {
		return `<div><span class="detail-label">Name:</span> ` + formatPersonName(*person) + ` <span style="color: #7f8c8d; font-style: italic;">(Living - limited information)</span></div>`
	}

	// Show full details
	if includeLabel {
		html.WriteString(`<div><span class="detail-label">Born:</span> ` + formatFGSDate(person.BirthDate) + `</div>`)
	} else {
		html.WriteString(`<div>` + formatFGSDate(person.BirthDate) + `</div>`)
	}
	if person.BirthPlace != "" {
		html.WriteString(`<div><span class="detail-label">Place:</span> ` + person.BirthPlace + `</div>`)
	}
	if !person.IsLiving {
		html.WriteString(`<div><span class="detail-label">Died:</span> ` + formatFGSDate(person.DeathDate) + `</div>`)
		if person.DeathPlace != "" {
			html.WriteString(`<div><span class="detail-label">Place:</span> ` + person.DeathPlace + `</div>`)
		}
	}
	return html.String()
}

// generateFamilyGroupSheetHTML generates the HTML content for the Family Group Sheet
func generateFamilyGroupSheetHTML(s *store.Store, person *store.Person, options HTMLExportOptions) string {
	var html strings.Builder

	// HTML header with embedded CSS
	html.WriteString(`<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Family Group Sheet - ` + formatPersonName(*person) + `</title>
    <style>
        * {
            margin: 0;
            padding: 0;
            box-sizing: border-box;
        }
        body {
            font-family: 'Segoe UI', Tahoma, Geneva, Verdana, sans-serif;
            line-height: 1.6;
            color: #333;
            background-color: #f5f5f5;
            padding: 20px;
        }
        .container {
            max-width: 900px;
            margin: 0 auto;
            background-color: white;
            padding: 40px;
            box-shadow: 0 2px 10px rgba(0,0,0,0.1);
            border-radius: 8px;
        }
        h1 {
            text-align: center;
            color: #2c3e50;
            margin-bottom: 30px;
            padding-bottom: 15px;
            border-bottom: 3px solid #3498db;
        }
        h2 {
            color: #34495e;
            margin-top: 30px;
            margin-bottom: 15px;
            font-size: 1.3em;
            background-color: #ecf0f1;
            padding: 10px;
            border-left: 4px solid #3498db;
        }
        .person-section {
            margin-bottom: 30px;
            padding: 20px;
            background-color: #fafafa;
            border-radius: 5px;
        }
        .person-name {
            font-size: 1.2em;
            font-weight: bold;
            color: #2c3e50;
            margin-bottom: 10px;
        }
        .person-details {
            margin-left: 20px;
            line-height: 1.8;
        }
        .detail-label {
            font-weight: 600;
            color: #555;
            display: inline-block;
            min-width: 120px;
        }
        .child-entry {
            margin: 20px 0;
            padding: 15px;
            background-color: white;
            border-left: 3px solid #95a5a6;
            border-radius: 3px;
        }
        .child-number {
            font-weight: bold;
            color: #3498db;
            margin-bottom: 8px;
        }
        .grandchildren {
            margin-left: 30px;
            margin-top: 10px;
            padding-top: 10px;
            border-top: 1px dashed #bdc3c7;
            color: #666;
            font-size: 0.95em;
        }
        .separator {
            margin: 40px 0;
            border: 0;
            border-top: 2px solid #ecf0f1;
        }
        .footer {
            margin-top: 40px;
            padding-top: 20px;
            border-top: 1px solid #ecf0f1;
            text-align: center;
            color: #7f8c8d;
            font-size: 0.9em;
        }
        .photo-gallery {
            display: grid;
            grid-template-columns: repeat(auto-fill, minmax(150px, 1fr));
            gap: 15px;
            margin: 20px 0;
        }
        .photo-item {
            background-color: white;
            border-radius: 5px;
            overflow: hidden;
            box-shadow: 0 2px 5px rgba(0,0,0,0.1);
        }
        .photo-item img {
            width: 100%;
            height: 150px;
            object-fit: cover;
            display: block;
        }
        .photo-caption {
            padding: 8px;
            font-weight: 600;
            color: #2c3e50;
            font-size: 0.9em;
        }
        .photo-date {
            padding: 0 8px 8px 8px;
            color: #7f8c8d;
            font-size: 0.8em;
        }
        @media print {
            body {
                background-color: white;
                padding: 0;
            }
            .container {
                box-shadow: none;
                max-width: 100%;
            }
            .photo-gallery {
                grid-template-columns: repeat(3, 1fr);
            }
            .photo-item img {
                height: 120px;
            }
        }
    </style>
</head>
<body>
    <div class="container">
        <h1>Family Group Sheet</h1>
        <h2 style="text-align: center; background-color: transparent; border: none;">` + formatPersonName(*person) + `</h2>
`)

	// Get spouse(s) with marriage information
	spouseInfos, _ := s.GetSpouses(person.ID)

	if len(spouseInfos) > 0 {
		// Sort spouses chronologically by marriage date
		sort.Slice(spouseInfos, func(i, j int) bool {
			return spouseInfos[i].MarriageDate < spouseInfos[j].MarriageDate
		})

		// Generate HTML for each marriage
		for idx, spouseInfo := range spouseInfos {
			if idx > 0 {
				html.WriteString(`<hr class="separator">`)
			}
			html.WriteString(generateMarriedFamilyHTML(s, person, &spouseInfo.Spouse, options))
		}
	} else {
		// Show person's parents and siblings
		parents, _ := s.GetRelatedPeople(person.ID, "parent")
		html.WriteString(generateParentalFamilyHTML(s, person, parents, options))
	}

	// Footer
	html.WriteString(`
        <div class="footer">
            <p>Generated by KrankyBear Genealogy on ` + time.Now().Format("January 2, 2006") + `</p>
        </div>
    </div>
</body>
</html>`)

	return html.String()
}

// generateMarriedFamilyHTML generates HTML for a married couple with children
func generateMarriedFamilyHTML(s *store.Store, person *store.Person, spouse *store.Person, options HTMLExportOptions) string {
	var html strings.Builder

	// Determine husband and wife
	var husband, wife *store.Person
	if person.Gender == "M" {
		husband = person
		wife = spouse
	} else {
		husband = spouse
		wife = person
	}

	// Husband section
	html.WriteString(`<div class="person-section">`)
	html.WriteString(`<h2>HUSBAND</h2>`)
	if husband != nil && shouldShowPerson(husband, options) {
		html.WriteString(`<div class="person-name">` + formatPersonName(*husband) + `</div>`)
		html.WriteString(`<div class="person-details">`)
		html.WriteString(formatPersonDetailsHTML(husband, options, true))
		html.WriteString(`</div>`)
		// Add photo gallery for husband
		photoGalleryHTML := generatePhotoGalleryHTML(s, husband.ID)
		if photoGalleryHTML != "" {
			html.WriteString(photoGalleryHTML)
		}
	} else if husband != nil {
		html.WriteString(`<div class="person-details" style="color: #7f8c8d; font-style: italic;">(Information withheld for privacy)</div>`)
	}
	html.WriteString(`</div>`)

	// Wife section
	html.WriteString(`<div class="person-section">`)
	html.WriteString(`<h2>WIFE</h2>`)
	if wife != nil && shouldShowPerson(wife, options) {
		html.WriteString(`<div class="person-name">` + formatPersonName(*wife) + `</div>`)
		html.WriteString(`<div class="person-details">`)
		html.WriteString(formatPersonDetailsHTML(wife, options, true))
		html.WriteString(`</div>`)
		// Add photo gallery for wife
		photoGalleryHTML := generatePhotoGalleryHTML(s, wife.ID)
		if photoGalleryHTML != "" {
			html.WriteString(photoGalleryHTML)
		}
	} else if wife != nil {
		html.WriteString(`<div class="person-details" style="color: #7f8c8d; font-style: italic;">(Information withheld for privacy)</div>`)
	}
	html.WriteString(`</div>`)

	// Marriage information
	html.WriteString(`<div class="person-section">`)
	html.WriteString(`<h2>MARRIAGE</h2>`)
	if husband != nil && wife != nil {
		// Check privacy settings - hide marriage details if both spouses are living and limited info is requested
		bothLiving := husband.IsLiving && wife.IsLiving
		if options.LimitLivingInfo && bothLiving {
			html.WriteString(`<div class="person-details" style="color: #7f8c8d; font-style: italic;">(Marriage information withheld for privacy - both spouses living)</div>`)
		} else {
			relationships, _ := s.GetRelationshipsBetween(husband.ID, wife.ID, "spouse")
			if len(relationships) > 0 {
				html.WriteString(`<div class="person-details">`)
				if relationships[0].MarriageDate != "" {
					html.WriteString(`<div><span class="detail-label">Date:</span> ` + formatFGSDate(relationships[0].MarriageDate) + `</div>`)
				}
				if relationships[0].MarriagePlace != "" {
					html.WriteString(`<div><span class="detail-label">Place:</span> ` + relationships[0].MarriagePlace + `</div>`)
				}
				html.WriteString(`</div>`)
			} else {
				html.WriteString(`<div class="person-details">(No marriage record)</div>`)
			}
		}
	}
	html.WriteString(`</div>`)

	// Children section
	html.WriteString(`<div class="person-section">`)
	html.WriteString(`<h2>CHILDREN</h2>`)

	// Get children of this specific couple
	var children []store.Person
	if husband != nil && wife != nil {
		husbandChildren, _ := s.GetRelatedPeople(husband.ID, "child")
		for _, child := range husbandChildren {
			parents, _ := s.GetRelatedPeople(child.ID, "parent")
			hasWifeAsParent := false
			for _, parent := range parents {
				if parent.ID == wife.ID {
					hasWifeAsParent = true
					break
				}
			}
			if hasWifeAsParent {
				children = append(children, child)
			}
		}
	} else if husband != nil {
		children, _ = s.GetRelatedPeople(husband.ID, "child")
	} else if wife != nil {
		children, _ = s.GetRelatedPeople(wife.ID, "child")
	}

	if len(children) == 0 {
		html.WriteString(`<div class="person-details">(No children recorded)</div>`)
	} else {
		// Sort children by birth date
		sort.Slice(children, func(i, j int) bool {
			return children[i].BirthDate < children[j].BirthDate
		})

		childNum := 0
		for _, child := range children {
			// Skip if privacy settings require hiding this child
			if !shouldShowPerson(&child, options) {
				continue
			}
			childNum++

			html.WriteString(`<div class="child-entry">`)
			html.WriteString(`<div class="child-number">` + fmt.Sprintf("%d.", childNum) + ` ` + formatPersonName(child) + `</div>`)
			html.WriteString(`<div class="person-details">`)
			
			// Handle privacy for child's details
			if options.LimitLivingInfo && child.IsLiving {
				html.WriteString(`<div style="color: #7f8c8d; font-style: italic;">(Living - limited information)</div>`)
			} else {
				html.WriteString(`<div><span class="detail-label">Born:</span> ` + formatFGSDate(child.BirthDate) + `</div>`)
				if child.BirthPlace != "" {
					html.WriteString(`<div><span class="detail-label">Place:</span> ` + child.BirthPlace + `</div>`)
				}

				// Show spouse if married
				childSpouses, _ := s.GetRelatedPeople(child.ID, "spouse")
				if len(childSpouses) > 0 {
					spouseNames := []string{}
					for _, sp := range childSpouses {
						if shouldShowPerson(&sp, options) {
							spouseNames = append(spouseNames, formatPersonName(sp))
						}
					}
					if len(spouseNames) > 0 {
						html.WriteString(`<div><span class="detail-label">Spouse:</span> ` + strings.Join(spouseNames, ", ") + `</div>`)
					}
				}

				// Show grandchildren
				grandchildren, _ := s.GetRelatedPeople(child.ID, "child")
				if len(grandchildren) > 0 {
					sort.Slice(grandchildren, func(a, b int) bool {
						return grandchildren[a].BirthDate < grandchildren[b].BirthDate
					})
					grandchildNames := []string{}
					for _, gc := range grandchildren {
						if shouldShowPerson(&gc, options) {
							if options.LimitLivingInfo && gc.IsLiving {
								grandchildNames = append(grandchildNames, formatPersonName(gc)+" (Living)")
							} else {
								grandchildNames = append(grandchildNames, fmt.Sprintf("%s (%s)", formatPersonName(gc), formatFGSDate(gc.BirthDate)))
							}
						}
					}
					if len(grandchildNames) > 0 {
						html.WriteString(`<div class="grandchildren"><span class="detail-label">Children:</span> ` + strings.Join(grandchildNames, ", ") + `</div>`)
					}
				}

				if !child.IsLiving {
					html.WriteString(`<div><span class="detail-label">Died:</span> ` + formatFGSDate(child.DeathDate) + `</div>`)
					if child.DeathPlace != "" {
						html.WriteString(`<div><span class="detail-label">Place:</span> ` + child.DeathPlace + `</div>`)
					}
				}
				
				// Add photo gallery for child
				photoGalleryHTML := generatePhotoGalleryHTML(s, child.ID)
				if photoGalleryHTML != "" {
					html.WriteString(photoGalleryHTML)
				}
			}
			html.WriteString(`</div>`)
			html.WriteString(`</div>`)
		}

		if childNum == 0 {
			html.WriteString(`<div class="person-details" style="color: #7f8c8d; font-style: italic;">(Children information withheld for privacy)</div>`)
		}
	}
	html.WriteString(`</div>`)

	return html.String()
}

// generateParentalFamilyHTML generates HTML for person's parental family
func generateParentalFamilyHTML(s *store.Store, person *store.Person, parents []store.Person, options HTMLExportOptions) string {
	var html strings.Builder

	// Show person's parents
	var father, mother *store.Person
	for i := range parents {
		if parents[i].Gender == "M" {
			father = &parents[i]
		} else {
			mother = &parents[i]
		}
	}

	if father != nil || mother != nil {
		html.WriteString(`<div class="person-section">`)
		html.WriteString(`<h2>PARENTS</h2>`)
		html.WriteString(`<div class="person-details">`)
		if father != nil && shouldShowPerson(father, options) {
			html.WriteString(`<div><span class="detail-label">Father:</span> ` + formatPersonName(*father) + `</div>`)
		}
		if mother != nil && shouldShowPerson(mother, options) {
			html.WriteString(`<div><span class="detail-label">Mother:</span> ` + formatPersonName(*mother) + `</div>`)
		}
		html.WriteString(`</div>`)
		html.WriteString(`</div>`)
	}

	// Show the person themselves
	html.WriteString(`<div class="person-section">`)
	html.WriteString(`<h2>PERSON</h2>`)
	html.WriteString(`<div class="person-name">` + formatPersonName(*person) + `</div>`)
	html.WriteString(`<div class="person-details">`)
	html.WriteString(formatPersonDetailsHTML(person, options, true))
	html.WriteString(`</div>`)
	// Add photo gallery for person
	photoGalleryHTML := generatePhotoGalleryHTML(s, person.ID)
	if photoGalleryHTML != "" {
		html.WriteString(photoGalleryHTML)
	}
	html.WriteString(`</div>`)

	// Show siblings
	html.WriteString(`<div class="person-section">`)
	html.WriteString(`<h2>SIBLINGS</h2>`)
	siblings := []store.Person{}

	if father != nil {
		fatherChildren, _ := s.GetRelatedPeople(father.ID, "child")
		for _, child := range fatherChildren {
			if child.ID != person.ID {
				siblings = append(siblings, child)
			}
		}
	} else if mother != nil {
		motherChildren, _ := s.GetRelatedPeople(mother.ID, "child")
		for _, child := range motherChildren {
			if child.ID != person.ID {
				siblings = append(siblings, child)
			}
		}
	}

	if len(siblings) == 0 {
		html.WriteString(`<div class="person-details">(No siblings recorded)</div>`)
	} else {
		// Remove duplicates
		uniqueSiblings := make(map[int64]store.Person)
		for _, sib := range siblings {
			uniqueSiblings[sib.ID] = sib
		}

		// Convert back to slice and sort
		siblings = []store.Person{}
		for _, sib := range uniqueSiblings {
			siblings = append(siblings, sib)
		}
		sort.Slice(siblings, func(i, j int) bool {
			return siblings[i].BirthDate < siblings[j].BirthDate
		})

		html.WriteString(`<div class="person-details">`)
		shownCount := 0
		for _, sibling := range siblings {
			if shouldShowPerson(&sibling, options) {
				if options.LimitLivingInfo && sibling.IsLiving {
					html.WriteString(`<div>` + formatPersonName(sibling) + ` (Living)</div>`)
				} else {
					html.WriteString(`<div>` + formatPersonName(sibling) + ` (` + formatFGSDate(sibling.BirthDate) + `)</div>`)
				}
				shownCount++
			}
		}
		if shownCount == 0 {
			html.WriteString(`<div style="color: #7f8c8d; font-style: italic;">(Sibling information withheld for privacy)</div>`)
		}
		html.WriteString(`</div>`)
	}
	html.WriteString(`</div>`)

	return html.String()
}

// exportDescendantReportPDF exports the Descendant Report to PDF format
func exportDescendantReportPDF(w fyne.Window, s *store.Store, rootPersonID int64) {
	if rootPersonID <= 0 {
		dialog.ShowInformation("Export Descendant Report", "Please select a person first", w)
		return
	}

	rootPerson, err := s.GetPersonByID(rootPersonID)
	if err != nil {
		dialog.ShowError(fmt.Errorf("Failed to load person: %w", err), w)
		return
	}

	// Show privacy options dialog
	showHTMLExportOptionsDialog(w, rootPerson, func(options HTMLExportOptions) {
		// Show file save dialog
		fd := dialog.NewFileSave(func(uc fyne.URIWriteCloser, err error) {
			if err != nil || uc == nil {
				return
			}
			defer uc.Close()

			// Generate PDF content with privacy options
			err = generateDescendantReportPDF(s, rootPerson, options, uc.URI().Path())
			if err != nil {
				dialog.ShowError(fmt.Errorf("Failed to generate PDF: %w", err), w)
				return
			}

			dialog.ShowInformation("Export Successful",
				fmt.Sprintf("Descendant Report exported to:\n%s", uc.URI().Path()), w)
		}, w)

		// Set default filename
		defaultName := fmt.Sprintf("Descendants_%s_%s.pdf",
			strings.ReplaceAll(rootPerson.Surname, " ", "_"),
			strings.ReplaceAll(rootPerson.GivenName, " ", "_"))
		fd.SetFileName(defaultName)
		fd.Show()
	})
}

// generateDescendantReportPDF creates a PDF document for the descendant report
func generateDescendantReportPDF(s *store.Store, rootPerson *store.Person, options HTMLExportOptions, filepath string) error {
	pdf := gofpdf.New("P", "mm", "Letter", "")
	pdf.AddPage()
	
	// Title
	pdf.SetFont("Arial", "B", 16)
	pdf.CellFormat(0, 10, "Descendant Report", "", 1, "C", false, 0, "")
	pdf.Ln(2)
	
	// Root person
	pdf.SetFont("Arial", "B", 14)
	pdf.CellFormat(0, 8, "Descendants of "+formatPersonName(*rootPerson), "", 1, "C", false, 0, "")
	pdf.Ln(3)
	
	// Collect descendants
	descendants := collectDescendants(s, rootPerson.ID, 1)
	
	// Filter based on privacy
	var filteredDescendants []DescendantNode
	for _, desc := range descendants {
		if shouldShowPerson(&desc.Person, options) {
			filteredDescendants = append(filteredDescendants, desc)
		}
	}
	
	// Count generations
	genCounts := make(map[int]int)
	for _, desc := range filteredDescendants {
		genCounts[desc.Generation]++
	}
	
	// Statistics
	pdf.SetFont("Arial", "", 10)
	pdf.CellFormat(0, 6, fmt.Sprintf("Total Descendants: %d", len(filteredDescendants)), "", 1, "L", false, 0, "")
	pdf.CellFormat(0, 6, fmt.Sprintf("Generations: %d", len(genCounts)), "", 1, "L", false, 0, "")
	pdf.Ln(3)
	
	// Render descendants
	for _, node := range filteredDescendants {
		// Check if we need a new page
		_, pageHeight := pdf.GetPageSize()
		_, y := pdf.GetXY()
		if y > pageHeight-30 {
			pdf.AddPage()
		}
		
		// Set indent based on generation
		indent := (node.Generation - 1) * 5
		leftMargin := 10.0 + float64(indent)
		pdf.SetLeftMargin(leftMargin)
		pdf.SetX(leftMargin)
		
		// Generation indicator
		pdf.SetFont("Arial", "B", 10)
		genLabel := fmt.Sprintf("Gen %d: ", node.Generation)
		pdf.Cell(20, 6, genLabel)
		
		// Person name
		pdf.SetFont("Arial", "", 10)
		personInfo := formatPersonName(node.Person)
		if !options.LimitLivingInfo || !node.Person.IsLiving {
			if node.Person.BirthDate != "" {
				personInfo += fmt.Sprintf(" (b. %s)", formatFGSDate(node.Person.BirthDate))
			}
		}
		if !node.Person.IsLiving && node.Person.DeathDate != "" {
			personInfo += fmt.Sprintf(" - (d. %s)", formatFGSDate(node.Person.DeathDate))
		} else if node.Person.IsLiving {
			personInfo += " (Living)"
		}
		pdf.MultiCell(0, 6, personInfo, "", "L", false)
		
		// Reset margin
		pdf.SetLeftMargin(10)
	}
	
	return pdf.OutputFileAndClose(filepath)
}

// exportDescendantReportHTML exports the Descendant Report to HTML format
func exportDescendantReportHTML(w fyne.Window, s *store.Store, rootPersonID int64) {
	if rootPersonID <= 0 {
		dialog.ShowInformation("Export Descendant Report", "Please select a person first", w)
		return
	}

	rootPerson, err := s.GetPersonByID(rootPersonID)
	if err != nil {
		dialog.ShowError(fmt.Errorf("Failed to load person: %w", err), w)
		return
	}

	// Show privacy options dialog
	showHTMLExportOptionsDialog(w, rootPerson, func(options HTMLExportOptions) {
		// Show file save dialog
		fd := dialog.NewFileSave(func(uc fyne.URIWriteCloser, err error) {
			if err != nil || uc == nil {
				return
			}
			defer uc.Close()

			// Generate HTML content with privacy options
			html := generateDescendantReportHTML(s, rootPerson, options)

			// Write to file
			_, err = uc.Write([]byte(html))
			if err != nil {
				dialog.ShowError(fmt.Errorf("Failed to write HTML file: %w", err), w)
				return
			}

			dialog.ShowInformation("Export Successful",
				fmt.Sprintf("Descendant Report exported to:\n%s", uc.URI().Path()), w)
		}, w)

		// Set default filename
		defaultName := fmt.Sprintf("Descendants_%s_%s.html",
			strings.ReplaceAll(rootPerson.Surname, " ", "_"),
			strings.ReplaceAll(rootPerson.GivenName, " ", "_"))
		fd.SetFileName(defaultName)
		fd.Show()
	})
}

// generateDescendantReportHTML generates the HTML content for the Descendant Report
func generateDescendantReportHTML(s *store.Store, rootPerson *store.Person, options HTMLExportOptions) string {
	var html strings.Builder

	// Build descendant tree
	descendants := collectDescendants(s, rootPerson.ID, 0)
	
	// Filter descendants based on privacy settings
	filteredDescendants := []DescendantNode{}
	for _, d := range descendants {
		if shouldShowPerson(&d.Person, options) {
			filteredDescendants = append(filteredDescendants, d)
		}
	}
	descendants = filteredDescendants

	// HTML header with embedded CSS
	html.WriteString(`<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Descendants of ` + formatPersonName(*rootPerson) + `</title>
    <style>
        * {
            margin: 0;
            padding: 0;
            box-sizing: border-box;
        }
        body {
            font-family: 'Segoe UI', Tahoma, Geneva, Verdana, sans-serif;
            line-height: 1.6;
            color: #333;
            background-color: #f5f5f5;
            padding: 20px;
        }
        .container {
            max-width: 900px;
            margin: 0 auto;
            background-color: white;
            padding: 40px;
            box-shadow: 0 2px 10px rgba(0,0,0,0.1);
            border-radius: 8px;
        }
        h1 {
            text-align: center;
            color: #2c3e50;
            margin-bottom: 30px;
            padding-bottom: 15px;
            border-bottom: 3px solid #3498db;
        }
        .summary {
            background-color: #ecf0f1;
            padding: 20px;
            border-radius: 5px;
            margin-bottom: 30px;
        }
        .summary-line {
            margin: 5px 0;
            font-size: 1.05em;
        }
        h2 {
            color: #34495e;
            margin-top: 30px;
            margin-bottom: 15px;
            font-size: 1.3em;
            background-color: #ecf0f1;
            padding: 10px;
            border-left: 4px solid #3498db;
        }
        .person-entry {
            margin: 10px 0;
            padding: 12px 15px;
            background-color: #fafafa;
            border-radius: 5px;
            border-left: 3px solid #95a5a6;
        }
        .person-name {
            font-weight: 600;
            color: #2c3e50;
        }
        .person-dates {
            color: #7f8c8d;
            font-size: 0.95em;
            margin-left: 10px;
        }
        .gen-1 { margin-left: 0; border-left-color: #e74c3c; }
        .gen-2 { margin-left: 30px; border-left-color: #e67e22; }
        .gen-3 { margin-left: 60px; border-left-color: #f39c12; }
        .gen-4 { margin-left: 90px; border-left-color: #27ae60; }
        .gen-5 { margin-left: 120px; border-left-color: #3498db; }
        .gen-6 { margin-left: 150px; border-left-color: #9b59b6; }
        .gen-7 { margin-left: 180px; border-left-color: #e91e63; }
        .footer {
            margin-top: 40px;
            padding-top: 20px;
            border-top: 1px solid #ecf0f1;
            text-align: center;
            color: #7f8c8d;
            font-size: 0.9em;
        }
        @media print {
            body {
                background-color: white;
                padding: 0;
            }
            .container {
                box-shadow: none;
                max-width: 100%;
            }
        }
    </style>
</head>
<body>
    <div class="container">
        <h1>Descendants of ` + formatPersonName(*rootPerson) + `</h1>
`)

	if len(descendants) == 0 {
		html.WriteString(`<p>No descendants found.</p>`)
	} else {
		// Summary section
		html.WriteString(`<div class="summary">`)
		html.WriteString(`<div class="summary-line"><strong>Total descendants:</strong> ` + fmt.Sprintf("%d", len(descendants)) + `</div>`)

		// Count by generation
		genCounts := make(map[int]int)
		for _, d := range descendants {
			genCounts[d.Generation]++
		}
		maxGen := 0
		for gen := range genCounts {
			if gen > maxGen {
				maxGen = gen
			}
		}
		for i := 1; i <= maxGen; i++ {
			html.WriteString(`<div class="summary-line"><strong>Generation ` + fmt.Sprintf("%d", i) + `:</strong> ` + fmt.Sprintf("%d descendants", genCounts[i]) + `</div>`)
		}
		html.WriteString(`</div>`)

		// Display descendants by generation
		for gen := 1; gen <= maxGen; gen++ {
			html.WriteString(`<h2>Generation ` + fmt.Sprintf("%d", gen) + `</h2>`)

			for _, d := range descendants {
				if d.Generation == gen {
					genClass := fmt.Sprintf("gen-%d", gen)
					if gen > 7 {
						genClass = "gen-7"
					}
					html.WriteString(`<div class="person-entry ` + genClass + `">`)
					html.WriteString(`<span class="person-name">` + formatPersonName(d.Person) + `</span>`)
					
					// Handle privacy for dates
					if options.LimitLivingInfo && d.Person.IsLiving {
						html.WriteString(`<span class="person-dates" style="font-style: italic; color: #7f8c8d;">(Living)</span>`)
					} else if d.Person.BirthDate != "" {
						html.WriteString(`<span class="person-dates">b. ` + formatFGSDate(d.Person.BirthDate))
						if !d.Person.IsLiving && d.Person.DeathDate != "" {
							html.WriteString(` - d. ` + formatFGSDate(d.Person.DeathDate))
						}
						html.WriteString(`</span>`)
					}
					html.WriteString(`</div>`)
				}
			}
		}
	}

	// Footer
	html.WriteString(`
        <div class="footer">
            <p>Generated by KrankyBear Genealogy on ` + time.Now().Format("January 2, 2006") + `</p>
        </div>
    </div>
</body>
</html>`)

	return html.String()
}

// exportAncestorReportPDF exports the Ancestor Report to PDF format
func exportAncestorReportPDF(w fyne.Window, s *store.Store, rootPersonID int64) {
	if rootPersonID <= 0 {
		dialog.ShowInformation("Export Ancestor Report", "Please select a person first", w)
		return
	}

	rootPerson, err := s.GetPersonByID(rootPersonID)
	if err != nil {
		dialog.ShowError(fmt.Errorf("Failed to load person: %w", err), w)
		return
	}

	// Show privacy options dialog
	showHTMLExportOptionsDialog(w, rootPerson, func(options HTMLExportOptions) {
		// Show file save dialog
		fd := dialog.NewFileSave(func(uc fyne.URIWriteCloser, err error) {
			if err != nil || uc == nil {
				return
			}
			defer uc.Close()

			// Generate PDF content with privacy options
			err = generateAncestorReportPDF(s, rootPerson, options, uc.URI().Path())
			if err != nil {
				dialog.ShowError(fmt.Errorf("Failed to generate PDF: %w", err), w)
				return
			}

			dialog.ShowInformation("Export Successful",
				fmt.Sprintf("Ancestor Report exported to:\n%s", uc.URI().Path()), w)
		}, w)

		// Set default filename
		defaultName := fmt.Sprintf("Ancestors_%s_%s.pdf",
			strings.ReplaceAll(rootPerson.Surname, " ", "_"),
			strings.ReplaceAll(rootPerson.GivenName, " ", "_"))
		fd.SetFileName(defaultName)
		fd.Show()
	})
}

// generateAncestorReportPDF creates a PDF document for the ancestor report
func generateAncestorReportPDF(s *store.Store, rootPerson *store.Person, options HTMLExportOptions, filepath string) error {
	pdf := gofpdf.New("P", "mm", "Letter", "")
	pdf.AddPage()
	
	// Title
	pdf.SetFont("Arial", "B", 16)
	pdf.CellFormat(0, 10, "Ancestor Report", "", 1, "C", false, 0, "")
	pdf.SetFont("Arial", "", 10)
	pdf.CellFormat(0, 6, "(Ahnentafel Numbering System)", "", 1, "C", false, 0, "")
	pdf.Ln(2)
	
	// Root person
	pdf.SetFont("Arial", "B", 14)
	pdf.CellFormat(0, 8, "Ancestors of "+formatPersonName(*rootPerson), "", 1, "C", false, 0, "")
	pdf.Ln(3)
	
	// Collect ancestors
	ancestors := collectAncestors(s, rootPerson.ID, 1)
	
	// Filter based on privacy
	var filteredAncestors []AncestorNode
	for _, anc := range ancestors {
		if shouldShowPerson(&anc.Person, options) {
			filteredAncestors = append(filteredAncestors, anc)
		}
	}
	
	// Sort by Ahnentafel number for proper order
	sort.Slice(filteredAncestors, func(i, j int) bool {
		return filteredAncestors[i].AhnentafelNumber < filteredAncestors[j].AhnentafelNumber
	})
	
	// Count generations
	generations := make(map[int]bool)
	for _, anc := range filteredAncestors {
		// Calculate generation from Ahnentafel number
		generation := 0
		num := anc.AhnentafelNumber
		for num > 0 {
			generation++
			num = num / 2
		}
		generations[generation] = true
	}
	
	// Statistics
	pdf.SetFont("Arial", "", 10)
	pdf.CellFormat(0, 6, fmt.Sprintf("Total Ancestors: %d", len(filteredAncestors)), "", 1, "L", false, 0, "")
	pdf.CellFormat(0, 6, fmt.Sprintf("Generations: %d", len(generations)), "", 1, "L", false, 0, "")
	pdf.Ln(3)
	
	// Check for pedigree collapse
	uniqueIDs := make(map[int64]bool)
	for _, anc := range filteredAncestors {
		uniqueIDs[anc.Person.ID] = true
	}
	if len(uniqueIDs) < len(filteredAncestors) {
		pdf.SetFillColor(255, 243, 205)
		pdf.SetFont("Arial", "B", 10)
		pdf.MultiCell(0, 6, "Warning: Pedigree Collapse Detected - Some ancestors appear multiple times", "", "L", true)
		pdf.Ln(2)
	}
	
	// Render ancestors
	pdf.SetFont("Arial", "", 10)
	for _, anc := range filteredAncestors {
		// Check if we need a new page
		_, pageHeight := pdf.GetPageSize()
		_, y := pdf.GetXY()
		if y > pageHeight-30 {
			pdf.AddPage()
		}
		
		// Ahnentafel number
		pdf.SetFont("Arial", "B", 10)
		pdf.Cell(15, 6, fmt.Sprintf("%d.", anc.AhnentafelNumber))
		
		// Person name and details
		pdf.SetFont("Arial", "", 10)
		personInfo := formatPersonName(anc.Person)
		if !options.LimitLivingInfo || !anc.Person.IsLiving {
			if anc.Person.BirthDate != "" {
				personInfo += fmt.Sprintf(" (b. %s", formatFGSDate(anc.Person.BirthDate))
				if anc.Person.BirthPlace != "" {
					personInfo += fmt.Sprintf(", %s", anc.Person.BirthPlace)
				}
				personInfo += ")"
			}
			if !anc.Person.IsLiving && anc.Person.DeathDate != "" {
				personInfo += fmt.Sprintf(" (d. %s", formatFGSDate(anc.Person.DeathDate))
				if anc.Person.DeathPlace != "" {
					personInfo += fmt.Sprintf(", %s", anc.Person.DeathPlace)
				}
				personInfo += ")"
			}
		}
		if anc.Person.IsLiving {
			personInfo += " (Living)"
		}
		pdf.MultiCell(0, 6, personInfo, "", "L", false)
		
		// Relationship (calculate from Ahnentafel number)
		relationship := getAhnentafelRelationship(anc.AhnentafelNumber)
		if relationship != "" {
			pdf.SetFont("Arial", "I", 9)
			pdf.Cell(15, 5, "")
			pdf.Cell(0, 5, relationship)
			pdf.Ln(5)
		}
	}
	
	// Footer with explanation
	pdf.Ln(3)
	pdf.SetFont("Arial", "I", 8)
	pdf.MultiCell(0, 4, "Ahnentafel Numbering: Person 1 is the root. For any person N: father = 2N, mother = 2N+1", "", "L", false)
	
	return pdf.OutputFileAndClose(filepath)
}

// exportAncestorReportHTML exports the Ancestor Report to HTML format
func exportAncestorReportHTML(w fyne.Window, s *store.Store, rootPersonID int64) {
	if rootPersonID <= 0 {
		dialog.ShowInformation("Export Ancestor Report", "Please select a person first", w)
		return
	}

	rootPerson, err := s.GetPersonByID(rootPersonID)
	if err != nil {
		dialog.ShowError(fmt.Errorf("Failed to load person: %w", err), w)
		return
	}

	// Show privacy options dialog
	showHTMLExportOptionsDialog(w, rootPerson, func(options HTMLExportOptions) {
		// Show file save dialog
		fd := dialog.NewFileSave(func(uc fyne.URIWriteCloser, err error) {
			if err != nil || uc == nil {
				return
			}
			defer uc.Close()

			// Generate HTML content with privacy options
			html := generateAncestorReportHTML(s, rootPerson, options)

			// Write to file
			_, err = uc.Write([]byte(html))
			if err != nil {
				dialog.ShowError(fmt.Errorf("Failed to write HTML file: %w", err), w)
				return
			}

			dialog.ShowInformation("Export Successful",
				fmt.Sprintf("Ancestor Report exported to:\n%s", uc.URI().Path()), w)
		}, w)

		// Set default filename
		defaultName := fmt.Sprintf("Ancestors_%s_%s.html",
			strings.ReplaceAll(rootPerson.Surname, " ", "_"),
			strings.ReplaceAll(rootPerson.GivenName, " ", "_"))
		fd.SetFileName(defaultName)
		fd.Show()
	})
}

// generateAncestorReportHTML generates the HTML content for the Ancestor Report
func generateAncestorReportHTML(s *store.Store, rootPerson *store.Person, options HTMLExportOptions) string {
	var html strings.Builder

	// Build ancestor tree in ahnentafel format
	ancestors := collectAncestors(s, rootPerson.ID, 1)
	
	// Filter ancestors based on privacy settings
	filteredAncestors := []AncestorNode{}
	for _, a := range ancestors {
		if shouldShowPerson(&a.Person, options) {
			filteredAncestors = append(filteredAncestors, a)
		}
	}
	ancestors = filteredAncestors

	// HTML header with embedded CSS
	html.WriteString(`<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Ancestors of ` + formatPersonName(*rootPerson) + ` (Ahnentafel)</title>
    <style>
        * {
            margin: 0;
            padding: 0;
            box-sizing: border-box;
        }
        body {
            font-family: 'Segoe UI', Tahoma, Geneva, Verdana, sans-serif;
            line-height: 1.6;
            color: #333;
            background-color: #f5f5f5;
            padding: 20px;
        }
        .container {
            max-width: 900px;
            margin: 0 auto;
            background-color: white;
            padding: 40px;
            box-shadow: 0 2px 10px rgba(0,0,0,0.1);
            border-radius: 8px;
        }
        h1 {
            text-align: center;
            color: #2c3e50;
            margin-bottom: 10px;
        }
        .subtitle {
            text-align: center;
            color: #7f8c8d;
            margin-bottom: 30px;
            padding-bottom: 15px;
            border-bottom: 3px solid #3498db;
            font-style: italic;
        }
        .summary {
            background-color: #ecf0f1;
            padding: 20px;
            border-radius: 5px;
            margin-bottom: 30px;
        }
        .summary-line {
            margin: 5px 0;
            font-size: 1.05em;
        }
        .warning {
            background-color: #fff3cd;
            border-left: 4px solid #ffc107;
            padding: 12px;
            margin-bottom: 20px;
            border-radius: 3px;
        }
        .ancestor-entry {
            margin: 15px 0;
            padding: 15px;
            background-color: #fafafa;
            border-radius: 5px;
            border-left: 4px solid #3498db;
        }
        .ahnentafel-number {
            font-weight: bold;
            color: #3498db;
            font-size: 1.1em;
            margin-right: 10px;
        }
        .person-name {
            font-weight: 600;
            color: #2c3e50;
            font-size: 1.1em;
        }
        .person-details {
            color: #555;
            margin-top: 5px;
            margin-left: 35px;
        }
        .relationship {
            color: #7f8c8d;
            font-style: italic;
            margin-left: 5px;
        }
        .generation {
            color: #95a5a6;
            font-size: 0.9em;
            margin-left: 10px;
        }
        .footer {
            margin-top: 40px;
            padding-top: 20px;
            border-top: 1px solid #ecf0f1;
            text-align: center;
            color: #7f8c8d;
            font-size: 0.9em;
        }
        @media print {
            body {
                background-color: white;
                padding: 0;
            }
            .container {
                box-shadow: none;
                max-width: 100%;
            }
        }
    </style>
</head>
<body>
    <div class="container">
        <h1>Ancestors of ` + formatPersonName(*rootPerson) + `</h1>
        <div class="subtitle">Ahnentafel Numbering System</div>
`)

	if len(ancestors) == 0 {
		html.WriteString(`<p>No ancestors found.</p>`)
	} else {
		// Summary section
		html.WriteString(`<div class="summary">`)
		html.WriteString(`<div class="summary-line"><strong>Total ancestors:</strong> ` + fmt.Sprintf("%d", len(ancestors)) + `</div>`)

		// Pedigree collapse detection
		uniqueIDs := make(map[int64]bool)
		collapseCount := 0
		for _, a := range ancestors {
			if uniqueIDs[a.Person.ID] {
				collapseCount++
			}
			uniqueIDs[a.Person.ID] = true
		}
		if collapseCount > 0 {
			html.WriteString(`<div class="summary-line"><strong>⚠️ Pedigree collapse:</strong> ` + fmt.Sprintf("%d ancestors appear multiple times", collapseCount) + `</div>`)
		}
		html.WriteString(`</div>`)

		if collapseCount > 0 {
			html.WriteString(`<div class="warning">`)
			html.WriteString(`<strong>Note:</strong> Pedigree collapse occurs when ancestors appear in multiple positions in the family tree, indicating intermarriage between related individuals in earlier generations.`)
			html.WriteString(`</div>`)
		}

		// Display ancestors in ahnentafel order
		// Sort by ahnentafel number
		sort.Slice(ancestors, func(i, j int) bool {
			return ancestors[i].AhnentafelNumber < ancestors[j].AhnentafelNumber
		})

		for _, a := range ancestors {
			generation := int(math.Log2(float64(a.AhnentafelNumber))) + 1
			relationship := getAhnentafelRelationship(a.AhnentafelNumber)

			html.WriteString(`<div class="ancestor-entry">`)
			html.WriteString(`<div>`)
			html.WriteString(`<span class="ahnentafel-number">` + fmt.Sprintf("%d.", a.AhnentafelNumber) + `</span>`)
			html.WriteString(`<span class="person-name">` + formatPersonName(a.Person) + `</span>`)
			html.WriteString(`<span class="relationship">` + relationship + `</span>`)
			html.WriteString(`<span class="generation">[Gen ` + fmt.Sprintf("%d", generation) + `]</span>`)
			html.WriteString(`</div>`)
			html.WriteString(`<div class="person-details">`)
			
			// Handle privacy for details
			if options.LimitLivingInfo && a.Person.IsLiving {
				html.WriteString(`<span style="font-style: italic; color: #7f8c8d;">(Living - limited information)</span>`)
			} else {
				if a.Person.BirthDate != "" {
					html.WriteString(`Born: ` + formatFGSDate(a.Person.BirthDate))
					if a.Person.BirthPlace != "" {
						html.WriteString(` in ` + a.Person.BirthPlace)
					}
				}
				if !a.Person.IsLiving && a.Person.DeathDate != "" {
					if a.Person.BirthDate != "" {
						html.WriteString(` • `)
					}
					html.WriteString(`Died: ` + formatFGSDate(a.Person.DeathDate))
					if a.Person.DeathPlace != "" {
						html.WriteString(` in ` + a.Person.DeathPlace)
					}
				}
			}
			html.WriteString(`</div>`)
			html.WriteString(`</div>`)
		}
	}

	// Footer
	html.WriteString(`
        <div class="footer">
            <p>Generated by KrankyBear Genealogy on ` + time.Now().Format("January 2, 2006") + `</p>
            <p style="margin-top: 10px; font-size: 0.85em;">Ahnentafel (German for "ancestor table") is a genealogical numbering system where each person is assigned a number. The subject is #1, their father is #2, mother is #3, and so on with fathers always even numbers and mothers odd.</p>
        </div>
    </div>
</body>
</html>`)

	return html.String()
}

func renderMarriedFamily(content *fyne.Container, s *store.Store, person *store.Person, spouse *store.Person, navigateFunc func(int64)) {
	// Determine husband and wife based on gender
	var husband, wife *store.Person
	if person.Gender == "M" {
		husband = person
		wife = spouse
	} else {
		husband = spouse
		wife = person
	}

	// Husband section
	content.Add(widget.NewLabelWithStyle("HUSBAND", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}))
	if husband != nil {
		husbandCopy := *husband
		husbandBtn := widget.NewButton(
			formatPersonName(husbandCopy),
			func() { navigateFunc(husbandCopy.ID) })
		content.Add(husbandBtn)
		content.Add(widget.NewLabel(fmt.Sprintf("  Born: %s", formatFGSDate(husband.BirthDate))))
		if husband.BirthPlace != "" {
			content.Add(widget.NewLabel(fmt.Sprintf("  Place: %s", husband.BirthPlace)))
		}
		if !husband.IsLiving {
			content.Add(widget.NewLabel(fmt.Sprintf("  Died: %s", formatFGSDate(husband.DeathDate))))
			if husband.DeathPlace != "" {
				content.Add(widget.NewLabel(fmt.Sprintf("  Place: %s", husband.DeathPlace)))
			}
		}
	}
	content.Add(widget.NewLabel(""))

	// Wife section
	content.Add(widget.NewLabelWithStyle("WIFE", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}))
	if wife != nil {
		wifeCopy := *wife
		wifeBtn := widget.NewButton(
			formatPersonName(wifeCopy),
			func() { navigateFunc(wifeCopy.ID) })
		content.Add(wifeBtn)
		content.Add(widget.NewLabel(fmt.Sprintf("  Born: %s", formatFGSDate(wife.BirthDate))))
		if wife.BirthPlace != "" {
			content.Add(widget.NewLabel(fmt.Sprintf("  Place: %s", wife.BirthPlace)))
		}
		if !wife.IsLiving {
			content.Add(widget.NewLabel(fmt.Sprintf("  Died: %s", formatFGSDate(wife.DeathDate))))
			if wife.DeathPlace != "" {
				content.Add(widget.NewLabel(fmt.Sprintf("  Place: %s", wife.DeathPlace)))
			}
		}
	}
	content.Add(widget.NewLabel(""))

	// Marriage information
	var marriageDate string
	var marriagePlace string
	if husband != nil && wife != nil {
		// Try to get marriage relationship between husband and wife
		relationships, _ := s.GetRelationshipsBetween(husband.ID, wife.ID, "spouse")
		if len(relationships) > 0 {
			marriageDate = relationships[0].MarriageDate
			marriagePlace = relationships[0].MarriagePlace
		}
	}
	content.Add(widget.NewLabelWithStyle("MARRIAGE", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}))
	if marriageDate != "" || marriagePlace != "" {
		if marriageDate != "" {
			content.Add(widget.NewLabel(fmt.Sprintf("  Date: %s", formatFGSDate(marriageDate))))
		}
		if marriagePlace != "" {
			content.Add(widget.NewLabel(fmt.Sprintf("  Place: %s", marriagePlace)))
		}
	} else {
		content.Add(widget.NewLabel("  (No marriage record)"))
	}
	content.Add(widget.NewLabel(""))

	// Children section
	content.Add(widget.NewLabelWithStyle("CHILDREN", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}))
	
	// Get children of this specific couple
	var children []store.Person
	if husband != nil && wife != nil {
		// Get all children of the husband
		husbandChildren, _ := s.GetRelatedPeople(husband.ID, "child")
		// Filter to only include children who have this wife as mother
		for _, child := range husbandChildren {
			parents, _ := s.GetRelatedPeople(child.ID, "parent")
			hasWifeAsParent := false
			for _, parent := range parents {
				if parent.ID == wife.ID {
					hasWifeAsParent = true
					break
				}
			}
			if hasWifeAsParent {
				children = append(children, child)
			}
		}
	} else if husband != nil {
		// Only husband, get all his children
		children, _ = s.GetRelatedPeople(husband.ID, "child")
	} else if wife != nil {
		// Only wife, get all her children
		children, _ = s.GetRelatedPeople(wife.ID, "child")
	}
	
	if len(children) == 0 {
		content.Add(widget.NewLabel("  (No children recorded)"))
	} else {
		// Sort children by birth date
		sort.Slice(children, func(i, j int) bool {
			return children[i].BirthDate < children[j].BirthDate
		})
		
		for i, child := range children {
			childCopy := child
			childNum := widget.NewLabelWithStyle(fmt.Sprintf("%d.", i+1), fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
			content.Add(childNum)
			
			childBtn := widget.NewButton(
				fmt.Sprintf("  %s", formatPersonName(childCopy)),
				func() { navigateFunc(childCopy.ID) })
			content.Add(childBtn)
			
			content.Add(widget.NewLabel(fmt.Sprintf("    Born: %s", formatFGSDate(child.BirthDate))))
			if child.BirthPlace != "" {
				content.Add(widget.NewLabel(fmt.Sprintf("    Place: %s", child.BirthPlace)))
			}
			
			// Show spouse if married
			childSpouses, _ := s.GetRelatedPeople(child.ID, "spouse")
			if len(childSpouses) > 0 {
				spouseNames := make([]string, len(childSpouses))
				for j, sp := range childSpouses {
					spouseNames[j] = formatPersonName(sp)
				}
				content.Add(widget.NewLabel(fmt.Sprintf("    Spouse: %s", strings.Join(spouseNames, ", "))))
			}
			
			// Show grandchildren (children of this child)
			grandchildren, _ := s.GetRelatedPeople(child.ID, "child")
			if len(grandchildren) > 0 {
				// Sort grandchildren by birth date
				sort.Slice(grandchildren, func(a, b int) bool {
					return grandchildren[a].BirthDate < grandchildren[b].BirthDate
				})
				grandchildNames := make([]string, len(grandchildren))
				for j, gc := range grandchildren {
					grandchildNames[j] = fmt.Sprintf("%s (%s)", formatPersonName(gc), formatFGSDate(gc.BirthDate))
				}
				content.Add(widget.NewLabel(fmt.Sprintf("    Children: %s", strings.Join(grandchildNames, ", "))))
			}
			
			if !child.IsLiving {
				content.Add(widget.NewLabel(fmt.Sprintf("    Died: %s", formatFGSDate(child.DeathDate))))
				if child.DeathPlace != "" {
					content.Add(widget.NewLabel(fmt.Sprintf("    Place: %s", child.DeathPlace)))
				}
			}
			content.Add(widget.NewLabel(""))
		}
	}
}

func renderParentalFamily(content *fyne.Container, s *store.Store, person *store.Person, parents []store.Person, navigateFunc func(int64)) {
	// Show person's parents
	var father, mother *store.Person
	for i := range parents {
		if parents[i].Gender == "M" {
			father = &parents[i]
		} else {
			mother = &parents[i]
		}
	}
	
	if father != nil || mother != nil {
		content.Add(widget.NewLabelWithStyle("PARENTS", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}))
		
		if father != nil {
			fatherCopy := *father
			fatherBtn := widget.NewButton(
				fmt.Sprintf("Father: %s", formatPersonName(fatherCopy)),
				func() { navigateFunc(fatherCopy.ID) })
			content.Add(fatherBtn)
		}
		
		if mother != nil {
			motherCopy := *mother
			motherBtn := widget.NewButton(
				fmt.Sprintf("Mother: %s", formatPersonName(motherCopy)),
				func() { navigateFunc(motherCopy.ID) })
			content.Add(motherBtn)
		}
		content.Add(widget.NewLabel(""))
	}
	
	// Show the person themselves
	content.Add(widget.NewLabelWithStyle("PERSON", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}))
	personCopy := *person
	personBtn := widget.NewButton(
		formatPersonName(personCopy),
		func() { navigateFunc(personCopy.ID) })
	content.Add(personBtn)
	content.Add(widget.NewLabel(fmt.Sprintf("  Born: %s", formatFGSDate(person.BirthDate))))
	if person.BirthPlace != "" {
		content.Add(widget.NewLabel(fmt.Sprintf("  Place: %s", person.BirthPlace)))
	}
	if !person.IsLiving {
		content.Add(widget.NewLabel(fmt.Sprintf("  Died: %s", formatFGSDate(person.DeathDate))))
		if person.DeathPlace != "" {
			content.Add(widget.NewLabel(fmt.Sprintf("  Place: %s", person.DeathPlace)))
		}
	}
	content.Add(widget.NewLabel(""))
	
	// Show siblings
	content.Add(widget.NewLabelWithStyle("SIBLINGS", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}))
	siblings := []store.Person{}
	
	// Get siblings through parents
	if father != nil {
		fatherChildren, _ := s.GetRelatedPeople(father.ID, "child")
		for _, child := range fatherChildren {
			if child.ID != person.ID {
				siblings = append(siblings, child)
			}
		}
	} else if mother != nil {
		motherChildren, _ := s.GetRelatedPeople(mother.ID, "child")
		for _, child := range motherChildren {
			if child.ID != person.ID {
				siblings = append(siblings, child)
			}
		}
	}
	
	if len(siblings) == 0 {
		content.Add(widget.NewLabel("  (No siblings recorded)"))
	} else {
		// Remove duplicates
		uniqueSiblings := make(map[int64]store.Person)
		for _, sib := range siblings {
			uniqueSiblings[sib.ID] = sib
		}
		
		// Convert back to slice and sort
		siblings = []store.Person{}
		for _, sib := range uniqueSiblings {
			siblings = append(siblings, sib)
		}
		sort.Slice(siblings, func(i, j int) bool {
			return siblings[i].BirthDate < siblings[j].BirthDate
		})
		
		for _, sibling := range siblings {
			siblingCopy := sibling
			sibBtn := widget.NewButton(
				fmt.Sprintf("  %s (%s)", formatPersonName(siblingCopy), formatFGSDate(sibling.BirthDate)),
				func() { navigateFunc(siblingCopy.ID) })
			content.Add(sibBtn)
		}
	}
}

func formatFGSDate(date string) string {
	if date == "" {
		return "Unknown"
	}
	return date
}

// Timeline View
var timelineDialog fyne.Window

func showTimelineView(w fyne.Window, s *store.Store, navigateFunc func(int64)) {
	// Check if already open
	if timelineDialog != nil {
		timelineDialog.RequestFocus()
		timelineDialog.Show()
		return
	}

	// Collect all events (births, deaths, marriages)
	events := collectTimelineEvents(s)

	// Build content
	content := container.NewVBox()

	title := widget.NewLabelWithStyle("Timeline of Life Events",
		fyne.TextAlignCenter, fyne.TextStyle{Bold: true})
	content.Add(title)

	if len(events) == 0 {
		content.Add(widget.NewLabel("No events with dates found."))
	} else {
		// Summary
		summary := widget.NewLabel(fmt.Sprintf("Total events: %d (births: %d, deaths: %d, marriages: %d)",
			len(events), countEventType(events, "Birth"), countEventType(events, "Death"), countEventType(events, "Marriage")))
		content.Add(summary)

		// Date range
		if len(events) > 0 {
			earliest := events[0].Date
			latest := events[len(events)-1].Date
			rangeLabel := widget.NewLabel(fmt.Sprintf("Date range: %s - %s", earliest, latest))
			content.Add(rangeLabel)
		}

		content.Add(widget.NewSeparator())

		// Display events chronologically
		currentYear := ""
		for _, event := range events {
			eCopy := event

			// Year separator
			year := extractYear(event.Date)
			if year != currentYear {
				currentYear = year
				yearLabel := widget.NewLabelWithStyle(currentYear,
					fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
				content.Add(yearLabel)
			}

			// Event icon
			icon := ""
			switch event.Type {
			case "Birth":
				icon = "👶"
			case "Death":
				icon = "✝"
			case "Marriage":
				icon = "💒"
			}

			eventBtn := widget.NewButton(
				fmt.Sprintf("%s %s - %s: %s (%s)",
					icon, event.Date, event.Type, event.PersonName, event.Place),
				func() {
					navigateFunc(eCopy.PersonID)
				})
			content.Add(eventBtn)
		}
	}

	scroll := container.NewVScroll(content)
	scroll.SetMinSize(fyne.NewSize(700, 400))

	timelineDialog = fyne.CurrentApp().NewWindow("Timeline View")
	
	// Export buttons
	exportHTMLBtn := widget.NewButton("Export to HTML", func() {
		exportTimelineToHTMLFile(timelineDialog, s, events)
	})
	exportPDFBtn := widget.NewButton("Export to PDF", func() {
		exportTimelineToPDFFile(timelineDialog, s, events)
	})
	exportButtons := container.NewHBox(exportHTMLBtn, exportPDFBtn)
	
	// Main content with export buttons at bottom
	mainContent := container.NewBorder(nil, exportButtons, nil, nil, scroll)
	timelineDialog.SetContent(mainContent)
	timelineDialog.Resize(fyne.NewSize(900, 600))
	timelineDialog.SetOnClosed(func() {
		timelineDialog = nil
	})
	timelineDialog.Show()
}

type TimelineEvent struct {
	PersonID   int64
	PersonName string
	Type       string // "Birth", "Death", "Marriage"
	Date       string
	Place      string
}

// exportTimelineToHTMLFile exports the timeline to an HTML file
func exportTimelineToHTMLFile(w fyne.Window, s *store.Store, events []TimelineEvent) {
	// Show file save dialog
	fd := dialog.NewFileSave(func(uc fyne.URIWriteCloser, err error) {
		if err != nil || uc == nil {
			return
		}
		defer uc.Close()

		// Generate HTML content
		html := generateTimelineHTMLReport(events)

		// Write to file
		_, err = uc.Write([]byte(html))
		if err != nil {
			dialog.ShowError(fmt.Errorf("Failed to write HTML file: %w", err), w)
			return
		}

		dialog.ShowInformation("Export Successful",
			fmt.Sprintf("Timeline exported to:\n%s", uc.URI().Path()), w)
	}, w)

	fd.SetFileName("timeline_report.html")
	fd.Show()
}

// generateTimelineHTMLReport creates an HTML document for the timeline
func generateTimelineHTMLReport(events []TimelineEvent) string {
	var html strings.Builder
	
	html.WriteString(`<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Timeline Report</title>
    <style>
        * { margin: 0; padding: 0; box-sizing: border-box; }
        body {
            font-family: 'Segoe UI', Tahoma, Geneva, Verdana, sans-serif;
            line-height: 1.6;
            color: #333;
            background-color: #f5f5f5;
            padding: 20px;
        }
        .container {
            max-width: 900px;
            margin: 0 auto;
            background-color: white;
            padding: 40px;
            box-shadow: 0 2px 10px rgba(0,0,0,0.1);
        }
        h1 { color: #2c3e50; margin-bottom: 10px; text-align: center; }
        .summary { text-align: center; color: #7f8c8d; margin-bottom: 30px; }
        .year-group { margin-bottom: 30px; }
        .year-header {
            background-color: #3498db;
            color: white;
            padding: 10px 15px;
            font-size: 1.2em;
            font-weight: bold;
            margin-bottom: 10px;
        }
        .event {
            padding: 10px;
            margin-bottom: 8px;
            background-color: #ecf0f1;
            border-left: 4px solid #95a5a6;
        }
        .event.birth { border-left-color: #27ae60; }
        .event.death { border-left-color: #e74c3c; }
        .event.marriage { border-left-color: #9b59b6; }
        .event-icon { font-size: 1.2em; margin-right: 5px; }
        .event-date { font-weight: bold; color: #2c3e50; }
        .event-person { color: #3498db; }
        .event-place { color: #7f8c8d; font-style: italic; }
        @media print {
            body { background-color: white; padding: 0; }
            .container { box-shadow: none; max-width: 100%; }
        }
    </style>
</head>
<body>
    <div class="container">
        <h1>Timeline of Life Events</h1>
        <div class="summary">
`)

	// Statistics
	birthCount := 0
	deathCount := 0
	marriageCount := 0
	for _, e := range events {
		switch e.Type {
		case "Birth":
			birthCount++
		case "Death":
			deathCount++
		case "Marriage":
			marriageCount++
		}
	}
	
	html.WriteString(fmt.Sprintf("<p>Total events: %d (births: %d, deaths: %d, marriages: %d)</p>", 
		len(events), birthCount, deathCount, marriageCount))
	
	if len(events) > 0 {
		html.WriteString(fmt.Sprintf("<p>Date range: %s - %s</p>", events[0].Date, events[len(events)-1].Date))
	}
	
	html.WriteString(`        </div>
`)

	// Group by year
	currentYear := ""
	for _, event := range events {
		year := extractYear(event.Date)
		
		if year != currentYear {
			if currentYear != "" {
				html.WriteString(`        </div>
`)
			}
			currentYear = year
			html.WriteString(`        <div class="year-group">
            <div class="year-header">` + year + `</div>
`)
		}
		
		// Event
		eventClass := strings.ToLower(event.Type)
		eventIcon := ""
		switch event.Type {
		case "Birth":
			eventIcon = "👶"
		case "Death":
			eventIcon = "✝️"
		case "Marriage":
			eventIcon = "💒"
		}
		
		html.WriteString(`            <div class="event ` + eventClass + `">
                <span class="event-icon">` + eventIcon + `</span>
                <span class="event-date">` + formatFGSDate(event.Date) + `</span> - 
                <span class="event-type">` + event.Type + `:</span> 
                <span class="event-person">` + event.PersonName + `</span>
`)
		if event.Place != "" {
			html.WriteString(`                <span class="event-place"> (` + event.Place + `)</span>
`)
		}
		html.WriteString(`            </div>
`)
	}
	
	if currentYear != "" {
		html.WriteString(`        </div>
`)
	}
	
	html.WriteString(`    </div>
</body>
</html>`)

	return html.String()
}

// exportTimelineToPDFFile exports the timeline to a PDF file
func exportTimelineToPDFFile(w fyne.Window, s *store.Store, events []TimelineEvent) {
	// Show file save dialog
	fd := dialog.NewFileSave(func(uc fyne.URIWriteCloser, err error) {
		if err != nil || uc == nil {
			return
		}
		defer uc.Close()

		// Generate PDF
		err = generateTimelinePDFReport(events, uc.URI().Path())
		if err != nil {
			dialog.ShowError(fmt.Errorf("Failed to generate PDF: %w", err), w)
			return
		}

		dialog.ShowInformation("Export Successful",
			fmt.Sprintf("Timeline exported to:\n%s", uc.URI().Path()), w)
	}, w)

	fd.SetFileName("timeline_report.pdf")
	fd.Show()
}

// generateTimelinePDFReport creates a PDF document for the timeline
func generateTimelinePDFReport(events []TimelineEvent, filepath string) error {
	pdf := gofpdf.New("P", "mm", "Letter", "")
	pdf.AddPage()
	
	// Title
	pdf.SetFont("Arial", "B", 16)
	pdf.CellFormat(0, 10, "Timeline of Life Events", "", 1, "C", false, 0, "")
	pdf.Ln(2)
	
	// Statistics
	pdf.SetFont("Arial", "", 10)
	birthCount := 0
	deathCount := 0
	marriageCount := 0
	for _, e := range events {
		switch e.Type {
		case "Birth":
			birthCount++
		case "Death":
			deathCount++
		case "Marriage":
			marriageCount++
		}
	}
	
	pdf.CellFormat(0, 6, fmt.Sprintf("Total events: %d (births: %d, deaths: %d, marriages: %d)",
		len(events), birthCount, deathCount, marriageCount), "", 1, "L", false, 0, "")
	
	if len(events) > 0 {
		pdf.CellFormat(0, 6, fmt.Sprintf("Date range: %s - %s", events[0].Date, events[len(events)-1].Date), "", 1, "L", false, 0, "")
	}
	
	pdf.Ln(5)
	
	// Group by year
	currentYear := ""
	for _, event := range events {
		// Check if we need a new page
		_, pageHeight := pdf.GetPageSize()
		_, y := pdf.GetXY()
		if y > pageHeight-30 {
			pdf.AddPage()
		}
		
		year := extractYear(event.Date)
		
		if year != currentYear {
			currentYear = year
			pdf.Ln(2)
			pdf.SetFont("Arial", "B", 12)
			pdf.SetFillColor(52, 152, 219)
			pdf.SetTextColor(255, 255, 255)
			pdf.CellFormat(0, 8, year, "", 1, "L", true, 0, "")
			pdf.SetTextColor(0, 0, 0)
		}
		
		// Event
		pdf.SetFont("Arial", "B", 10)
		pdf.Cell(30, 6, formatFGSDate(event.Date))
		pdf.SetFont("Arial", "", 10)
		
		eventText := event.Type + ": " + event.PersonName
		if event.Place != "" {
			eventText += " (" + event.Place + ")"
		}
		pdf.MultiCell(0, 6, eventText, "", "L", false)
	}
	
	return pdf.OutputFileAndClose(filepath)
}

func collectTimelineEvents(s *store.Store) []TimelineEvent {
	events := []TimelineEvent{}

	people, err := s.GetPeople()
	if err != nil {
		return events
	}

	for _, person := range people {
		name := formatPersonName(person)

		// Birth events
		if person.BirthDate != "" {
			events = append(events, TimelineEvent{
				PersonID:   person.ID,
				PersonName: name,
				Type:       "Birth",
				Date:       person.BirthDate,
				Place:      person.BirthPlace,
			})
		}

		// Death events
		if person.DeathDate != "" {
			events = append(events, TimelineEvent{
				PersonID:   person.ID,
				PersonName: name,
				Type:       "Death",
				Date:       person.DeathDate,
				Place:      person.DeathPlace,
			})
		}

		// Marriage events
		rels, err := s.GetRelationshipsForPerson(person.ID)
		if err == nil {
			for _, rel := range rels {
				if rel.Type == "spouse" && rel.MarriageDate != "" && rel.SubjectID == person.ID {
					spouse, err := s.GetPersonByID(rel.ObjectID)
					if err == nil {
						events = append(events, TimelineEvent{
							PersonID:   person.ID,
							PersonName: fmt.Sprintf("%s & %s", name, formatPersonName(*spouse)),
							Type:       "Marriage",
							Date:       rel.MarriageDate,
							Place:      rel.MarriagePlace,
						})
					}
				}
			}
		}
	}

	// Sort by date chronologically
	sort.Slice(events, func(i, j int) bool {
		dateI := normalizeDateForSort(events[i].Date)
		dateJ := normalizeDateForSort(events[j].Date)
		return dateI < dateJ
	})

	return events
}

func countEventType(events []TimelineEvent, eventType string) int {
	count := 0
	for _, e := range events {
		if e.Type == eventType {
			count++
		}
	}
	return count
}

func extractYear(dateStr string) string {
	// Extract year from various date formats
	if dateStr == "" {
		return ""
	}
	
	// Try parsing common formats
	layouts := []string{
		"2006-01-02",      // YYYY-MM-DD
		"02 Jan 2006",     // DD Mon YYYY
		"Jan 2006",        // Mon YYYY
		"2006",            // YYYY only
		"01/02/2006",      // MM/DD/YYYY
		"02-01-2006",      // DD-MM-YYYY
	}
	
	for _, layout := range layouts {
		if t, err := time.Parse(layout, dateStr); err == nil {
			return fmt.Sprintf("%d", t.Year())
		}
	}
	
	// Fallback: if string starts with 4 digits, use those
	if len(dateStr) >= 4 {
		// Check if starts with year (19xx or 20xx)
		if (dateStr[0] == '1' || dateStr[0] == '2') && dateStr[1] >= '0' && dateStr[1] <= '9' {
			return dateStr[:4]
		}
		// Otherwise, try to find year at end (DD Mon YYYY format)
		parts := strings.Fields(dateStr)
		if len(parts) > 0 {
			lastPart := parts[len(parts)-1]
			if len(lastPart) == 4 {
				return lastPart
			}
		}
	}
	
	return dateStr
}

// Reviewed Items Management Report
var reviewedItemsDialog fyne.Window
var markAsReviewedDialogOpen bool

func showReviewedItemsReport(w fyne.Window, s *store.Store, navigateFunc func(int64)) {
	// Check if already open
	if reviewedItemsDialog != nil {
		reviewedItemsDialog.RequestFocus()
		reviewedItemsDialog.Show()
		return
	}

	// Function to rebuild the content
	var content *fyne.Container
	var scroll *container.Scroll
	var rebuildContent func() // Declare first to allow recursive reference

	rebuildContent = func() {
		validatedItems, err := s.GetAllValidatedItems()
		if err != nil {
			dialog.ShowError(fmt.Errorf("Failed to load reviewed items: %w", err), w)
			return
		}

		// Build content
		content.Objects = nil // Clear existing content

		if len(validatedItems) == 0 {
			content.Add(widget.NewLabel("No reviewed items yet."))
			content.Add(widget.NewLabel("Mark conflicts or duplicates as reviewed to see them here."))
		} else {
			// Summary
			summary := widget.NewLabelWithStyle(
				fmt.Sprintf("Total reviewed items: %d", len(validatedItems)),
				fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
			content.Add(summary)
			content.Add(widget.NewSeparator())

			// Group by type
			conflictItems := []store.ValidatedItem{}
			duplicateItems := []store.ValidatedItem{}

			for _, item := range validatedItems {
				if item.ItemType == "conflict" {
					conflictItems = append(conflictItems, item)
				} else if item.ItemType == "duplicate" {
					duplicateItems = append(duplicateItems, item)
				}
			}

			// Show conflicts
			if len(conflictItems) > 0 {
				conflictLabel := widget.NewLabelWithStyle(
					fmt.Sprintf("Reviewed Conflicts (%d):", len(conflictItems)),
					fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
				content.Add(conflictLabel)

				for _, item := range conflictItems {
					addReviewedItemDisplay(content, s, w, item, navigateFunc, rebuildContent)
				}

				content.Add(widget.NewSeparator())
			}

			// Show duplicates
			if len(duplicateItems) > 0 {
				dupLabel := widget.NewLabelWithStyle(
					fmt.Sprintf("Reviewed Duplicates (%d):", len(duplicateItems)),
					fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
				content.Add(dupLabel)

				for _, item := range duplicateItems {
					addReviewedItemDisplay(content, s, w, item, navigateFunc, rebuildContent)
				}
			}
		}

		content.Refresh()
		if scroll != nil {
			scroll.Refresh()
		}
	}

	// Initialize content container
	content = container.NewVBox()

	// Initial build
	rebuildContent()

	scroll = container.NewVScroll(content)
	scroll.SetMinSize(fyne.NewSize(700, 400))

	reviewedItemsDialog = fyne.CurrentApp().NewWindow("Reviewed Items Management")
	reviewedItemsDialog.SetContent(scroll)
	reviewedItemsDialog.Resize(fyne.NewSize(800, 600))
	reviewedItemsDialog.SetOnClosed(func() {
		reviewedItemsDialog = nil
	})
	reviewedItemsDialog.Show()
}

func addReviewedItemDisplay(content *fyne.Container, s *store.Store, w fyne.Window, item store.ValidatedItem, navigateFunc func(int64), refreshFunc func()) {
	itemCopy := item

	// Get person name
	person, err := s.GetPersonByID(item.PersonID)
	if err != nil {
		content.Add(widget.NewLabel(fmt.Sprintf("Error loading person %d", item.PersonID)))
		return
	}

	personName := formatPersonName(*person)

	// Build display
	typeLabel := widget.NewLabelWithStyle(
		fmt.Sprintf("%s - %s", item.ConflictType, personName),
		fyne.TextAlignLeading, fyne.TextStyle{Bold: true})

	reviewInfo := widget.NewLabel(fmt.Sprintf("  Reviewed by: %s", item.ReviewedBy))
	noteInfo := widget.NewLabel(fmt.Sprintf("  Note: %s", item.ValidationNote))
	dateInfo := widget.NewLabel(fmt.Sprintf("  Date: %s", item.ValidatedAt.Format("2006-01-02 15:04")))

	viewBtn := widget.NewButton("View Person", func() {
		navigateFunc(itemCopy.PersonID)
	})

	unmarkBtn := widget.NewButton("Unmark", func() {
		dialog.ShowConfirm("Unmark as Reviewed",
			"Are you sure you want to remove the review marking from this item? It will appear in reports again.",
			func(confirmed bool) {
				if confirmed {
					if err := s.UnmarkAsReviewed(itemCopy.ItemType, itemCopy.PersonID, itemCopy.RelatedPersonID, itemCopy.ConflictType); err != nil {
						dialog.ShowError(fmt.Errorf("Failed to unmark: %w", err), w)
						return
					}
					dialog.ShowInformation("Unmarked", "The item has been unmarked and will appear in reports again.", w)
					refreshFunc()
				}
			}, w)
	})

	buttonBox := container.NewHBox(viewBtn, unmarkBtn)

	itemBox := container.NewVBox(
		typeLabel,
		reviewInfo,
		noteInfo,
		dateInfo,
		buttonBox,
	)

	content.Add(itemBox)
	content.Add(widget.NewSeparator())
}

// Mass Mark as Living Report
var massMarkLivingDialog fyne.Window

func showMassMarkLivingReport(w fyne.Window, s *store.Store, refreshFunc func()) {
	// Check if already open
	if massMarkLivingDialog != nil {
		massMarkLivingDialog.RequestFocus()
		massMarkLivingDialog.Show()
		return
	}

	people, err := s.GetPeople()
	if err != nil {
		dialog.ShowError(fmt.Errorf("Failed to load people: %w", err), w)
		return
	}

	// Find people who might be living but aren't marked as such
	type candidatePerson struct {
		person store.Person
		age    int
	}

	var allCandidates []candidatePerson
	currentYear := time.Now().Year()

	for _, p := range people {
		// Skip if already marked as living
		if p.IsLiving {
			continue
		}

		// Skip if has death date
		if p.DeathDate != "" {
			continue
		}

		// Check birth date
		if p.BirthDate != "" {
			birthYear := parseBirthYear(p.BirthDate)
			if birthYear > 0 {
				age := currentYear - birthYear
				// Likely still living if younger than 120
				if age >= 0 && age < 120 {
					allCandidates = append(allCandidates, candidatePerson{p, age})
				}
			}
		}
	}

	// State variables
	filterText := ""
	sortAlphabetically := true // true = A-Z, false = by age

	// Track selections
	selections := make(map[int64]bool)
	checkboxes := make(map[int64]*widget.Check)

	// Build content container and scroll
	var content *fyne.Container
	var scroll *container.Scroll
	var rebuildContent func()

	rebuildContent = func() {
		// Filter candidates
		var candidates []candidatePerson
		for _, cand := range allCandidates {
			personName := strings.ToLower(formatPersonName(cand.person))
			if filterText == "" || strings.Contains(personName, strings.ToLower(filterText)) {
				candidates = append(candidates, cand)
			}
		}

		// Sort candidates
		if sortAlphabetically {
			sort.Slice(candidates, func(i, j int) bool {
				nameI := formatPersonName(candidates[i].person)
				nameJ := formatPersonName(candidates[j].person)
				return strings.ToLower(nameI) < strings.ToLower(nameJ)
			})
		} else {
			sort.Slice(candidates, func(i, j int) bool {
				return candidates[i].age < candidates[j].age
			})
		}

		// Clear and rebuild content
		content.Objects = nil
		checkboxes = make(map[int64]*widget.Check) // Reset checkboxes

		// Title
		title := widget.NewLabelWithStyle("Mark as Living (Bulk Update)",
			fyne.TextAlignCenter, fyne.TextStyle{Bold: true})
		content.Add(title)
		content.Add(widget.NewSeparator())

		if len(allCandidates) == 0 {
			content.Add(widget.NewLabel("✅ No candidates found!"))
			content.Add(widget.NewLabel("All people who appear to be living are already marked as such."))
		} else if len(candidates) == 0 {
			content.Add(widget.NewLabel(fmt.Sprintf("No matches for filter \"%s\"", filterText)))
			content.Add(widget.NewLabel(fmt.Sprintf("Total candidates: %d", len(allCandidates))))
		} else {
			// Summary
			summary := widget.NewLabel(fmt.Sprintf("Showing %d of %d people who may be living but are not marked as such:",
				len(candidates), len(allCandidates)))
			content.Add(summary)
			content.Add(widget.NewSeparator())

			// Select All / Deselect All / Mark Selected buttons
			selectAllBtn := widget.NewButton("Select All Visible", func() {
				for _, cand := range candidates {
					if check, ok := checkboxes[cand.person.ID]; ok {
						check.SetChecked(true)
						selections[cand.person.ID] = true
					}
				}
			})

			deselectAllBtn := widget.NewButton("Deselect All", func() {
				for id, check := range checkboxes {
					check.SetChecked(false)
					selections[id] = false
				}
			})

			markBtn := widget.NewButton("✓ Mark Selected as Living", func() {
				selectedCount := 0
				for _, selected := range selections {
					if selected {
						selectedCount++
					}
				}

				if selectedCount == 0 {
					dialog.ShowInformation("No Selection", "Please select at least one person to mark as living.", massMarkLivingDialog)
					return
				}

				dialog.ShowConfirm("Confirm Bulk Update",
					fmt.Sprintf("Are you sure you want to mark %d people as living?", selectedCount),
					func(confirmed bool) {
						if confirmed {
							updatedCount := 0
							for id, selected := range selections {
								if selected {
									person, err := s.GetPersonByID(id)
									if err == nil {
										person.IsLiving = true
										if err := s.UpdatePerson(person); err == nil {
											updatedCount++
										}
									}
								}
							}

							dialog.ShowInformation("Success",
								fmt.Sprintf("Successfully marked %d people as living.", updatedCount),
								massMarkLivingDialog)

							// Refresh and close
							refreshFunc()
							massMarkLivingDialog.Close()
						}
					}, massMarkLivingDialog)
			})

			selectionBox := container.NewHBox(selectAllBtn, deselectAllBtn, markBtn)
			content.Add(selectionBox)
			content.Add(widget.NewSeparator())

			// Add each candidate
			for _, cand := range candidates {
				candCopy := cand // Capture for closure

				// Restore previous selection state if it exists
				previouslySelected := selections[cand.person.ID]

				check := widget.NewCheck("", func(checked bool) {
					selections[candCopy.person.ID] = checked
				})
				check.SetChecked(previouslySelected)
				checkboxes[cand.person.ID] = check

				nameLabel := widget.NewLabel(fmt.Sprintf("%s (age %d, born %s)",
					formatPersonName(cand.person), cand.age, cand.person.BirthDate))

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

	// Sort options
	sortRadio := widget.NewRadioGroup([]string{"Sort Alphabetically (A-Z)", "Sort by Age (youngest first)"}, func(value string) {
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
			sortRadio,
			widget.NewSeparator(),
		),
		nil, nil, nil,
		scroll,
	)

	massMarkLivingDialog = fyne.CurrentApp().NewWindow("Mark as Living (Bulk)")
	massMarkLivingDialog.SetContent(mainContent)
	massMarkLivingDialog.Resize(fyne.NewSize(850, 700))
	massMarkLivingDialog.SetOnClosed(func() {
		massMarkLivingDialog = nil
	})
	massMarkLivingDialog.Show()
}

// Geographic Distribution Report
var geographicDialog fyne.Window

func showGeographicDistributionReport(w fyne.Window, s *store.Store, navigateFunc func(int64)) {
	// Check if already open
	if geographicDialog != nil {
		geographicDialog.RequestFocus()
		geographicDialog.Show()
		return
	}

	people, err := s.GetPeople()
	if err != nil {
		dialog.ShowError(fmt.Errorf("Failed to load people: %w", err), w)
		return
	}

	// Collect birth and death locations
	birthLocations := make(map[string][]store.Person)
	deathLocations := make(map[string][]store.Person)

	for _, p := range people {
		birthPlace := strings.TrimSpace(p.BirthPlace)
		if birthPlace != "" && birthPlace != "Unknown" {
			birthLocations[birthPlace] = append(birthLocations[birthPlace], p)
		}

		deathPlace := strings.TrimSpace(p.DeathPlace)
		if deathPlace != "" && deathPlace != "Unknown" && !p.IsLiving {
			deathLocations[deathPlace] = append(deathLocations[deathPlace], p)
		}
	}

	// State variables for filtering and sorting
	filterText := ""
	sortByCount := true // true = by count, false = alphabetical
	showBirths := true
	showDeaths := true

	// Location stat struct
	type locationStat struct {
		place  string
		people []store.Person
		count  int
	}

	// Build content container and scroll
	var content *fyne.Container
	var scroll *container.Scroll
	var rebuildContent func()

	rebuildContent = func() {
		// Filter and sort birth locations
		var birthStats []locationStat
		for place, peopleList := range birthLocations {
			if filterText == "" || strings.Contains(strings.ToLower(place), strings.ToLower(filterText)) {
				birthStats = append(birthStats, locationStat{place, peopleList, len(peopleList)})
			}
		}

		if sortByCount {
			sort.Slice(birthStats, func(i, j int) bool {
				return birthStats[i].count > birthStats[j].count
			})
		} else {
			sort.Slice(birthStats, func(i, j int) bool {
				return strings.ToLower(birthStats[i].place) < strings.ToLower(birthStats[j].place)
			})
		}

		// Filter and sort death locations
		var deathStats []locationStat
		for place, peopleList := range deathLocations {
			if filterText == "" || strings.Contains(strings.ToLower(place), strings.ToLower(filterText)) {
				deathStats = append(deathStats, locationStat{place, peopleList, len(peopleList)})
			}
		}

		if sortByCount {
			sort.Slice(deathStats, func(i, j int) bool {
				return deathStats[i].count > deathStats[j].count
			})
		} else {
			sort.Slice(deathStats, func(i, j int) bool {
				return strings.ToLower(deathStats[i].place) < strings.ToLower(deathStats[j].place)
			})
		}

		// Clear and rebuild content
		content.Objects = nil

		// Title
		title := widget.NewLabelWithStyle("🗺️  Geographic Distribution: Birth & Death Locations",
			fyne.TextAlignCenter, fyne.TextStyle{Bold: true})
		content.Add(title)
		content.Add(widget.NewSeparator())

		// Summary
		summary := widget.NewLabel(fmt.Sprintf("Total: %d birth locations, %d death locations (showing: %d birth, %d death)",
			len(birthLocations), len(deathLocations), len(birthStats), len(deathStats)))
		content.Add(summary)
		content.Add(widget.NewSeparator())

		// Birth Locations Section
		if showBirths {
			if len(birthStats) > 0 {
				birthSection := widget.NewLabelWithStyle("🏠 Birth Locations:",
					fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
				content.Add(birthSection)

				for _, stat := range birthStats {
					statCopy := stat // Capture for closure

					placeLabel := widget.NewLabelWithStyle(
						fmt.Sprintf("%s (%d people)", stat.place, stat.count),
						fyne.TextAlignLeading, fyne.TextStyle{Bold: true})

					expandBtn := widget.NewButton("Show People", func() {
						showLocationPeopleDialog(geographicDialog, statCopy.place, "Born", statCopy.people, navigateFunc)
					})

					placeBox := container.NewVBox(placeLabel, expandBtn)
					content.Add(placeBox)
				}
				content.Add(widget.NewSeparator())
			} else if filterText != "" {
				content.Add(widget.NewLabel("No birth locations match the filter."))
				content.Add(widget.NewSeparator())
			} else {
				content.Add(widget.NewLabel("No birth locations recorded."))
				content.Add(widget.NewSeparator())
			}
		}

		// Death Locations Section
		if showDeaths {
			if len(deathStats) > 0 {
				deathSection := widget.NewLabelWithStyle("⚰️  Death Locations:",
					fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
				content.Add(deathSection)

				for _, stat := range deathStats {
					statCopy := stat // Capture for closure

					placeLabel := widget.NewLabelWithStyle(
						fmt.Sprintf("%s (%d people)", stat.place, stat.count),
						fyne.TextAlignLeading, fyne.TextStyle{Bold: true})

					expandBtn := widget.NewButton("Show People", func() {
						showLocationPeopleDialog(geographicDialog, statCopy.place, "Died", statCopy.people, navigateFunc)
					})

					placeBox := container.NewVBox(placeLabel, expandBtn)
					content.Add(placeBox)
				}
			} else if filterText != "" {
				content.Add(widget.NewLabel("No death locations match the filter."))
			} else {
				content.Add(widget.NewLabel("No death locations recorded."))
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
	filterEntry.SetPlaceHolder("Filter by location name...")
	filterEntry.OnChanged = func(text string) {
		filterText = text
		rebuildContent()
	}

	// Sort options
	sortRadio := widget.NewRadioGroup([]string{"Sort by Count (most common first)", "Sort Alphabetically"}, func(value string) {
		sortByCount = (value == "Sort by Count (most common first)")
		rebuildContent()
	})
	sortRadio.SetSelected("Sort by Count (most common first)")

	// Show/Hide options
	showBirthsCheck := widget.NewCheck("Show Birth Locations", func(checked bool) {
		showBirths = checked
		rebuildContent()
	})
	showBirthsCheck.SetChecked(true)

	showDeathsCheck := widget.NewCheck("Show Death Locations", func(checked bool) {
		showDeaths = checked
		rebuildContent()
	})
	showDeathsCheck.SetChecked(true)

	displayOptions := container.NewHBox(showBirthsCheck, showDeathsCheck)

	// Initial build
	rebuildContent()

	scroll = container.NewVScroll(content)
	scroll.SetMinSize(fyne.NewSize(700, 400))

	// Main layout with controls at top
	mainContent := container.NewBorder(
		container.NewVBox(
			filterEntry,
			sortRadio,
			displayOptions,
			widget.NewSeparator(),
		),
		nil, nil, nil,
		scroll,
	)

	geographicDialog = fyne.CurrentApp().NewWindow("Geographic Distribution Report")
	geographicDialog.SetContent(mainContent)
	geographicDialog.Resize(fyne.NewSize(850, 700))
	geographicDialog.SetOnClosed(func() {
		geographicDialog = nil
	})
	geographicDialog.Show()
}

func showLocationPeopleDialog(parentWindow fyne.Window, place string, eventType string, people []store.Person, navigateFunc func(int64)) {
	content := container.NewVBox()

	title := widget.NewLabelWithStyle(
		fmt.Sprintf("People %s in: %s", eventType, place),
		fyne.TextAlignCenter, fyne.TextStyle{Bold: true})
	content.Add(title)
	content.Add(widget.NewSeparator())

	// Add each person
	for _, p := range people {
		personCopy := p // Capture for closure

		nameBtn := widget.NewButton(formatPersonName(p), func() {
			navigateFunc(personCopy.ID)
		})

		dateInfo := ""
		if eventType == "Born" && p.BirthDate != "" {
			dateInfo = fmt.Sprintf("  Born: %s", p.BirthDate)
		} else if eventType == "Died" && p.DeathDate != "" {
			dateInfo = fmt.Sprintf("  Died: %s", p.DeathDate)
		}

		personBox := container.NewVBox(
			nameBtn,
			widget.NewLabel(dateInfo),
		)
		content.Add(personBox)
		content.Add(widget.NewSeparator())
	}

	scroll := container.NewVScroll(content)
	scroll.SetMinSize(fyne.NewSize(500, 400))

	dialog.ShowCustom(fmt.Sprintf("%s in %s", eventType, place), "Close", scroll, parentWindow)
}

// Recent People Report
var recentPeopleDialog fyne.Window

func showRecentPeopleReport(w fyne.Window, s *store.Store, navigateFunc func(int64)) {
	// Check if already open
	if recentPeopleDialog != nil {
		recentPeopleDialog.RequestFocus()
		recentPeopleDialog.Show()
		return
	}

	// Get recent people (last 20)
	recentPeople, err := s.GetRecentPeople(20)
	if err != nil {
		dialog.ShowError(err, w)
		return
	}

	// Build content
	content := container.NewVBox()

	title := widget.NewLabelWithStyle("Recent People",
		fyne.TextAlignCenter, fyne.TextStyle{Bold: true})
	content.Add(title)

	if len(recentPeople) == 0 {
		content.Add(widget.NewLabel("No recently accessed people."))
		content.Add(widget.NewLabel("People you view will appear here for quick access."))
	} else {
		summary := widget.NewLabel(fmt.Sprintf("Showing %d recently accessed people:", len(recentPeople)))
		content.Add(summary)
		content.Add(widget.NewSeparator())

		// Display people in order of access
		for i, p := range recentPeople {
			personCopy := p // Capture for closure

			// Format access time
			accessTime := ""
			if p.LastAccessed != nil {
				accessTime = p.LastAccessed.Format("Jan 2, 2006 3:04 PM")
			}

			// Show bookmark indicator
			bookmarkIndicator := ""
			if p.Bookmarked {
				bookmarkIndicator = "★ "
			}

			nameBtn := widget.NewButton(
				fmt.Sprintf("%d. %s%s", i+1, bookmarkIndicator, formatPersonName(p)),
				func() {
					navigateFunc(personCopy.ID)
					// Close the dialog after navigation
					if recentPeopleDialog != nil {
						recentPeopleDialog.Close()
					}
				})

			// Show birth/death info and access time
			infoText := formatPersonInfo(p)
			if accessTime != "" {
				infoText = fmt.Sprintf("%s  •  Last viewed: %s", infoText, accessTime)
			}
			infoLabel := widget.NewLabel(infoText)

			personBox := container.NewVBox(
				nameBtn,
				infoLabel,
			)
			content.Add(personBox)
			content.Add(widget.NewSeparator())
		}
	}

	scroll := container.NewVScroll(content)
	scroll.SetMinSize(fyne.NewSize(600, 400))

	recentPeopleDialog = fyne.CurrentApp().NewWindow("Recent People")
	recentPeopleDialog.SetContent(scroll)
	recentPeopleDialog.Resize(fyne.NewSize(800, 600))
	recentPeopleDialog.SetOnClosed(func() {
		recentPeopleDialog = nil
	})
	recentPeopleDialog.Show()
}

// Bookmarked People Report
var bookmarkedPeopleDialog fyne.Window

func showBookmarkedPeopleReport(w fyne.Window, s *store.Store, navigateFunc func(int64)) {
	// Check if already open
	if bookmarkedPeopleDialog != nil {
		bookmarkedPeopleDialog.RequestFocus()
		bookmarkedPeopleDialog.Show()
		return
	}

	// Get bookmarked people
	bookmarkedPeople, err := s.GetBookmarkedPeople()
	if err != nil {
		dialog.ShowError(err, w)
		return
	}

	// Build content
	content := container.NewVBox()

	title := widget.NewLabelWithStyle("Bookmarked People",
		fyne.TextAlignCenter, fyne.TextStyle{Bold: true})
	content.Add(title)

	if len(bookmarkedPeople) == 0 {
		content.Add(widget.NewLabel("No bookmarked people."))
		content.Add(widget.NewLabel("Click the '⭐ Bookmark' button when viewing a person to add them here."))
	} else {
		summary := widget.NewLabel(fmt.Sprintf("Showing %d bookmarked people:", len(bookmarkedPeople)))
		content.Add(summary)
		content.Add(widget.NewSeparator())

		// Display people alphabetically (sorted by GetBookmarkedPeople)
		for i, p := range bookmarkedPeople {
			personCopy := p // Capture for closure

			nameBtn := widget.NewButton(
				fmt.Sprintf("%d. ★ %s", i+1, formatPersonName(p)),
				func() {
					navigateFunc(personCopy.ID)
					// Close the dialog after navigation
					if bookmarkedPeopleDialog != nil {
						bookmarkedPeopleDialog.Close()
					}
				})

			// Show birth/death info
			infoLabel := widget.NewLabel(formatPersonInfo(p))

			personBox := container.NewVBox(
				nameBtn,
				infoLabel,
			)
			content.Add(personBox)
			content.Add(widget.NewSeparator())
		}
	}

	scroll := container.NewVScroll(content)
	scroll.SetMinSize(fyne.NewSize(600, 400))

	bookmarkedPeopleDialog = fyne.CurrentApp().NewWindow("Bookmarked People")
	bookmarkedPeopleDialog.SetContent(scroll)
	bookmarkedPeopleDialog.Resize(fyne.NewSize(800, 600))
	bookmarkedPeopleDialog.SetOnClosed(func() {
		bookmarkedPeopleDialog = nil
	})
	bookmarkedPeopleDialog.Show()
}

// showTodoManager shows the research todo manager dialog for a person.
var todoManagerWindows = make(map[int64]fyne.Window) // Track open windows by person ID

func showTodoManager(parentWindow fyne.Window, s *store.Store, personID int64, personName string) {
	// Check if window already open for this person
	if existingWindow, exists := todoManagerWindows[personID]; exists {
		existingWindow.RequestFocus()
		existingWindow.Show()
		return
	}

	// Create new window
	todoWindow := fyne.CurrentApp().NewWindow(fmt.Sprintf("Research To-Do: %s", personName))

	// Function to rebuild content
	var rebuildContent func()
	rebuildContent = func() {
		// Reload todos
		todos, err := s.GetTodosForPerson(personID)
		if err != nil {
			dialog.ShowError(fmt.Errorf("Failed to load todos: %w", err), parentWindow)
			return
		}

		content := container.NewVBox()

		// Header
		header := widget.NewLabelWithStyle(
			fmt.Sprintf("Research To-Do List (%d items)", len(todos)),
			fyne.TextAlignCenter, fyne.TextStyle{Bold: true})
		content.Add(header)
		content.Add(widget.NewSeparator())

		// Add New button
		addBtn := widget.NewButton("+ Add New To-Do", func() {
			showAddTodoDialog(todoWindow, s, personID, personName, rebuildContent)
		})
		content.Add(addBtn)
		content.Add(widget.NewSeparator())

		if len(todos) == 0 {
			content.Add(widget.NewLabel("No research tasks yet. Click '+ Add New To-Do' to create one."))
		} else {
			// Display todos
			for _, todo := range todos {
				todoCopy := todo // Capture for closure
				todoCard := makeTodoCard(todoCopy, s, rebuildContent, todoWindow)
				content.Add(todoCard)
				content.Add(widget.NewSeparator())
			}
		}

		scroll := container.NewVScroll(content)
		scroll.SetMinSize(fyne.NewSize(700, 500))
		todoWindow.SetContent(scroll)
	}

	rebuildContent()
	todoWindow.Resize(fyne.NewSize(800, 600))
	todoWindow.SetOnClosed(func() {
		delete(todoManagerWindows, personID)
	})

	todoManagerWindows[personID] = todoWindow
	todoWindow.Show()
}

// makeTodoCard creates a card widget for displaying a todo item.
func makeTodoCard(todo store.ResearchTodo, s *store.Store, refresh func(), w fyne.Window) fyne.CanvasObject {
	// Priority indicator and status
	priorityEmoji := "●"
	switch todo.Priority {
	case "high":
		priorityEmoji = "🔴"
	case "medium":
		priorityEmoji = "🟡"
	case "low":
		priorityEmoji = "🟢"
	}

	statusText := "Pending"
	if todo.Status == "completed" {
		statusText = "✓ Completed"
		priorityEmoji = "✓"
	}

	// Description
	description := widget.NewLabelWithStyle(
		fmt.Sprintf("%s %s", priorityEmoji, todo.Description),
		fyne.TextAlignLeading, fyne.TextStyle{Bold: todo.Status == "pending"})
	description.Wrapping = fyne.TextWrapWord

	// Details
	detailsText := fmt.Sprintf("Priority: %s | Status: %s", todo.Priority, statusText)
	if todo.CompletedAt != nil {
		detailsText += fmt.Sprintf(" | Completed: %s", todo.CompletedAt.Format("Jan 2, 2006"))
	}
	details := widget.NewLabel(detailsText)
	details.TextStyle.Italic = true

	// Notes (if any)
	var notesWidget fyne.CanvasObject
	if todo.Notes != "" {
		notesLabel := widget.NewLabel("Notes: " + todo.Notes)
		notesLabel.Wrapping = fyne.TextWrapWord
		notesWidget = notesLabel
	}

	// Buttons
	var buttons *fyne.Container
	if todo.Status == "pending" {
		completeBtn := widget.NewButton("✓ Mark Complete", func() {
			if err := s.MarkTodoComplete(todo.ID); err != nil {
				dialog.ShowError(err, w)
			} else {
				refresh()
			}
		})

		editBtn := widget.NewButton("Edit", func() {
			showEditTodoDialog(w, s, &todo, refresh)
		})

		deleteBtn := widget.NewButton("Delete", func() {
			dialog.ShowConfirm("Delete To-Do",
				"Are you sure you want to delete this research task?",
				func(confirmed bool) {
					if confirmed {
						if err := s.DeleteTodo(todo.ID); err != nil {
							dialog.ShowError(err, w)
						} else {
							refresh()
						}
					}
				}, w)
		})

		buttons = container.NewHBox(completeBtn, editBtn, deleteBtn)
	} else {
		// Completed todo - allow uncomplete or delete
		uncompleteBtn := widget.NewButton("↶ Mark Pending", func() {
			if err := s.MarkTodoPending(todo.ID); err != nil {
				dialog.ShowError(err, w)
			} else {
				refresh()
			}
		})

		deleteBtn := widget.NewButton("Delete", func() {
			dialog.ShowConfirm("Delete To-Do",
				"Are you sure you want to delete this completed research task?",
				func(confirmed bool) {
					if confirmed {
						if err := s.DeleteTodo(todo.ID); err != nil {
							dialog.ShowError(err, w)
						} else {
							refresh()
						}
					}
				}, w)
		})

		buttons = container.NewHBox(uncompleteBtn, deleteBtn)
	}

	// Build card
	card := container.NewVBox(description, details)
	if notesWidget != nil {
		card.Add(notesWidget)
	}
	card.Add(buttons)

	return card
}

// showAddTodoDialog shows a dialog to add a new todo.
func showAddTodoDialog(w fyne.Window, s *store.Store, personID int64, personName string, onSave func()) {
	descEntry := widget.NewEntry()
	descEntry.SetPlaceHolder("e.g., Find birth certificate, Verify marriage date, Check census records")
	descEntry.MultiLine = true
	descEntry.SetMinRowsVisible(2)

	prioritySelect := widget.NewSelect([]string{"low", "medium", "high"}, func(string) {})
	prioritySelect.SetSelected("medium")

	notesEntry := widget.NewMultiLineEntry()
	notesEntry.SetPlaceHolder("Additional notes or details (optional)")
	notesEntry.SetMinRowsVisible(3)

	form := container.NewVBox(
		widget.NewLabel("Description:"),
		descEntry,
		widget.NewLabel("Priority:"),
		prioritySelect,
		widget.NewLabel("Notes:"),
		notesEntry,
	)

	dlg := dialog.NewCustomConfirm(
		fmt.Sprintf("Add Research To-Do for %s", personName),
		"Save",
		"Cancel",
		form,
		func(save bool) {
			if !save {
				return
			}

			description := strings.TrimSpace(descEntry.Text)
			if description == "" {
				dialog.ShowError(fmt.Errorf("Description is required"), w)
				return
			}

			todo := &store.ResearchTodo{
				PersonID:    personID,
				Description: description,
				Priority:    prioritySelect.Selected,
				Status:      "pending",
				Notes:       strings.TrimSpace(notesEntry.Text),
			}

			if err := s.CreateTodo(todo); err != nil {
				dialog.ShowError(fmt.Errorf("Failed to create todo: %w", err), w)
				return
			}

			onSave()
		},
		w)

	dlg.Resize(fyne.NewSize(600, 400))
	dlg.Show()
}

// showEditTodoDialog shows a dialog to edit an existing todo.
func showEditTodoDialog(w fyne.Window, s *store.Store, todo *store.ResearchTodo, onSave func()) {
	descEntry := widget.NewEntry()
	descEntry.SetText(todo.Description)
	descEntry.MultiLine = true
	descEntry.SetMinRowsVisible(2)

	prioritySelect := widget.NewSelect([]string{"low", "medium", "high"}, func(string) {})
	prioritySelect.SetSelected(todo.Priority)

	notesEntry := widget.NewMultiLineEntry()
	notesEntry.SetText(todo.Notes)
	notesEntry.SetMinRowsVisible(3)

	form := container.NewVBox(
		widget.NewLabel("Description:"),
		descEntry,
		widget.NewLabel("Priority:"),
		prioritySelect,
		widget.NewLabel("Notes:"),
		notesEntry,
	)

	dlg := dialog.NewCustomConfirm(
		"Edit Research To-Do",
		"Save",
		"Cancel",
		form,
		func(save bool) {
			if !save {
				return
			}

			description := strings.TrimSpace(descEntry.Text)
			if description == "" {
				dialog.ShowError(fmt.Errorf("Description is required"), w)
				return
			}

			todo.Description = description
			todo.Priority = prioritySelect.Selected
			todo.Notes = strings.TrimSpace(notesEntry.Text)

			if err := s.UpdateTodo(todo); err != nil {
				dialog.ShowError(fmt.Errorf("Failed to update todo: %w", err), w)
				return
			}

			onSave()
		},
		w)

	dlg.Resize(fyne.NewSize(600, 400))
	dlg.Show()
}

// showCompletedTodosReport shows all completed research todos grouped by person.
var completedTodosDialog fyne.Window

func showCompletedTodosReport(w fyne.Window, s *store.Store, navigateFunc func(int64)) {
	// Check if already open
	if completedTodosDialog != nil {
		completedTodosDialog.RequestFocus()
		completedTodosDialog.Show()
		return
	}

	// Function to build/rebuild content
	var buildContent func() fyne.CanvasObject
	buildContent = func() fyne.CanvasObject {
		// Get all completed todos
		todos, err := s.GetAllCompletedTodos()
		if err != nil {
			return widget.NewLabel(fmt.Sprintf("Error loading completed todos: %v", err))
		}

		content := container.NewVBox()

		title := widget.NewLabelWithStyle("Completed Research To-Dos",
			fyne.TextAlignCenter, fyne.TextStyle{Bold: true})
		content.Add(title)

		if len(todos) == 0 {
			content.Add(widget.NewSeparator())
			content.Add(widget.NewLabel("No completed research tasks."))
			content.Add(widget.NewLabel("Completed tasks will appear here for reference."))
		} else {
			summary := widget.NewLabel(fmt.Sprintf("Showing %d completed research tasks:", len(todos)))
			content.Add(summary)
			content.Add(widget.NewSeparator())

			// Add "Delete All Completed" button
			deleteAllBtn := widget.NewButton("🗑️ Delete All Completed To-Dos", func() {
				dialog.ShowConfirm("Delete All Completed To-Dos",
					fmt.Sprintf("Are you sure you want to permanently delete all %d completed research tasks?\n\nThis cannot be undone.", len(todos)),
					func(confirmed bool) {
						if confirmed {
							// Delete all completed todos
							for _, todo := range todos {
								_ = s.DeleteTodo(todo.ID)
							}
							// Refresh the display
							completedTodosDialog.SetContent(container.NewVScroll(buildContent()))
						}
					}, completedTodosDialog)
			})
			content.Add(deleteAllBtn)
			content.Add(widget.NewSeparator())

			// Group todos by person
			type PersonTodos struct {
				PersonID   int64
				PersonName string
				Todos      []store.ResearchTodo
			}

			personMap := make(map[int64]*PersonTodos)
			var personOrder []int64 // Track order

			for _, todo := range todos {
				if _, exists := personMap[todo.PersonID]; !exists {
					person, _ := s.GetPersonByID(todo.PersonID)
					personName := "Unknown"
					if person != nil {
						personName = formatPersonName(*person)
					}
					personMap[todo.PersonID] = &PersonTodos{
						PersonID:   todo.PersonID,
						PersonName: personName,
						Todos:      []store.ResearchTodo{},
					}
					personOrder = append(personOrder, todo.PersonID)
				}
				personMap[todo.PersonID].Todos = append(personMap[todo.PersonID].Todos, todo)
			}

			// Display by person
			for _, personID := range personOrder {
				pt := personMap[personID]

				// Person header with navigation button
				personBtn := widget.NewButton(
					fmt.Sprintf("👤 %s (%d completed)", pt.PersonName, len(pt.Todos)),
					func() {
						navigateFunc(pt.PersonID)
						if completedTodosDialog != nil {
							completedTodosDialog.Close()
						}
					})
				personBtn.Importance = widget.HighImportance

				content.Add(personBtn)

				// List todos for this person
				for _, todo := range pt.Todos {
					todoCopy := todo // Capture for closure

					// Completed date
					completedDate := "Unknown"
					if todoCopy.CompletedAt != nil {
						completedDate = todoCopy.CompletedAt.Format("Jan 2, 2006")
					}

					// Task description
					taskLabel := widget.NewLabel(fmt.Sprintf("    ✓ %s", todoCopy.Description))
					taskLabel.Wrapping = fyne.TextWrapWord

					// Details
					detailsLabel := widget.NewLabel(fmt.Sprintf("        Completed: %s | Priority: %s", completedDate, todoCopy.Priority))
					detailsLabel.TextStyle.Italic = true

					// Notes if any
					var notesWidget fyne.CanvasObject
					if todoCopy.Notes != "" {
						notesLabel := widget.NewLabel(fmt.Sprintf("        Notes: %s", todoCopy.Notes))
						notesLabel.Wrapping = fyne.TextWrapWord
						notesLabel.TextStyle.Italic = true
						notesWidget = notesLabel
					}

					// Delete button
					deleteBtn := widget.NewButton("Delete", func() {
						dialog.ShowConfirm("Delete Completed To-Do",
							fmt.Sprintf("Delete this completed task?\n\n%s", todoCopy.Description),
							func(confirmed bool) {
								if confirmed {
									if err := s.DeleteTodo(todoCopy.ID); err != nil {
										dialog.ShowError(err, completedTodosDialog)
									} else {
										// Refresh the display
										completedTodosDialog.SetContent(container.NewVScroll(buildContent()))
									}
								}
							}, completedTodosDialog)
					})

					todoBox := container.NewVBox(taskLabel, detailsLabel)
					if notesWidget != nil {
						todoBox.Add(notesWidget)
					}
					todoBox.Add(container.NewHBox(deleteBtn))

					content.Add(todoBox)
				}

				content.Add(widget.NewSeparator())
			}
		}

		return content
	}

	scroll := container.NewVScroll(buildContent())
	scroll.SetMinSize(fyne.NewSize(700, 400))

	completedTodosDialog = fyne.CurrentApp().NewWindow("Completed Research To-Dos")
	completedTodosDialog.SetContent(scroll)
	completedTodosDialog.Resize(fyne.NewSize(900, 600))
	completedTodosDialog.SetOnClosed(func() {
		completedTodosDialog = nil
	})
	completedTodosDialog.Show()
}

// showAllTodosReport shows all pending research todos across all people.
var allTodosDialog fyne.Window

func showAllTodosReport(w fyne.Window, s *store.Store, navigateFunc func(int64)) {
	// Check if already open
	if allTodosDialog != nil {
		allTodosDialog.RequestFocus()
		allTodosDialog.Show()
		return
	}

	// Get all pending todos
	todos, err := s.GetAllPendingTodos()
	if err != nil {
		dialog.ShowError(fmt.Errorf("Failed to load todos: %w", err), w)
		return
	}

	// Build content
	content := container.NewVBox()

	title := widget.NewLabelWithStyle("All Pending Research To-Dos",
		fyne.TextAlignCenter, fyne.TextStyle{Bold: true})
	content.Add(title)

	if len(todos) == 0 {
		content.Add(widget.NewSeparator())
		content.Add(widget.NewLabel("No pending research tasks."))
		content.Add(widget.NewLabel("Add research to-dos from individual person edit dialogs."))
	} else {
		summary := widget.NewLabel(fmt.Sprintf("Showing %d pending research tasks:", len(todos)))
		content.Add(summary)
		content.Add(widget.NewSeparator())

		// Group by priority
		highPriority := []store.ResearchTodo{}
		mediumPriority := []store.ResearchTodo{}
		lowPriority := []store.ResearchTodo{}

		for _, todo := range todos {
			switch todo.Priority {
			case "high":
				highPriority = append(highPriority, todo)
			case "medium":
				mediumPriority = append(mediumPriority, todo)
			case "low":
				lowPriority = append(lowPriority, todo)
			}
		}

		// Display by priority
		if len(highPriority) > 0 {
			priorityLabel := widget.NewLabelWithStyle("🔴 High Priority",
				fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
			content.Add(priorityLabel)

			for i, todo := range highPriority {
				todoCopy := todo
				person, _ := s.GetPersonByID(todoCopy.PersonID)
				personName := "Unknown"
				if person != nil {
					personName = formatPersonName(*person)
				}

				taskBtn := widget.NewButton(
					fmt.Sprintf("%d. %s", i+1, todoCopy.Description),
					func() {
						navigateFunc(todoCopy.PersonID)
						// Close the dialog after navigation
						if allTodosDialog != nil {
							allTodosDialog.Close()
						}
					})

				personLabel := widget.NewLabel(fmt.Sprintf("    Person: %s", personName))
				personLabel.TextStyle.Italic = true

				content.Add(taskBtn)
				content.Add(personLabel)
				if todoCopy.Notes != "" {
					notesLabel := widget.NewLabel(fmt.Sprintf("    Notes: %s", todoCopy.Notes))
					notesLabel.Wrapping = fyne.TextWrapWord
					content.Add(notesLabel)
				}
				content.Add(widget.NewSeparator())
			}
		}

		if len(mediumPriority) > 0 {
			priorityLabel := widget.NewLabelWithStyle("🟡 Medium Priority",
				fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
			content.Add(priorityLabel)

			for i, todo := range mediumPriority {
				todoCopy := todo
				person, _ := s.GetPersonByID(todoCopy.PersonID)
				personName := "Unknown"
				if person != nil {
					personName = formatPersonName(*person)
				}

				taskBtn := widget.NewButton(
					fmt.Sprintf("%d. %s", i+1, todoCopy.Description),
					func() {
						navigateFunc(todoCopy.PersonID)
						if allTodosDialog != nil {
							allTodosDialog.Close()
						}
					})

				personLabel := widget.NewLabel(fmt.Sprintf("    Person: %s", personName))
				personLabel.TextStyle.Italic = true

				content.Add(taskBtn)
				content.Add(personLabel)
				if todoCopy.Notes != "" {
					notesLabel := widget.NewLabel(fmt.Sprintf("    Notes: %s", todoCopy.Notes))
					notesLabel.Wrapping = fyne.TextWrapWord
					content.Add(notesLabel)
				}
				content.Add(widget.NewSeparator())
			}
		}

		if len(lowPriority) > 0 {
			priorityLabel := widget.NewLabelWithStyle("🟢 Low Priority",
				fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
			content.Add(priorityLabel)

			for i, todo := range lowPriority {
				todoCopy := todo
				person, _ := s.GetPersonByID(todoCopy.PersonID)
				personName := "Unknown"
				if person != nil {
					personName = formatPersonName(*person)
				}

				taskBtn := widget.NewButton(
					fmt.Sprintf("%d. %s", i+1, todoCopy.Description),
					func() {
						navigateFunc(todoCopy.PersonID)
						if allTodosDialog != nil {
							allTodosDialog.Close()
						}
					})

				personLabel := widget.NewLabel(fmt.Sprintf("    Person: %s", personName))
				personLabel.TextStyle.Italic = true

				content.Add(taskBtn)
				content.Add(personLabel)
				if todoCopy.Notes != "" {
					notesLabel := widget.NewLabel(fmt.Sprintf("    Notes: %s", todoCopy.Notes))
					notesLabel.Wrapping = fyne.TextWrapWord
					content.Add(notesLabel)
				}
				content.Add(widget.NewSeparator())
			}
		}
	}

	scroll := container.NewVScroll(content)
	scroll.SetMinSize(fyne.NewSize(700, 400))

	allTodosDialog = fyne.CurrentApp().NewWindow("All Pending Research To-Dos")
	allTodosDialog.SetContent(scroll)
	allTodosDialog.Resize(fyne.NewSize(900, 600))
	allTodosDialog.SetOnClosed(func() {
		allTodosDialog = nil
	})
	allTodosDialog.Show()
}

// showUnsourcedPeopleReport shows all people without any source citations.
var unsourcedPeopleDialog fyne.Window

func showUnsourcedPeopleReport(w fyne.Window, s *store.Store, navigateFunc func(int64)) {
	// Check if already open
	if unsourcedPeopleDialog != nil {
		unsourcedPeopleDialog.RequestFocus()
		unsourcedPeopleDialog.Show()
		return
	}

	// Get all people
	allPeople, err := s.GetPeople()
	if err != nil {
		dialog.ShowError(fmt.Errorf("Failed to load people: %w", err), w)
		return
	}

	// Filter for people without sources
	var unsourcedPeople []store.Person
	for _, person := range allPeople {
		count, err := s.CountCitationsForPerson(person.ID)
		if err == nil && count == 0 {
			unsourcedPeople = append(unsourcedPeople, person)
		}
	}

	// Build content
	content := container.NewVBox()

	title := widget.NewLabelWithStyle("People Without Sources",
		fyne.TextAlignCenter, fyne.TextStyle{Bold: true})
	content.Add(title)
	content.Add(widget.NewSeparator())

	if len(unsourcedPeople) == 0 {
		content.Add(widget.NewLabel("All people have at least one source citation!"))
		content.Add(widget.NewLabel("Great work on documenting your research."))
	} else {
		summary := widget.NewLabel(fmt.Sprintf("%d people without sources (%.1f%% of total):",
			len(unsourcedPeople),
			float64(len(unsourcedPeople))/float64(len(allPeople))*100))
		content.Add(summary)
		content.Add(widget.NewSeparator())

		// Display unsourced people
		for _, person := range unsourcedPeople {
			personCopy := person // Capture for closure
			personName := formatPersonName(personCopy)
			info := formatPersonInfo(personCopy)

			buttonText := personName
			if info != "" {
				buttonText += fmt.Sprintf(" (%s)", info)
			}

			personBtn := widget.NewButton(buttonText, func() {
				navigateFunc(personCopy.ID)
				if unsourcedPeopleDialog != nil {
					unsourcedPeopleDialog.Close()
				}
			})

			content.Add(personBtn)
		}
	}

	scroll := container.NewVScroll(content)
	scroll.SetMinSize(fyne.NewSize(700, 400))

	unsourcedPeopleDialog = fyne.CurrentApp().NewWindow("People Without Sources")
	unsourcedPeopleDialog.SetContent(scroll)
	unsourcedPeopleDialog.Resize(fyne.NewSize(900, 600))
	unsourcedPeopleDialog.SetOnClosed(func() {
		unsourcedPeopleDialog = nil
	})
	unsourcedPeopleDialog.Show()
}

// Helper function to format person info (birth/death)
func formatPersonInfo(p store.Person) string {
	info := ""
	if p.BirthDate != "" {
		info = fmt.Sprintf("b. %s", p.BirthDate)
		if p.BirthPlace != "" {
			info += fmt.Sprintf(" in %s", p.BirthPlace)
		}
	}
	if p.DeathDate != "" {
		if info != "" {
			info += "  •  "
		}
		info += fmt.Sprintf("d. %s", p.DeathDate)
		if p.DeathPlace != "" {
			info += fmt.Sprintf(" in %s", p.DeathPlace)
		}
	}
	if info == "" {
		info = "No dates recorded"
	}
	return info
}

func min(a, b, c int) int {
	if a < b && a < c {
		return a
	}
	if b < c {
		return b
	}
	return c
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
