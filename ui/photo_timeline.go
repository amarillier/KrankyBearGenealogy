package ui

import (
	"bytes"
	"fmt"
	"image"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"

	"genealogy/internal/dedupe"
	"genealogy/internal/exifdate"
	"genealogy/store"
)

// Track the photo timeline window so a second request brings it to front.
var photoTimelineWindow fyne.Window

const undatedLabel = "Undated"

// photoImageExts are the image types batch import and EXIF scanning handle.
var photoImageExts = map[string]bool{".jpg": true, ".jpeg": true, ".png": true, ".gif": true, ".webp": true}

// mediaYearGroup is the media taken in one year (or with no usable date).
type mediaYearGroup struct {
	Label string // "1995" or undatedLabel
	Media []store.Media
}

// groupMediaByYear splits media into years, oldest first, then an Undated group.
// Within a year, items are in date order, then by title.
func groupMediaByYear(media []store.Media) []mediaYearGroup {
	byYear := map[int][]store.Media{}
	var undated []store.Media
	for _, m := range media {
		if y := dedupe.ParseDate(m.DateTaken).Year; y != 0 {
			byYear[y] = append(byYear[y], m)
		} else {
			undated = append(undated, m)
		}
	}
	years := make([]int, 0, len(byYear))
	for y := range byYear {
		years = append(years, y)
	}
	sort.Ints(years)

	less := func(list []store.Media) func(i, j int) bool {
		return func(i, j int) bool {
			di, dj := dedupe.ParseDate(list[i].DateTaken), dedupe.ParseDate(list[j].DateTaken)
			if di != dj {
				return di.Year*10000+di.Month*100+di.Day < dj.Year*10000+dj.Month*100+dj.Day
			}
			return strings.ToLower(list[i].Title) < strings.ToLower(list[j].Title)
		}
	}
	var groups []mediaYearGroup
	for _, y := range years {
		sort.SliceStable(byYear[y], less(byYear[y]))
		groups = append(groups, mediaYearGroup{Label: strconv.Itoa(y), Media: byYear[y]})
	}
	if len(undated) > 0 {
		sort.SliceStable(undated, func(i, j int) bool {
			return strings.ToLower(undated[i].Title) < strings.ToLower(undated[j].Title)
		})
		groups = append(groups, mediaYearGroup{Label: undatedLabel, Media: undated})
	}
	return groups
}

