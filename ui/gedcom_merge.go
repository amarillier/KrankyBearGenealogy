package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/widget"

	"genealogy/importer"
	"genealogy/internal/dedupe"
	"genealogy/internal/premerge"
	"genealogy/store"
)

// Track the merge window so a second request brings the existing one to front.
var gedcomMergeWindow fyne.Window

const (
	mergeActionAdd   = "Add as new person"
	mergeActionMerge = "Merge into existing person"
	mergeActionSkip  = "Skip"
)

// mergeRow is one incoming person with its candidate matches and the decision made so far.
type mergeRow struct {
	parsed     *importer.ParsedPerson
	candidates []dedupe.Candidate
	status     dedupe.Status
	reviewed   bool // the user has looked at and decided on this row

	action    importer.MergeAction
	candidate int // index into candidates when merging
	choices   map[string]importer.FieldSource
}

func (r *mergeRow) suggestedAction() (importer.MergeAction, int) {
	if r.status == dedupe.StatusExact {
		return importer.ActionMerge, 0
	}
	// Likely matches default to "add" so a wrong merge never happens silently;
	// a stray duplicate can be found later with the Duplicates report.
	return importer.ActionAdd, 0
}

func (r *mergeRow) reset() {
	r.action, r.candidate = r.suggestedAction()
	r.choices = map[string]importer.FieldSource{}
	r.reviewed = false
}

func personSummary(p store.Person) string {
	name := strings.TrimSpace(p.GivenName + " " + p.Surname)
	if name == "" {
		name = "(no name)"
	}
	var parts []string
	if p.BirthDate != "" {
		parts = append(parts, "b. "+p.BirthDate)
	}
	if p.DeathDate != "" {
		parts = append(parts, "d. "+p.DeathDate)
	}
	if len(parts) > 0 {
		name += " (" + strings.Join(parts, ", ") + ")"
	}
	return name
}

func (r *mergeRow) listLabel() string {
	label := personSummary(*r.parsed.Person)
	switch r.action {
	case importer.ActionAdd:
		label = "➕ " + label
	case importer.ActionSkip:
		label = "⏭ " + label
	default:
		label = "🔀 " + label + "  →  " + personSummary(r.candidates[r.candidate].Person)
	}
	if r.status == dedupe.StatusLikely && !r.reviewed {
		label += "  ⚠ review"
	}
	return label
}

// showGedcomMerge lets the user pick a GEDCOM file, then review and apply a merge into the current database.
func showGedcomMerge(w fyne.Window, s *store.Store, refreshAll func()) {
	if gedcomMergeWindow != nil {
		gedcomMergeWindow.RequestFocus()
		gedcomMergeWindow.Show()
		return
	}

	fd := dialog.NewFileOpen(func(r fyne.URIReadCloser, err error) {
		if err != nil || r == nil {
			return
		}
		path := r.URI().Path()
		r.Close()

		var parsed *importer.ParsedGedcom
		var rows []*mergeRow
		runWithProgress(w, "Merge GEDCOM", "Reading file and looking for matches...", func() error {
			var err error
			parsed, rows, err = buildMergeRows(path, s)
			return err
		}, func(err error) {
			if err != nil {
				dialog.ShowError(err, w)
				return
			}
			if len(rows) == 0 {
				dialog.ShowInformation("Merge GEDCOM", "That file contains no people.", w)
				return
			}
			openGedcomMergeWindow(w, s, filepath.Base(path), parsed, rows, refreshAll)
		})
	}, w)
	fd.SetFilter(storage.NewExtensionFileFilter([]string{".ged", ".GED"}))
	fd.Show()
}

func buildMergeRows(path string, s *store.Store) (*importer.ParsedGedcom, []*mergeRow, error) {
	parsed, err := importer.ParseGedcom(path)
	if err != nil {
		return nil, nil, err
	}
	existing, err := s.GetPeople()
	if err != nil {
		return nil, nil, err
	}
	idx := dedupe.NewIndex(existing)
	rows := make([]*mergeRow, 0, len(parsed.People))
	for _, pp := range parsed.People {
		row := &mergeRow{parsed: pp, candidates: idx.Match(*pp.Person, dedupe.DefaultThreshold)}
		if len(row.candidates) > 0 {
			row.status = row.candidates[0].Status
		}
		row.reset()
		rows = append(rows, row)
	}
	return parsed, rows, nil
}

