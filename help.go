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

• Relationship Calculator (Tools → Relationship Calculator)
  - Calculate relationships between any two people
  - Detects relationships up to 3 generations:
    * Parents/children, grandparents/grandchildren
    * Great-grandparents/great-grandchildren
  - Identifies cousins (1st, 2nd, 3rd + "removed" variations)
  - Recognizes aunts/uncles, nieces/nephews, great-aunts/uncles
  - Shows in-law relationships with descriptive context
  - Displays common ancestors and line of descent
  - For deeper ancestry (4+ generations), use Pedigree View

MEDIA MANAGEMENT:
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
• Photos & Documents
  - Attach images, PDFs, videos, Office documents
  - Store in database or link to external files
  - Many-to-many linking (one photo → multiple people)
  - Automatic thumbnail generation

• Supported Formats
  - Images: JPEG, PNG, GIF, WebP
  - Videos: MP4, MOV
  - Documents: PDF, Word, Excel, PowerPoint

• Media Features
  - 📷 Visual indicators show who has media
  - Media Library: View and manage all media
  - Add Media button: Attach media to multiple people at once
  - View Media: See all photos/documents for current person

DATA IMPORT/EXPORT:
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
• GEDCOM (.ged)
  - Standard genealogy format
  - Import existing family trees
  - Export for backup or sharing
  - Branch export (person + descendants only)

• GenoPro (.gno) & Gramps (.db)
  - Import from GenoPro or Gramps software
  - Automatic deduplication
  - Preserves relationships, dates, notes, media references

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

REPORTS & ANALYSIS:
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
• Data Quality Report
  - Identifies incomplete records
  - Shows missing names, dates, places
  - Click to navigate directly to any record

• Statistics Dashboard
  - Database overview (total people, living vs deceased)
  - Oldest living/deceased, average lifespan
  - Top 10 surnames
  - Media counts and data quality percentages

• Other Reports
  - Conflicts: Impossible dates and relationships
  - Duplicates: Find and merge potential duplicates
  - Timeline: Chronological view of all life events
  - Ancestor/Descendant: Full lineage with generation counts
  - Geographic: Birth/death locations with filtering

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
• Cmd/Ctrl+N - Add new person
• Cmd/Ctrl+E - Edit current person
• Cmd/Ctrl+D - Delete current person
• Cmd/Ctrl+O - Open database
• Cmd/Ctrl+Shift+R - Relationship Calculator
• Cmd/Ctrl+1/2/3 - Switch between views
• Cmd/Ctrl+F - Focus search/filter
• Cmd/Ctrl+Q - Quit application
• Double-click - Edit person
• Enter - Submit search/form

Full list: Settings → Keyboard Shortcuts (customizable)

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
	releaseNotesURL, _ := url.Parse("https://github.com/amarillier/KrankyBearGenealogy/blob/allanm/ReleaseNotes.txt")
	releaseNotesLink := widget.NewHyperlink("View Release Notes", releaseNotesURL)
	releaseNotesLink.Alignment = fyne.TextAlignCenter

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
		container.NewCenter(container.NewHBox(releaseNotesLink, githubLink, licenseLink)),
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
