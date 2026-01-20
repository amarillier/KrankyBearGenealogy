package main

import (
	"net/url"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

// showHelp displays comprehensive help documentation
// Reusable pattern from KrankyBearClock - customize these for your app:
//   - appName: Your application name
//   - resourceKrankyBearGenealogyPng: Your embedded icon resource
//   - helpText: Your application's help content (see below for structure)
//   - GitHub and License URLs
//
// Help text structure recommendation:
//   - Use section headers with visual separators (━━━)
//   - Group related features together
//   - Include tips, tricks, and known limitations
//   - Add keyboard shortcuts
//   - Provide links to external resources
func showHelp(a fyne.App) {
	if helpWindow != nil && helpWindow.Content().Visible() {
		helpWindow.Show()
		helpWindow.RequestFocus()
		return
	}

	helpWindow = a.NewWindow(appName + " - Help")
	helpWindow.SetIcon(resourceKrankyBearGenealogyPng)

	// Customize this help text for your application
	helpText := `KrankyBear Genealogy - Modern Family Tree Management

OVERVIEW:
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
A modern, cross-platform genealogy application inspired by
Personal Ancestral File (PAF). Store and visualize your family
history with ease.

THREE VIEWS:
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
• Family View
  - View current person with parents, spouses, and children
  - See marriage, divorce, and separation dates
  - Add/edit relationships directly
  - Navigate between family members

• Pedigree View
  - Visual ancestor chart showing 4 generations
  - Click any person to make them the focus
  - Double-click names to edit
  - Auto-scrollable for large families

• Individual View
  - Sortable table of all people
  - Filter by name in the left panel
  - Double-click to edit any record
  - Auto-scrolls to selected person

PERSON MANAGEMENT:
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
• Add/Edit/Delete People
  - Full name (given, surname, suffix)
  - Birth and death dates/places
  - Living status (auto-managed)
  - Notes field for additional information

• Smart Features
  - Auto-link new children to both parents
  - "Still living" defaults checked, unchecks with death date
  - Searchable person selection dialogs
  - Alphabetically sorted person list

RELATIONSHIPS:
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
• Multiple Marriages
  - Track marriage, divorce, separation dates
  - Multiple spouses per person
  - Select correct spouse when adding children

• Auto-Relationship Creation
  - Adding a child automatically creates child→parent links
  - Adding a spouse creates bidirectional spouse links
  - Children auto-link to both parents if married

DATA IMPORT/EXPORT:
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
• GEDCOM (.ged)
  - Standard genealogy format
  - Import existing family trees
  - Export for backup or sharing

• GenoPro (.gno)
  - Import from GenoPro software
  - Automatic deduplication
  - Preserves relationships and dates

DATABASE MANAGEMENT:
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
• Multiple Databases
  - Create new empty databases
  - Open different databases
  - Hot-reload without restarting

• Remembers State
  - Last opened database
  - Last focused person per database
  - Optional "Focus User" setting

SETTINGS:
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
• Startup Behavior
  - Open with last focused person
  - OR open with designated "Focus User"

• Focus User
  - Search by ID or name
  - Always start at your preferred person
  - Per-database setting

• Themes
  - Light, Dark, or System theme
  - Persisted across sessions

DATA QUALITY:
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
• Data Quality Report
  - Identifies incomplete records
  - Shows missing names, dates, places
  - Click to navigate directly to record
  - Helps maintain clean data

TIPS & TRICKS:
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
💡 Navigation:
   - Use left panel filter to find anyone quickly
   - Click names in any view to change focus
   - All three views stay synchronized

💡 Workflow:
   1. Import GEDCOM or start fresh
   2. Use Data Quality Report to find issues
   3. Navigate to incomplete records
   4. Edit and complete information
   5. Export GEDCOM as backup

💡 Multiple Marriages:
   - When adding a child, select correct spouse
   - Marriage dates appear between spouses
   - Divorce/separation dates tracked separately

💡 Searchable Dialogs:
   - Type to filter when selecting people
   - Or create new person directly
   - Auto-links relationships

KEYBOARD SHORTCUTS:
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
• Cmd/Ctrl+Q - Quit
• Cmd/Ctrl+W - Close window
• Enter - Submit search/form
• Double-click - Edit person

MORE INFORMATION:
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
For detailed documentation, bug reports, or feature requests:
📦 GitHub: https://github.com/amarillier/KrankyBearGenealogy
📄 License: https://github.com/amarillier/KrankyBearGenealogy/blob/main/LICENSE
📝 Release Notes: Check "Help → Check for Updates"

FREE SOFTWARE - Use anywhere, anytime, any purpose!
No registration, no tracking, no phone-home (except manual update checks).
`

	helpLabel := widget.NewLabel(helpText)
	helpLabel.Wrapping = fyne.TextWrapWord

	// Links - update URLs for your project
	githubURL, _ := url.Parse("https://github.com/amarillier/KrankyBearGenealogy")
	githubLink := widget.NewHyperlink("Visit GitHub Repository", githubURL)
	githubLink.Alignment = fyne.TextAlignCenter

	licenseURL, _ := url.Parse("https://github.com/amarillier/KrankyBearGenealogy/blob/main/LICENSE")
	licenseLink := widget.NewHyperlink("View License", licenseURL)
	licenseLink.Alignment = fyne.TextAlignCenter

	// Create scrollable area with minimum size for better readability
	scrollContent := container.NewScroll(helpLabel)
	scrollContent.SetMinSize(fyne.NewSize(750, 550))

	// Layout with better proportions
	header := container.NewVBox(
		widget.NewLabelWithStyle(appName+" - Help", fyne.TextAlignCenter, fyne.TextStyle{Bold: true}),
		widget.NewSeparator(),
	)

	footer := container.NewVBox(
		widget.NewSeparator(),
		container.NewCenter(container.NewHBox(githubLink, licenseLink)),
	)

	content := container.NewBorder(header, footer, nil, nil, scrollContent)

	helpWindow.SetContent(container.NewPadded(content))
	helpWindow.Resize(fyne.NewSize(850, 700))

	helpWindow.SetCloseIntercept(func() {
		helpWindow.Hide()
	})

	helpWindow.Show()
	helpWindow.RequestFocus() // Bring window to front
}

// "Now this is not the end. It is not even the beginning of the end. But it is, perhaps, the end of the beginning." Winston Churchill, November 10, 1942