func openGedcomMergeWindow(parent fyne.Window, s *store.Store, fileName string, parsed *importer.ParsedGedcom, rows []*mergeRow, refreshAll func()) {
	win := fyne.CurrentApp().NewWindow("Merge GEDCOM - " + fileName)
	gedcomMergeWindow = win

	summary := widget.NewLabel("")
	updateSummary := func() {
		var add, merge, skip, review int
		for _, r := range rows {
			switch r.action {
			case importer.ActionAdd:
				add++
			case importer.ActionMerge:
				merge++
			default:
				skip++
			}
			if r.status == dedupe.StatusLikely && !r.reviewed {
				review++
			}
		}
		summary.SetText(fmt.Sprintf("%d people in file: %d to add, %d to merge, %d to skip. %d likely matches still need review.",
			len(rows), add, merge, skip, review))
	}

	// Filtered view of the rows
	filter := widget.NewSelect([]string{"All", "Needs review", "Likely matches", "Exact matches", "New people"}, nil)
	var visible []*mergeRow
	var list *widget.List
	applyFilter := func() {
		visible = visible[:0]
		for _, r := range rows {
			ok := true
			switch filter.Selected {
			case "Needs review":
				ok = r.status == dedupe.StatusLikely && !r.reviewed
			case "Likely matches":
				ok = r.status == dedupe.StatusLikely
			case "Exact matches":
				ok = r.status == dedupe.StatusExact
			case "New people":
				ok = r.status == dedupe.StatusNew
			}
			if ok {
				visible = append(visible, r)
			}
		}
		if list != nil {
			list.Refresh()
		}
	}

	detail := container.NewVBox(widget.NewLabel("Select a person on the left to review the match."))
	detailScroll := container.NewVScroll(detail)
	var selected *mergeRow

	var showDetail func(r *mergeRow)
	refreshRow := func() {
		updateSummary()
		list.Refresh()
	}

	showDetail = func(r *mergeRow) {
		selected = r
		detail.Objects = nil

		detail.Add(widget.NewLabelWithStyle("Incoming: "+personSummary(*r.parsed.Person), fyne.TextAlignLeading, fyne.TextStyle{Bold: true}))
		if r.status == dedupe.StatusNew {
			detail.Add(widget.NewLabel("No matching person found in your database."))
		} else {
			detail.Add(widget.NewLabel(fmt.Sprintf("%s - best score %.0f%%", r.status, r.candidates[0].Score.Overall)))
		}

		actions := []string{mergeActionAdd, mergeActionSkip}
		if len(r.candidates) > 0 {
			actions = []string{mergeActionAdd, mergeActionMerge, mergeActionSkip}
		}
		actionRadio := widget.NewRadioGroup(actions, nil)
		diffBox := container.NewVBox()

		var rebuildDiff func()
		rebuildDiff = func() {
			diffBox.Objects = nil
			if r.action == importer.ActionMerge {
				diffBox.Add(buildMergeDiff(r, refreshRow))
			}
			diffBox.Refresh()
		}

		setAction := func(label string) {
			switch label {
			case mergeActionMerge:
				r.action = importer.ActionMerge
			case mergeActionSkip:
				r.action = importer.ActionSkip
			default:
				r.action = importer.ActionAdd
			}
		}
		current := mergeActionAdd
		switch r.action {
		case importer.ActionMerge:
			current = mergeActionMerge
		case importer.ActionSkip:
			current = mergeActionSkip
		}
		actionRadio.SetSelected(current)
		actionRadio.OnChanged = func(label string) {
			setAction(label)
			r.reviewed = true
			rebuildDiff()
			refreshRow()
		}
		detail.Add(actionRadio)

		if len(r.candidates) > 1 {
			options := make([]string, len(r.candidates))
			for i, c := range r.candidates {
				options[i] = fmt.Sprintf("%s - %.0f%%", personSummary(c.Person), c.Score.Overall)
			}
			pick := widget.NewSelect(options, nil)
			pick.SetSelectedIndex(r.candidate)
			pick.OnChanged = func(string) {
				r.candidate = pick.SelectedIndex()
				r.choices = map[string]importer.FieldSource{}
				r.reviewed = true
				rebuildDiff()
				refreshRow()
			}
			detail.Add(widget.NewLabel("Existing person to merge into:"))
			detail.Add(pick)
		} else if len(r.candidates) == 1 {
			detail.Add(widget.NewLabel("Existing person: " + personSummary(r.candidates[0].Person)))
		}

		detail.Add(widget.NewSeparator())
		detail.Add(diffBox)
		rebuildDiff()
		detail.Refresh()
	}

	list = widget.NewList(
		func() int { return len(visible) },
		func() fyne.CanvasObject { return widget.NewLabel("template person name and dates here") },
		func(i widget.ListItemID, o fyne.CanvasObject) {
			if i < len(visible) {
				o.(*widget.Label).SetText(visible[i].listLabel())
			}
		})
	list.OnSelected = func(i widget.ListItemID) {
		if i < len(visible) {
			showDetail(visible[i])
		}
	}
	filter.OnChanged = func(string) { applyFilter() }
	filter.SetSelected("All")

	// Bulk actions
	resetBtn := widget.NewButton("Reset to suggested", func() {
		for _, r := range rows {
			r.reset()
		}
		applyFilter()
		updateSummary()
		if selected != nil {
			showDetail(selected)
		}
	})
	mergeLikelyBtn := widget.NewButton("Merge all likely into best match", func() {
		for _, r := range rows {
			if r.status == dedupe.StatusLikely {
				r.action, r.candidate, r.reviewed = importer.ActionMerge, 0, true
			}
		}
		applyFilter()
		updateSummary()
		if selected != nil {
			showDetail(selected)
		}
	})
	skipLikelyBtn := widget.NewButton("Skip all likely matches", func() {
		for _, r := range rows {
			if r.status == dedupe.StatusLikely {
				r.action, r.reviewed = importer.ActionSkip, true
			}
		}
		applyFilter()
		updateSummary()
		if selected != nil {
			showDetail(selected)
		}
	})

	applyBtn := widget.NewButton("Apply Merge...", func() {
		plan := importer.MergePlan{}
		for _, r := range rows {
			pl := importer.PersonPlan{Action: r.action, Choices: r.choices}
			if r.action == importer.ActionMerge {
				pl.ExistingID = r.candidates[r.candidate].Person.ID
			}
			plan[r.parsed.GedID] = pl
		}
		confirmAndApplyMerge(win, s, parsed, plan, summary.Text, func() {
			refreshAll()
			win.Close()
		})
	})
	applyBtn.Importance = widget.HighImportance
	cancelBtn := widget.NewButton("Cancel", func() { win.Close() })

	top := container.NewVBox(
		summary,
		container.NewHBox(widget.NewLabel("Show:"), filter, resetBtn, mergeLikelyBtn, skipLikelyBtn),
		widget.NewSeparator(),
	)
	bottom := container.NewHBox(widget.NewLabel("A backup is taken automatically before merging."), applyBtn, cancelBtn)
	split := container.NewHSplit(list, detailScroll)
	split.SetOffset(0.45)
	win.SetContent(container.NewBorder(top, bottom, nil, nil, split))
	win.Resize(fyne.NewSize(1100, 700))
	win.SetOnClosed(func() { gedcomMergeWindow = nil })
	updateSummary()
	win.Show()
}

