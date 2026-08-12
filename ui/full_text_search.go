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

var fullTextSearchWindow fyne.Window

// showFullTextSearchDialog shows the full-text search dialog.
func showFullTextSearchDialog(w fyne.Window, s *store.Store, onNavigate func(int64)) {
	// If window already exists and is visible, just bring it to front
	if fullTextSearchWindow != nil && fullTextSearchWindow.Content().Visible() {
		fullTextSearchWindow.Show()
		fullTextSearchWindow.RequestFocus()
		return
	}

	// Create a new window for search
	searchWin := fyne.CurrentApp().NewWindow("Full-Text Search")
	searchWin.Resize(fyne.NewSize(900, 700))
	fullTextSearchWindow = searchWin
	
	// Register for window lifecycle management
	RegisterSecondaryWindow(searchWin)

	// Search entry
	searchEntry := widget.NewEntry()
	searchEntry.SetPlaceHolder("Try: Durban, \"military service\", photo*, London OR Paris")

	// Results list
	var resultsList *widget.List
	var searchResults []store.FullTextSearchResult
	var resultsLabel *widget.Label

	resultsList = widget.NewList(
		func() int { return len(searchResults) },
		func() fyne.CanvasObject {
			return container.NewVBox(
				widget.NewLabel("Template Person Name"),
				widget.NewLabel("Template snippet..."),
			)
		},
		func(id widget.ListItemID, obj fyne.CanvasObject) {
			vbox := obj.(*fyne.Container)
			nameLabel := vbox.Objects[0].(*widget.Label)
			snippetLabel := vbox.Objects[1].(*widget.Label)

			result := searchResults[id]
			
			// Format name with field indicator
			nameText := formatPersonName(result.Person)
			if result.MatchedField != "" {
				nameText += fmt.Sprintf(" [%s]", result.MatchedField)
			}
			
			nameLabel.SetText(nameText)
			nameLabel.TextStyle.Bold = true
			
			// Format snippet with highlighted matches
			snippetText := result.Snippet
			if len(snippetText) > 200 {
				snippetText = snippetText[:200] + "..."
			}
			// Replace ** markers with visual emphasis
			snippetText = strings.ReplaceAll(snippetText, "**", "→")
			
			snippetLabel.SetText(snippetText)
			snippetLabel.TextStyle.Italic = true
			snippetLabel.Wrapping = fyne.TextWrapWord
		},
	)

	// Double-click to navigate
	resultsList.OnSelected = func(id widget.ListItemID) {
		if id >= 0 && id < len(searchResults) {
			result := searchResults[id]
			if onNavigate != nil {
				onNavigate(result.PersonID)
				searchWin.Hide() // Hide search window after navigation
			}
		}
		resultsList.UnselectAll() // Clear selection
	}

	// Search button
	searchBtn := widget.NewButton("Search", func() {
		query := strings.TrimSpace(searchEntry.Text)
		if query == "" {
			dialog.ShowInformation("Search Required", "Please enter search terms.", searchWin)
			return
		}

		// Perform search
		results, err := s.FullTextSearch(query, 100)
		if err != nil {
			dialog.ShowError(fmt.Errorf("Search failed: %v", err), searchWin)
			return
		}

		searchResults = results
		resultsList.Refresh()
		
		if len(results) == 0 {
			resultsLabel.SetText(fmt.Sprintf("No results found for '%s'", query))
		} else {
			resultsLabel.SetText(fmt.Sprintf("Found %d results for '%s'", len(results), query))
		}
	})
	searchBtn.Importance = widget.HighImportance

	// Clear button
	clearBtn := widget.NewButton("Clear", func() {
		searchEntry.SetText("")
		searchResults = []store.FullTextSearchResult{}
		resultsList.Refresh()
		resultsLabel.SetText("Enter search terms and click Search")
	})

	// Search on Enter key
	searchEntry.OnSubmitted = func(text string) {
		searchBtn.OnTapped()
	}

	// Info label
	infoLabel := widget.NewLabel(
		"Searches across all names, places, notes, marriages, spouse names, and contact info.")
	infoLabel.Wrapping = fyne.TextWrapWord
	infoLabel.TextStyle.Italic = true

	// Quick tips label
	quickTipsLabel := widget.NewLabel(
		"💡 Quick Tips: Multiple words = AND (e.g., \"Durban photographer\" finds both). " +
		"Use quotes for exact phrases: \"military service\". Click any result to navigate.")
	quickTipsLabel.Wrapping = fyne.TextWrapWord
	
	// Advanced syntax examples (accordion style)
	advancedSyntaxLabel := widget.NewLabel(
		"📖 Advanced Query Syntax:\n\n" +
		"Multiple Words (AND):\n" +
		"  Durban South Africa → finds all three words\n\n" +
		"Exact Phrase:\n" +
		"  \"military service\" → exact phrase in quotes\n\n" +
		"OR Logic:\n" +
		"  London OR Paris → finds either city (or both)\n\n" +
		"Exclude (NOT):\n" +
		"  Durban -Natal → finds Durban but excludes Natal\n\n" +
		"Prefix Matching:\n" +
		"  photo* → finds photo, photographer, photography\n\n" +
		"Complex Boolean:\n" +
		"  Durban AND (photographer OR artist)\n" +
		"  (London OR Paris) AND \"world war\"\n\n" +
		"Case Insensitive:\n" +
		"  All searches ignore case automatically")
	advancedSyntaxLabel.Wrapping = fyne.TextWrapWord
	advancedSyntaxLabel.TextStyle.Monospace = false
	advancedSyntaxLabel.Hide() // Hidden by default
	
	// Toggle button for advanced syntax
	var syntaxToggleBtn *widget.Button
	syntaxToggleBtn = widget.NewButton("Show Advanced Query Syntax", func() {
		if advancedSyntaxLabel.Visible() {
			advancedSyntaxLabel.Hide()
			syntaxToggleBtn.SetText("Show Advanced Query Syntax")
		} else {
			advancedSyntaxLabel.Show()
			syntaxToggleBtn.SetText("Hide Advanced Query Syntax")
		}
	})

	// Layout
	header := widget.NewLabelWithStyle("Full-Text Search", fyne.TextAlignCenter, fyne.TextStyle{Bold: true})

	searchForm := container.NewVBox(
		widget.NewLabel("Search for:"),
		searchEntry,
		container.NewHBox(searchBtn, clearBtn),
		widget.NewSeparator(),
		infoLabel,
		quickTipsLabel,
		syntaxToggleBtn,
		advancedSyntaxLabel,
	)

	resultsLabel = widget.NewLabel("Enter search terms and click Search")
	resultsContainer := container.NewBorder(
		container.NewVBox(resultsLabel, widget.NewSeparator()),
		nil, nil, nil,
		container.NewVScroll(resultsList),
	)

	content := container.NewBorder(
		container.NewVBox(header, widget.NewSeparator(), searchForm, widget.NewSeparator()),
		nil, nil, nil,
		resultsContainer,
	)

	searchWin.SetContent(content)

	// Hide window instead of closing it
	searchWin.SetCloseIntercept(func() {
		searchWin.Hide()
	})

	searchWin.Show()
	searchWin.RequestFocus()
	
	// Focus the search entry
	searchEntry.FocusGained()
}