// showPhotoTimeline shows photos and other media by the year they were taken.
func showPhotoTimeline(parent fyne.Window, s *store.Store) {
	if photoTimelineWindow != nil {
		photoTimelineWindow.RequestFocus()
		photoTimelineWindow.Show()
		return
	}
	win := fyne.CurrentApp().NewWindow("Photo Timeline")
	photoTimelineWindow = win

	// People who have media, for the filter
	personOptions := []string{"All people"}
	personIDs := []int64{0}
	if people, err := s.GetPeople(); err == nil {
		if ind, err := s.GetPersonIndicators(); err == nil {
			for _, p := range people {
				if ind[p.ID].HasMedia {
					personOptions = append(personOptions, formatPersonName(p))
					personIDs = append(personIDs, p.ID)
				}
			}
		}
	}

	var groups []mediaYearGroup
	var current []store.Media
	thumbCache := map[int64]image.Image{}

	header := widget.NewLabelWithStyle("", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	summary := widget.NewLabel("")

	grid := widget.NewGridWrap(
		func() int { return len(current) },
		func() fyne.CanvasObject {
			img := canvas.NewImageFromResource(nil)
			img.FillMode = canvas.ImageFillContain
			img.SetMinSize(fyne.NewSize(130, 130))
			label := widget.NewLabel("date")
			label.Alignment = fyne.TextAlignCenter
			label.Truncation = fyne.TextTruncateEllipsis
			return container.NewVBox(img, label)
		},
		func(id widget.GridWrapItemID, item fyne.CanvasObject) {
			if id >= len(current) {
				return
			}
			m := current[id]
			box := item.(*fyne.Container)
			img := box.Objects[0].(*canvas.Image)
			label := box.Objects[1].(*widget.Label)

			text := m.Title
			if m.DateTaken != "" {
				text = m.DateTaken + "  " + m.Title
			}
			label.SetText(text)

			decoded, ok := thumbCache[m.ID]
			if !ok {
				if data, err := s.GetMediaThumbnail(m.ID); err == nil && len(data) > 0 {
					if d, _, err := image.Decode(bytes.NewReader(data)); err == nil {
						decoded = d
					}
				}
				if len(thumbCache) > 400 { // keep memory bounded on huge libraries
					thumbCache = map[int64]image.Image{}
				}
				thumbCache[m.ID] = decoded
			}
			img.Image = decoded
			img.Refresh()
		})
	grid.OnSelected = func(id widget.GridWrapItemID) {
		defer grid.Unselect(id)
		if id < len(current) {
			showMediaLibraryViewer(win, s, current[id].ID, nil)
		}
	}

	var yearList *widget.List
	var reload func()
	showGroup := func(i int) {
		if i < 0 || i >= len(groups) {
			current = nil
			header.SetText("")
		} else {
			current = groups[i].Media
			header.SetText(fmt.Sprintf("%s - %d item(s)", groups[i].Label, len(current)))
		}
		grid.Refresh()
		grid.ScrollToTop()
	}

	yearList = widget.NewList(
		func() int { return len(groups) },
		func() fyne.CanvasObject { return widget.NewLabel("0000 (000)") },
		func(id widget.ListItemID, o fyne.CanvasObject) {
			if id < len(groups) {
				o.(*widget.Label).SetText(fmt.Sprintf("%s (%d)", groups[id].Label, len(groups[id].Media)))
			}
		})
	yearList.OnSelected = showGroup

	personSelect := widget.NewSelect(personOptions, nil)
	personSelect.SetSelectedIndex(0)
	typeSelect := widget.NewSelect([]string{"Photos only", "All media"}, nil)
	typeSelect.SetSelectedIndex(0)

	reload = func() {
		var media []store.Media
		var err error
		if idx := personSelect.SelectedIndex(); idx > 0 {
			media, err = s.GetMediaForPerson(personIDs[idx])
		} else {
			media, err = s.GetAllMedia()
		}
		if err != nil {
			dialog.ShowError(fmt.Errorf("Failed to load media: %w", err), win)
			return
		}
		if typeSelect.SelectedIndex() == 0 {
			photos := media[:0:0]
			for _, m := range media {
				if m.MediaType == "image" {
					photos = append(photos, m)
				}
			}
			media = photos
		}
		groups = groupMediaByYear(media)
		undated := 0
		for _, g := range groups {
			if g.Label == undatedLabel {
				undated = len(g.Media)
			}
		}
		summary.SetText(fmt.Sprintf("%d item(s), %d with no date taken.", len(media), undated))
		yearList.UnselectAll()
		yearList.Refresh()
		if len(groups) > 0 {
			yearList.Select(0)
		} else {
			showGroup(-1)
		}
	}
	personSelect.OnChanged = func(string) { reload() }
	typeSelect.OnChanged = func(string) { reload() }

	fillBtn := widget.NewButton("Fill Dates from Photos...", func() {
		confirmFillPhotoDates(win, s, reload)
	})
	importBtn := widget.NewButton("Batch Import...", func() {
		showBatchPhotoImport(win, s, reload)
	})

	top := container.NewVBox(
		container.NewHBox(widget.NewLabel("Person:"), personSelect, widget.NewLabel("Show:"), typeSelect, fillBtn, importBtn),
		summary,
		widget.NewSeparator(),
	)
	right := container.NewBorder(header, nil, nil, nil, grid)
	split := container.NewHSplit(yearList, right)
	split.SetOffset(0.18)
	win.SetContent(container.NewBorder(top, nil, nil, nil, split))
	win.Resize(fyne.NewSize(1000, 700))
	win.SetOnClosed(func() { photoTimelineWindow = nil })
	reload()
	win.Show()
}

// confirmFillPhotoDates offers to scan photos with no date taken for EXIF dates.
func confirmFillPhotoDates(w fyne.Window, s *store.Store, onDone func()) {
	dialog.ShowConfirm("Fill Dates from Photos",
		"This reads the date each undated photo was taken from the information stored inside the "+
			"photo file (EXIF) and fills in its Date Taken. Photos that already have a date are not changed.\n\nContinue?",
		func(ok bool) {
			if !ok {
				return
			}
			var scanned, updated int
			runWithProgress(w, "Fill Dates from Photos", "Reading photo dates...", func() error {
				var err error
				scanned, updated, err = fillMissingPhotoDates(s)
				return err
			}, func(err error) {
				if err != nil {
					dialog.ShowError(err, w)
					return
				}
				dialog.ShowInformation("Fill Dates from Photos",
					fmt.Sprintf("Checked %d undated photo(s) and found a date for %d.\n\nThe rest have no date stored in the file.", scanned, updated), w)
				if onDone != nil {
					onDone()
				}
			})
		}, w)
}

// fillMissingPhotoDates sets Date Taken from EXIF for every image that has none.
// It returns how many undated photos it checked and how many it updated.
func fillMissingPhotoDates(s *store.Store) (scanned, updated int, err error) {
	media, err := s.GetAllMedia()
	if err != nil {
		return 0, 0, err
	}
	for _, m := range media {
		if m.MediaType != "image" || strings.TrimSpace(m.DateTaken) != "" {
			continue
		}
		scanned++
		var date string
		var ok bool
		if m.IsExternal {
			date, ok = exifdate.FromFile(m.ExternalPath)
		} else {
			full, err := s.GetMediaByID(m.ID)
			if err != nil {
				continue
			}
			date, ok = exifdate.DateTaken(full.FullImage)
		}
		if !ok {
			continue
		}
		if err := s.SetMediaDateTaken(m.ID, date); err != nil {
			return scanned, updated, err
		}
		updated++
	}
	return scanned, updated, nil
}

// buildPeoplePicker returns a filterable list of people with checkboxes and a
// function that reports which people are ticked.
func buildPeoplePicker(s *store.Store) (fyne.CanvasObject, func() []int64, error) {
	people, err := s.GetPeople()
	if err != nil {
		return nil, nil, err
	}
	selected := map[int64]bool{}
	filtered := people

	filter := widget.NewEntry()
	filter.SetPlaceHolder("Filter by name...")
	list := widget.NewList(
		func() int { return len(filtered) },
		func() fyne.CanvasObject { return container.NewHBox(widget.NewCheck("", nil), widget.NewLabel("Name")) },
		func(id widget.ListItemID, o fyne.CanvasObject) {
			if id >= len(filtered) {
				return
			}
			p := filtered[id]
			box := o.(*fyne.Container)
			check := box.Objects[0].(*widget.Check)
			name := formatPersonName(p)
			if p.BirthDate != "" {
				name += fmt.Sprintf(" (b. %s)", p.BirthDate)
			}
			box.Objects[1].(*widget.Label).SetText(name)
			check.OnChanged = nil
			check.SetChecked(selected[p.ID])
			check.OnChanged = func(on bool) { selected[p.ID] = on }
		})
	filter.OnChanged = func(text string) {
		text = strings.ToLower(strings.TrimSpace(text))
		filtered = filtered[:0:0]
		for _, p := range people {
			if text == "" || strings.Contains(strings.ToLower(p.GivenName+" "+p.Surname), text) {
				filtered = append(filtered, p)
			}
		}
		list.Refresh()
	}
	scroll := container.NewScroll(list)
	scroll.SetMinSize(fyne.NewSize(420, 260))
	chosen := func() []int64 {
		var ids []int64
		for id, on := range selected {
			if on {
				ids = append(ids, id)
			}
		}
		return ids
	}
	return container.NewBorder(filter, nil, nil, nil, scroll), chosen, nil
}

// findPhotos returns the image files under dir (skipping hidden files and
// folders), up to limit, and whether the limit cut the list short.
func findPhotos(dir string, limit int) (files []string, truncated bool, err error) {
	err = filepath.WalkDir(dir, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil // unreadable entries are skipped, not fatal
		}
		if path != dir && strings.HasPrefix(d.Name(), ".") {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if d.IsDir() || !photoImageExts[strings.ToLower(filepath.Ext(d.Name()))] {
			return nil
		}
		if len(files) >= limit {
			truncated = true
			return filepath.SkipAll
		}
		files = append(files, path)
		return nil
	})
	sort.Strings(files)
	return files, truncated, err
}