// buildMergeDiff shows the fields that differ between the incoming person and
// the chosen existing person, with a choice for each real conflict.
func buildMergeDiff(r *mergeRow, onChange func()) fyne.CanvasObject {
	existing := r.candidates[r.candidate].Person
	diffs := importer.DiffPersons(existing, *r.parsed.Person)
	box := container.NewVBox()
	if len(diffs) == 0 {
		box.Add(widget.NewLabel("The two records are identical in every field a merge can change."))
		return box
	}
	box.Add(widget.NewLabelWithStyle("Differences", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}))
	for _, d := range diffs {
		d := d
		if !d.Conflict {
			if d.Existing == "" {
				box.Add(widget.NewLabel(fmt.Sprintf("%s: will be filled in with %q", d.Label, d.Incoming)))
			} else {
				box.Add(widget.NewLabel(fmt.Sprintf("%s: %q kept (the file has no value)", d.Label, d.Existing)))
			}
			continue
		}
		const keepOld, useNew, keepBoth = "Keep existing: ", "Use incoming: ", "Keep both"
		options := []string{keepOld + abbreviate(d.Existing), useNew + abbreviate(d.Incoming)}
		if d.Key == importer.FieldNotes {
			options = append(options, keepBoth)
		}
		radio := widget.NewRadioGroup(options, nil)
		switch r.choices[d.Key] {
		case importer.UseIncoming:
			radio.Selected = options[1]
		case importer.UseBoth:
			radio.Selected = keepBoth
		default:
			radio.Selected = options[0]
		}
		radio.OnChanged = func(sel string) {
			switch {
			case strings.HasPrefix(sel, useNew):
				r.choices[d.Key] = importer.UseIncoming
			case sel == keepBoth:
				r.choices[d.Key] = importer.UseBoth
			default:
				r.choices[d.Key] = importer.UseExisting
			}
			r.reviewed = true
			onChange()
		}
		box.Add(widget.NewLabelWithStyle(d.Label+" differs", fyne.TextAlignLeading, fyne.TextStyle{Italic: true}))
		box.Add(radio)
	}
	return box
}