const batchImportLimit = 5000

// showBatchPhotoImport imports every photo in a folder, reading each photo's
// date taken from its EXIF data, and links them all to the chosen people.
func showBatchPhotoImport(w fyne.Window, s *store.Store, onDone func()) {
	dialog.NewFolderOpen(func(dir fyne.ListableURI, err error) {
		if err != nil || dir == nil {
			return
		}
		var files []string
		var truncated bool
		runWithProgress(w, "Batch Import Photos", "Looking for photos...", func() error {
			var err error
			files, truncated, err = findPhotos(dir.Path(), batchImportLimit)
			return err
		}, func(err error) {
			if err != nil {
				dialog.ShowError(err, w)
				return
			}
			if len(files) == 0 {
				dialog.ShowInformation("Batch Import Photos", "No photos (JPEG, PNG, GIF or WebP) were found in that folder.", w)
				return
			}
			confirmBatchPhotoImport(w, s, dir.Path(), files, truncated, onDone)
		})
	}, w).Show()
}

func confirmBatchPhotoImport(w fyne.Window, s *store.Store, dir string, files []string, truncated bool, onDone func()) {
	picker, chosen, err := buildPeoplePicker(s)
	if err != nil {
		dialog.ShowError(err, w)
		return
	}
	storageOptions := []string{"Store in Database (recommended)", "Link to External Files"}
	storage := widget.NewRadioGroup(storageOptions, nil)
	storage.SetSelected(storageOptions[0])
	skipDupes := widget.NewCheck("Skip photos already in the library (same name and size)", nil)
	skipDupes.SetChecked(true)

	intro := fmt.Sprintf("Found %d photo(s) in %s.", len(files), dir)
	if truncated {
		intro += fmt.Sprintf("\nOnly the first %d are included; import the rest in another pass.", batchImportLimit)
	}
	introLabel := widget.NewLabel(intro)
	introLabel.Wrapping = fyne.TextWrapWord
	note := widget.NewLabel("Each photo's Date Taken is read from the photo itself when available. All photos will be linked to the people you select.")
	note.Wrapping = fyne.TextWrapWord

	form := container.NewVBox(introLabel, note, widget.NewSeparator(), storage, skipDupes,
		widget.NewSeparator(), widget.NewLabelWithStyle("Link all photos to:", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}), picker)

	dialog.ShowCustomConfirm("Batch Import Photos", "Import", "Cancel", form, func(ok bool) {
		if !ok {
			return
		}
		people := chosen()
		if len(people) == 0 {
			dialog.ShowInformation("No People Selected", "Please select at least one person to link the photos to.", w)
			return
		}
		runBatchPhotoImport(w, s, files, storage.Selected == storageOptions[1], skipDupes.Checked, people, onDone)
	}, w)
}

// batchImportResult summarises a batch import.
type batchImportResult struct {
	Imported, Skipped, Dated int
	Failed                   []string // "file: reason" for the first few failures
	FailedCount              int
}

func runBatchPhotoImport(w fyne.Window, s *store.Store, files []string, external, skipDupes bool, people []int64, onDone func()) {
	var res batchImportResult
	runWithProgressBar(w, "Importing Photos", len(files), func(report func(int, string)) error {
		res = importPhotos(s, files, external, skipDupes, people, report)
		return nil
	}, func(error) {
		msg := fmt.Sprintf("Imported %d photo(s); %d had a date taken read from the photo.", res.Imported, res.Dated)
		if res.Skipped > 0 {
			msg += fmt.Sprintf("\nSkipped %d already in the library.", res.Skipped)
		}
		if res.FailedCount > 0 {
			msg += fmt.Sprintf("\n\n%d photo(s) could not be imported:\n%s", res.FailedCount, strings.Join(res.Failed, "\n"))
			if res.FailedCount > len(res.Failed) {
				msg += "\n..."
			}
		}
		dialog.ShowInformation("Batch Import Complete", msg, w)
		if onDone != nil {
			onDone()
		}
	})
}

// importPhotos stores each file as media linked to people. A file that fails is
// recorded and skipped; it never stops the rest of the batch.
func importPhotos(s *store.Store, files []string, external, skipDupes bool, people []int64, report func(int, string)) batchImportResult {
	var res batchImportResult
	seenPath := map[string]bool{}
	seenNameSize := map[string]bool{}
	if skipDupes {
		if existing, err := s.GetAllMedia(); err == nil {
			for _, m := range existing {
				if m.IsExternal {
					seenPath[m.ExternalPath] = true
				}
				seenNameSize[nameSizeKey(m.Title, m.FileSize)] = true
			}
		}
	}
	fail := func(file string, err error) {
		res.FailedCount++
		if len(res.Failed) < 5 {
			res.Failed = append(res.Failed, fmt.Sprintf("%s: %v", filepath.Base(file), err))
		}
	}

	for i, file := range files {
		report(i, fmt.Sprintf("%d of %d: %s", i+1, len(files), filepath.Base(file)))
		info, err := os.Stat(file)
		if err != nil {
			fail(file, err)
			continue
		}
		title := strings.TrimSuffix(filepath.Base(file), filepath.Ext(file))
		if skipDupes && ((external && seenPath[file]) || seenNameSize[nameSizeKey(title, info.Size())]) {
			res.Skipped++
			continue
		}

		thumb, err := generateThumbnail(file)
		if err != nil {
			fail(file, err)
			continue
		}
		media := &store.Media{
			Title:     title,
			MediaType: "image",
			MimeType:  getMimeType(file),
			Thumbnail: thumb,
			FileSize:  info.Size(),
		}
		if date, ok := exifdate.FromFile(file); ok {
			media.DateTaken = date
		}
		if external {
			media.IsExternal, media.ExternalPath = true, file
		} else {
			data, err := readFullImage(file)
			if err != nil {
				fail(file, err)
				continue
			}
			media.FullImage, media.FileSize = data, int64(len(data))
		}
		if err := s.CreateMedia(media); err != nil {
			fail(file, err)
			continue
		}
		for _, pid := range people {
			if err := s.LinkMediaToPerson(media.ID, pid); err != nil {
				fail(file, fmt.Errorf("linked to person %d failed: %w", pid, err))
			}
		}
		seenPath[file] = external
		seenNameSize[nameSizeKey(title, media.FileSize)] = true
		res.Imported++
		if media.DateTaken != "" {
			res.Dated++
		}
	}
	report(len(files), "Done")
	return res
}

func nameSizeKey(title string, size int64) string {
	return strings.ToLower(title) + "|" + strconv.FormatInt(size, 10)
}