func abbreviate(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	if len(s) > 60 {
		return s[:57] + "..."
	}
	return s
}

func confirmAndApplyMerge(win fyne.Window, s *store.Store, parsed *importer.ParsedGedcom, plan importer.MergePlan, summary string, onSuccess func()) {
	msg := widget.NewLabel(summary + "\n\nA backup of your database is saved first and removed automatically after " +
		fmt.Sprintf("%d days.\nThe merge cannot be undone with Undo; restore the backup if you need to go back.", int(premerge.MaxAge.Hours()/24)))
	msg.Wrapping = fyne.TextWrapWord

	dialog.ShowCustomConfirm("Apply Merge", "Merge", "Cancel", msg, func(ok bool) {
		if !ok {
			return
		}
		var backupPath string
		var result importer.MergeResult
		runWithProgress(win, "Merging", "Backing up and merging...", func() error {
			var err error
			if backupPath, err = createPreMergeBackup(s); err != nil {
				return fmt.Errorf("merge cancelled, backup failed: %w", err)
			}
			result, err = importer.ApplyMerge(s, parsed, plan)
			return err
		}, func(err error) {
			if err != nil {
				dialog.ShowError(err, win)
				return
			}
			dialog.ShowInformation("Merge Complete",
				fmt.Sprintf("%s\n\nBackup saved to:\n%s", result, backupPath), win)
			onSuccess()
		})
	}, win)
}

// databaseFilePath returns the file backing s's main database.
func databaseFilePath(s *store.Store) (string, error) {
	rows, err := s.DB.Query(`PRAGMA database_list`)
	if err != nil {
		return "", err
	}
	defer rows.Close()
	for rows.Next() {
		var seq int
		var name, file string
		if err := rows.Scan(&seq, &name, &file); err != nil {
			return "", err
		}
		if name == "main" && file != "" {
			return file, nil
		}
	}
	return "", fmt.Errorf("database file path not found")
}

// createPreMergeBackup zips the database into the merge-backups folder next to it.
func createPreMergeBackup(s *store.Store) (string, error) {
	dbPath, err := databaseFilePath(s)
	if err != nil {
		return "", err
	}
	// Fold the write-ahead log into the main file so the copy is complete
	if _, err := s.DB.Exec(`PRAGMA wal_checkpoint(TRUNCATE)`); err != nil {
		return "", fmt.Errorf("checkpoint database: %w", err)
	}
	dir := premerge.Dir(dbPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	out := filepath.Join(dir, premerge.FileName(dbPath, time.Now()))
	if err := backupDatabase(dbPath, out); err != nil {
		return "", err
	}
	return out, nil
}

// cleanupPreMergeBackups deletes expired pre-merge backups; run at startup.
func cleanupPreMergeBackups(dbPath string) {
	premerge.Cleanup(premerge.Dir(dbPath), premerge.MaxAge, time.Now())
}
