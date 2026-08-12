package ui

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/widget"
	"github.com/disintegration/imaging"

	"genealogy/store"
)

const thumbnailSize = 150

// createPlaceholderThumbnail creates a visual placeholder for non-image media
func createPlaceholderThumbnail(mediaType string) []byte {
	width := thumbnailSize
	height := thumbnailSize
	
	// Create background based on media type
	var bgColor color.RGBA
	
	switch mediaType {
	case "video":
		bgColor = color.RGBA{100, 100, 180, 255} // Blue-ish for video
	case "document":
		bgColor = color.RGBA{180, 100, 100, 255} // Red-ish for documents/PDFs
	default:
		bgColor = color.RGBA{150, 150, 150, 255} // Gray for unknown
	}
	
	// Create base image
	img := imaging.New(width, height, bgColor)
	
	// Create a lighter "paper" rectangle in the center for document icon look
	if mediaType == "document" {
		// Draw a document-like shape
		for y := 20; y < height-20; y++ {
			for x := 30; x < width-30; x++ {
				if y < 35 && x > width-50 {
					// Skip top-right corner for folded effect
					continue
				}
				img.Set(x, y, color.RGBA{240, 240, 240, 255})
			}
		}
		// Add fold triangle
		for y := 20; y < 35; y++ {
			for x := width - 50; x < width-30; x++ {
				if x > width-50+(y-20) {
					img.Set(x, y, color.RGBA{220, 220, 220, 255})
				}
			}
		}
	} else if mediaType == "video" {
		// Draw a play button triangle
		centerX := width / 2
		centerY := height / 2
		triangleSize := 30
		
		for y := centerY - triangleSize/2; y <= centerY + triangleSize/2; y++ {
			for x := centerX - triangleSize/2; x <= centerX + triangleSize/2; x++ {
				// Simple triangle shape (pointing right)
				dy := y - centerY
				dx := x - centerX
				if dx >= -triangleSize/4 && dx <= triangleSize/2 {
					if dy >= -dx && dy <= dx {
						img.Set(x, y, color.RGBA{255, 255, 255, 255})
					}
				}
			}
		}
	}
	
	var buf bytes.Buffer
	jpeg.Encode(&buf, img, &jpeg.Options{Quality: 85})
	return buf.Bytes()
}

// openInDefaultViewer opens a file in the OS default application
func openInDefaultViewer(filePath string) error {
	var cmd *exec.Cmd
	
	switch runtime.GOOS {
	case "darwin": // macOS
		cmd = exec.Command("open", filePath)
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", "", filePath)
	case "linux":
		cmd = exec.Command("xdg-open", filePath)
	default:
		return fmt.Errorf("unsupported operating system: %s", runtime.GOOS)
	}
	
	return cmd.Start()
}

// openMediaInViewer opens media in appropriate viewer (default OS viewer for non-images)
func openMediaInViewer(w fyne.Window, media *store.Media) error {
	if media.MediaType != "image" {
		// For PDFs and videos, use OS default viewer
		var filePath string
		
		if media.IsExternal {
			// Use external path directly
			filePath = media.ExternalPath
			if _, err := os.Stat(filePath); err != nil {
				return fmt.Errorf("external file not found: %s", filePath)
			}
		} else {
			// Create temporary file from database blob
			ext := ""
			switch media.MimeType {
			case "application/pdf":
				ext = ".pdf"
			case "video/mp4":
				ext = ".mp4"
			case "video/quicktime":
				ext = ".mov"
			case "application/msword":
				ext = ".doc"
			case "application/vnd.openxmlformats-officedocument.wordprocessingml.document":
				ext = ".docx"
			case "application/vnd.ms-excel":
				ext = ".xls"
			case "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet":
				ext = ".xlsx"
			case "application/vnd.ms-powerpoint":
				ext = ".ppt"
			case "application/vnd.openxmlformats-officedocument.presentationml.presentation":
				ext = ".pptx"
			default:
				ext = filepath.Ext(media.Title)
			}
			
			tmpFile, err := ioutil.TempFile("", "genealogy-media-*"+ext)
			if err != nil {
				return fmt.Errorf("failed to create temporary file: %w", err)
			}
			defer tmpFile.Close()
			
			if _, err := tmpFile.Write(media.FullImage); err != nil {
				return fmt.Errorf("failed to write temporary file: %w", err)
			}
			
			filePath = tmpFile.Name()
		}
		
		// Open in default viewer
		return openInDefaultViewer(filePath)
	}
	
	return nil
}

// showMediaLibrary displays a browsable library of all media in the database
func showMediaLibrary(w fyne.Window, s *store.Store) {
	// Load all media
	allMedia, err := s.GetAllMedia()
	if err != nil {
		dialog.ShowError(fmt.Errorf("Failed to load media: %w", err), w)
		return
	}
	
	if len(allMedia) == 0 {
		dialog.ShowInformation("Media Library", 
			"No media in the database yet.\n\nClick 'Add Media' to get started!", w)
		return
	}
	
	// Filter support
	filterEntry := widget.NewEntry()
	filterEntry.SetPlaceHolder("Filter by title or description...")
	
	filteredMedia := allMedia
	
	var mediaGrid *widget.List
	
	mediaGrid = widget.NewList(
		func() int {
			return len(filteredMedia)
		},
		func() fyne.CanvasObject {
			thumbnail := canvas.NewImageFromResource(nil)
			thumbnail.FillMode = canvas.ImageFillContain
			thumbnail.SetMinSize(fyne.NewSize(120, 120))
			
			title := widget.NewLabel("Title")
			title.TextStyle = fyne.TextStyle{Bold: true}
			title.Wrapping = fyne.TextWrapWord
			
			info := widget.NewLabel("Info")
			info.Wrapping = fyne.TextWrapWord
			
			linkedInfo := widget.NewLabel("Linked to")
			linkedInfo.TextStyle = fyne.TextStyle{Italic: true}
			
			viewBtn := widget.NewButton("View", nil)
			editBtn := widget.NewButton("Edit", nil)
			manageBtn := widget.NewButton("Manage Links", nil)
			deleteBtn := widget.NewButton("Delete", nil)
			
			return container.NewHBox(
				thumbnail,
				container.NewVBox(title, info, linkedInfo),
				container.NewVBox(viewBtn, editBtn, manageBtn, deleteBtn),
			)
		},
		func(id widget.ListItemID, item fyne.CanvasObject) {
			if id >= len(filteredMedia) {
				return
			}
			m := filteredMedia[id]
			
			hbox := item.(*fyne.Container)
			if len(hbox.Objects) < 3 {
				return
			}
			
			thumb := hbox.Objects[0].(*canvas.Image)
			center := hbox.Objects[1].(*fyne.Container)
			right := hbox.Objects[2].(*fyne.Container)
			
			// Load thumbnail
			fullMedia, err := s.GetMediaByID(m.ID)
			if err == nil && len(fullMedia.Thumbnail) > 0 {
				img, _, err := image.Decode(bytes.NewReader(fullMedia.Thumbnail))
				if err == nil {
					thumb.Image = img
					thumb.Refresh()
				}
			}
			
			// Get linked people
			linkedPeople, _ := s.GetLinkedPeopleForMedia(m.ID)
			
			// Set labels
			if len(center.Objects) < 3 {
				return
			}
			title := center.Objects[0].(*widget.Label)
			info := center.Objects[1].(*widget.Label)
			linkedInfo := center.Objects[2].(*widget.Label)
			
			// Check for broken external links
			brokenLink := false
			if m.IsExternal {
				if _, err := os.Stat(m.ExternalPath); err != nil {
					brokenLink = true
				}
			}
			
			// Set title with visual indicators
			if brokenLink {
				title.SetText("⚠️ " + m.Title + " (BROKEN LINK)")
			} else if m.IsExternal {
				title.SetText("🔗 " + m.Title)
			} else {
				title.SetText(m.Title)
			}
			
			sizeStr := fmt.Sprintf("%.1f KB", float64(m.FileSize)/1024.0)
			var infoText string
			if brokenLink {
				infoText = fmt.Sprintf("⚠️ External (FILE NOT FOUND) - %s - %s", m.MediaType, sizeStr)
			} else if m.IsExternal {
				infoText = fmt.Sprintf("🔗 External - %s - %s", m.MediaType, sizeStr)
			} else {
				infoText = fmt.Sprintf("Database - %s - %s", m.MediaType, sizeStr)
			}
			if m.DateTaken != "" {
				infoText += fmt.Sprintf("\nDate: %s", m.DateTaken)
			}
			info.SetText(infoText)
			
			// Show linked people
			if len(linkedPeople) > 0 {
				names := []string{}
				for _, personID := range linkedPeople {
					person, err := s.GetPersonByID(personID)
					if err == nil {
						names = append(names, fmt.Sprintf("%s %s", person.GivenName, person.Surname))
					}
				}
				linkedInfo.SetText(fmt.Sprintf("Linked to: %s", strings.Join(names, ", ")))
			} else {
				linkedInfo.SetText("Not linked to anyone")
			}
			
			// Set button actions
			if len(right.Objects) < 4 {
				return
			}
			viewBtn := right.Objects[0].(*widget.Button)
			editBtn := right.Objects[1].(*widget.Button)
			manageBtn := right.Objects[2].(*widget.Button)
			deleteBtn := right.Objects[3].(*widget.Button)
			
			viewBtn.OnTapped = func() {
				showMediaLibraryViewer(w, s, m.ID, func() {
					// Refresh list after any changes
					allMedia, _ = s.GetAllMedia()
					filteredMedia = allMedia
					mediaGrid.Refresh()
				})
			}
			
			editBtn.OnTapped = func() {
				showEditMediaDialog(w, s, m.ID, func() {
					// Refresh list after edit
					allMedia, _ = s.GetAllMedia()
					filteredMedia = allMedia
					mediaGrid.Refresh()
				})
			}
			
			manageBtn.OnTapped = func() {
				showManageLinksDialog(w, s, m.ID, m.Title, func() {
					// Refresh list after link changes
					allMedia, _ = s.GetAllMedia()
					filteredMedia = allMedia
					mediaGrid.Refresh()
				})
			}
			
			deleteBtn.OnTapped = func() {
				linkCount := len(linkedPeople)
				dialog.ShowConfirm("Delete Media",
					fmt.Sprintf("Permanently delete this media from the database?\n\n%s\n\nThis will remove %d %s.", 
						m.Title, linkCount, map[bool]string{true: "link", false: "links"}[linkCount == 1]),
					func(confirmed bool) {
						if !confirmed {
							return
						}
						if err := s.DeleteMedia(m.ID); err != nil {
							dialog.ShowError(err, w)
							return
						}
						// Refresh list
						allMedia, _ = s.GetAllMedia()
						filteredMedia = allMedia
						mediaGrid.Refresh()
					}, w)
			}
		},
	)
	
	filterEntry.OnChanged = func(filterText string) {
		filterText = strings.ToLower(strings.TrimSpace(filterText))
		if filterText == "" {
			filteredMedia = allMedia
		} else {
			filteredMedia = []store.Media{}
			for _, m := range allMedia {
				if strings.Contains(strings.ToLower(m.Title), filterText) ||
					strings.Contains(strings.ToLower(m.Description), filterText) {
					filteredMedia = append(filteredMedia, m)
				}
			}
		}
		mediaGrid.Refresh()
	}
	
	// Refresh function to reload media list
	refreshLibrary := func() {
		allMedia, _ = s.GetAllMedia()
		filteredMedia = allMedia
		mediaGrid.Refresh()
	}
	
	// Add Media button in library
	addMediaBtn := widget.NewButton("📷 Add Media", func() {
		// Save current dialog reference
		showGlobalMediaDialogWithCallback(w, s, func() {
			// Refresh the library after adding media
			refreshLibrary()
		})
	})
	
	content := container.NewBorder(
		container.NewVBox(
			widget.NewLabelWithStyle(fmt.Sprintf("Media Library (%d items)", len(allMedia)), 
				fyne.TextAlignCenter, fyne.TextStyle{Bold: true}),
			widget.NewSeparator(),
			addMediaBtn,
			filterEntry,
		),
		nil, nil, nil,
		mediaGrid,
	)
	
	libraryDialog := dialog.NewCustom("Media Library", "Close", content, w)
	libraryDialog.Resize(fyne.NewSize(900, 600))
	libraryDialog.Show()
}

// showGlobalMediaDialogWithCallback shows a dialog to add media with success callback
func showGlobalMediaDialogWithCallback(w fyne.Window, s *store.Store, onSuccess func()) {
	// Step 1: Select file
	fd := dialog.NewFileOpen(func(reader fyne.URIReadCloser, err error) {
		if err != nil || reader == nil {
			return
		}
		defer reader.Close()
		
		imagePath := reader.URI().Path()
		filename := filepath.Base(imagePath)
		
		// Determine media type
		ext := strings.ToLower(filepath.Ext(filename))
		isImage := ext == ".jpg" || ext == ".jpeg" || ext == ".png" || ext == ".gif" || ext == ".webp"
		
		// Show progress
		progress := dialog.NewInformation("Processing", "Loading media...", w)
		progress.Show()
		
		var thumbnail []byte
		var fullImage []byte
		
		if isImage {
			// Generate thumbnail for images
			thumbnail, err = generateThumbnail(imagePath)
			if err != nil {
				progress.Hide()
				dialog.ShowError(fmt.Errorf("Failed to generate thumbnail: %w", err), w)
				return
			}
			
			// Read full image (we'll store it or not based on user choice)
			fullImage, err = readFullImage(imagePath)
			if err != nil {
				progress.Hide()
				dialog.ShowError(fmt.Errorf("Failed to read image: %w", err), w)
				return
			}
		} else {
			// For non-images (PDFs, videos), just read the file
			fullImage, err = readFullImage(imagePath)
			if err != nil {
				progress.Hide()
				dialog.ShowError(fmt.Errorf("Failed to read file: %w", err), w)
				return
			}
			// Generate placeholder thumbnail for non-images
			ext := strings.ToLower(filepath.Ext(imagePath))
			mediaType := "document"
			if ext == ".mp4" || ext == ".mov" {
				mediaType = "video"
			}
			thumbnail = createPlaceholderThumbnail(mediaType)
		}
		
		progress.Hide()
		
		// Step 2: Show metadata and person selection dialog
		showGlobalMediaMetadataDialog(w, s, filename, imagePath, thumbnail, fullImage, onSuccess)
	}, w)
	
	fd.SetFilter(storage.NewExtensionFileFilter([]string{
		".jpg", ".jpeg", ".png", ".gif", ".webp", 
		".mp4", ".mov", 
		".pdf", ".doc", ".docx", ".xls", ".xlsx", ".ppt", ".pptx"}))
	fd.Show()
}

// showGlobalMediaMetadataDialog shows dialog to enter media metadata and select people
func showGlobalMediaMetadataDialog(w fyne.Window, s *store.Store, filename, fullPath string, thumbnail, fullImage []byte, onSuccess func()) {
	titleEntry := widget.NewEntry()
	titleEntry.SetPlaceHolder("Title/Caption")
	titleEntry.SetText(strings.TrimSuffix(filename, filepath.Ext(filename)))
	
	descEntry := widget.NewMultiLineEntry()
	descEntry.SetPlaceHolder("Description (optional)")
	descEntry.SetMinRowsVisible(3)
	
	dateEntry := widget.NewEntry()
	dateEntry.SetPlaceHolder("YYYY-MM-DD (optional)")
	
	// Storage option
	storageOptions := []string{"Store in Database (recommended)", "Link to External File"}
	storageRadio := widget.NewRadioGroup(storageOptions, nil)
	storageRadio.SetSelected(storageOptions[0])
	storageRadio.Horizontal = false
	
	// Get all people for selection
	people, err := s.GetPeople()
	if err != nil {
		dialog.ShowError(err, w)
		return
	}
	
	// Create a searchable/filterable list of people with checkboxes
	filterEntry := widget.NewEntry()
	filterEntry.SetPlaceHolder("Filter by name...")
	
	selectedPeople := make(map[int64]bool)
	
	var peopleList *widget.List
	filteredPeople := people
	
	peopleList = widget.NewList(
		func() int {
			return len(filteredPeople)
		},
		func() fyne.CanvasObject {
			check := widget.NewCheck("", nil)
			label := widget.NewLabel("Name")
			return container.NewHBox(check, label)
		},
		func(id widget.ListItemID, item fyne.CanvasObject) {
			if id >= len(filteredPeople) {
				return
			}
			p := filteredPeople[id]
			
			hbox := item.(*fyne.Container)
			check := hbox.Objects[0].(*widget.Check)
			label := hbox.Objects[1].(*widget.Label)
			
			personName := fmt.Sprintf("%s %s", p.GivenName, p.Surname)
			if p.BirthDate != "" {
				personName += fmt.Sprintf(" (b. %s)", p.BirthDate)
			}
			label.SetText(personName)
			
			// Set checkbox state
			check.Checked = selectedPeople[p.ID]
			check.OnChanged = func(checked bool) {
				selectedPeople[p.ID] = checked
			}
		},
	)
	
	filterEntry.OnChanged = func(filterText string) {
		filterText = strings.ToLower(strings.TrimSpace(filterText))
		if filterText == "" {
			filteredPeople = people
		} else {
			filteredPeople = []store.Person{}
			for _, p := range people {
				fullName := strings.ToLower(fmt.Sprintf("%s %s", p.GivenName, p.Surname))
				if strings.Contains(fullName, filterText) {
					filteredPeople = append(filteredPeople, p)
				}
			}
		}
		peopleList.Refresh()
	}
	
	peopleScroll := container.NewScroll(peopleList)
	peopleScroll.SetMinSize(fyne.NewSize(400, 300))
	
	selectedLabel := widget.NewLabel("Select people to link this media to:")
	selectedLabel.TextStyle = fyne.TextStyle{Bold: true}
	
	storageInfo := widget.NewLabel("External files remain in their original location. Database storage ensures portability but increases file size.")
	storageInfo.Wrapping = fyne.TextWrapWord
	storageInfo.TextStyle = fyne.TextStyle{Italic: true}
	
	form := container.NewVBox(
		widget.NewLabel("Title:"),
		titleEntry,
		widget.NewLabel("Description:"),
		descEntry,
		widget.NewLabel("Date Taken:"),
		dateEntry,
		widget.NewSeparator(),
		widget.NewLabel("Storage:"),
		storageRadio,
		storageInfo,
		widget.NewSeparator(),
		selectedLabel,
		filterEntry,
		peopleScroll,
	)
	
	dialog.ShowCustomConfirm("Add Media and Link to People", "Save", "Cancel", form, func(save bool) {
		if !save {
			return
		}
		
		// Count selected people
		selectedCount := 0
		for _, selected := range selectedPeople {
			if selected {
				selectedCount++
			}
		}
		
		if selectedCount == 0 {
			dialog.ShowInformation("No People Selected", 
				"Please select at least one person to link this media to.", w)
			return
		}
		
		// Determine storage type
		isExternal := storageRadio.Selected == storageOptions[1]
		
		// Determine media type from file extension
		ext := strings.ToLower(filepath.Ext(filename))
		mediaType := "image" // default
		if ext == ".mp4" || ext == ".mov" {
			mediaType = "video"
		} else if ext == ".pdf" || ext == ".doc" || ext == ".docx" || ext == ".xls" || ext == ".xlsx" || ext == ".ppt" || ext == ".pptx" {
			mediaType = "document"
		}
		
		// Create media record
		media := &store.Media{
			Title:       titleEntry.Text,
			Description: descEntry.Text,
			MediaType:   mediaType,
			MimeType:    getMimeType(filename),
			Thumbnail:   thumbnail,
			DateTaken:   dateEntry.Text,
		}
		
		if isExternal {
			// Store as external reference
			media.IsExternal = true
			media.ExternalPath = fullPath
			media.FullImage = nil // Don't store the blob
			// Get file size from disk
			if fileInfo, err := os.Stat(fullPath); err == nil {
				media.FileSize = fileInfo.Size()
			}
		} else {
			// Store in database
			media.IsExternal = false
			media.ExternalPath = ""
			media.FullImage = fullImage
			media.FileSize = int64(len(fullImage))
		}
		
		if err := s.CreateMedia(media); err != nil {
			dialog.ShowError(fmt.Errorf("Failed to save media: %w", err), w)
			return
		}
		
		// Link media to all selected people
		linkedCount := 0
		for personID, selected := range selectedPeople {
			if selected {
				if err := s.LinkMediaToPerson(media.ID, personID); err != nil {
					// Log error but continue
					fmt.Printf("Failed to link media to person %d: %v\n", personID, err)
				} else {
					linkedCount++
				}
			}
		}
		
		dialog.ShowInformation("Success", 
			fmt.Sprintf("Media added successfully and linked to %d %s", 
				linkedCount,
				map[bool]string{true: "person", false: "people"}[linkedCount == 1]), w)
		
		// Call success callback
		if onSuccess != nil {
			onSuccess()
		}
	}, w)
}

// showGlobalMediaDialog shows a dialog to add media and link it to selected people
func showGlobalMediaDialog(w fyne.Window, s *store.Store) {
	showGlobalMediaDialogWithCallback(w, s, nil)
}

// generateThumbnail creates a thumbnail from an image file
func generateThumbnail(imagePath string) ([]byte, error) {
	// Open and decode the image
	img, err := imaging.Open(imagePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open image: %w", err)
	}

	// Resize to thumbnail size (maintaining aspect ratio)
	thumbnail := imaging.Fit(img, thumbnailSize, thumbnailSize, imaging.Lanczos)

	// Encode as JPEG
	var buf bytes.Buffer
	err = jpeg.Encode(&buf, thumbnail, &jpeg.Options{Quality: 85})
	if err != nil {
		return nil, fmt.Errorf("failed to encode thumbnail: %w", err)
	}

	return buf.Bytes(), nil
}

// generateThumbnailFromBytes creates a thumbnail from image bytes
func generateThumbnailFromBytes(imageData []byte) ([]byte, error) {
	// Decode the image
	img, _, err := image.Decode(bytes.NewReader(imageData))
	if err != nil {
		return nil, fmt.Errorf("failed to decode image: %w", err)
	}

	// Resize to thumbnail size (maintaining aspect ratio)
	thumbnail := imaging.Fit(img, thumbnailSize, thumbnailSize, imaging.Lanczos)

	// Encode as JPEG
	var buf bytes.Buffer
	err = jpeg.Encode(&buf, thumbnail, &jpeg.Options{Quality: 85})
	if err != nil {
		return nil, fmt.Errorf("failed to encode thumbnail: %w", err)
	}

	return buf.Bytes(), nil
}

// readFullImage reads an image file into bytes
func readFullImage(imagePath string) ([]byte, error) {
	file, err := os.Open(imagePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open image: %w", err)
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		return nil, fmt.Errorf("failed to read image: %w", err)
	}

	return data, nil
}

// getMimeType determines MIME type from file extension
func getMimeType(filename string) string {
	ext := strings.ToLower(filepath.Ext(filename))
	switch ext {
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".gif":
		return "image/gif"
	case ".webp":
		return "image/webp"
	case ".pdf":
		return "application/pdf"
	case ".mp4":
		return "video/mp4"
	case ".mov":
		return "video/quicktime"
	case ".doc":
		return "application/msword"
	case ".docx":
		return "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
	case ".xls":
		return "application/vnd.ms-excel"
	case ".xlsx":
		return "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
	case ".ppt":
		return "application/vnd.ms-powerpoint"
	case ".pptx":
		return "application/vnd.openxmlformats-officedocument.presentationml.presentation"
	default:
		return "application/octet-stream"
	}
}

// showMediaManager displays the media management dialog for a person
func showMediaManager(w fyne.Window, s *store.Store, personID int64, personName string) {
	// Load existing media
	mediaList, err := s.GetMediaForPerson(personID)
	if err != nil {
		dialog.ShowError(fmt.Errorf("Failed to load media: %w", err), w)
		return
	}

	// Create media list widget
	var listWidget *widget.List
	
	listWidget = widget.NewList(
		func() int {
			return len(mediaList)
		},
		func() fyne.CanvasObject {
			// Create widgets with accessible references
			thumbnail := canvas.NewImageFromResource(nil)
			thumbnail.FillMode = canvas.ImageFillContain
			thumbnail.SetMinSize(fyne.NewSize(80, 80))
			
			title := widget.NewLabel("Title")
			title.TextStyle = fyne.TextStyle{Bold: true}
			
			info := widget.NewLabel("Info")
			
			viewBtn := widget.NewButton("View", nil)
			actionBtn := widget.NewButton("Delete", nil)  // Will be "Unlink" or "Delete" depending on link count
			
			// Use HBox for predictable layout
			return container.NewHBox(
				thumbnail,
				container.NewVBox(title, info),
				viewBtn,
				actionBtn,
			)
		},
		func(id widget.ListItemID, item fyne.CanvasObject) {
			if id >= len(mediaList) {
				return
			}
			m := mediaList[id]
			
			// Access widgets from HBox by index
			hbox := item.(*fyne.Container)
			if len(hbox.Objects) < 4 {
				return
			}
			
			// Get widgets
			thumb := hbox.Objects[0].(*canvas.Image)
			textBox := hbox.Objects[1].(*fyne.Container)
			viewBtn := hbox.Objects[2].(*widget.Button)
			actionBtn := hbox.Objects[3].(*widget.Button)
			
			// Set thumbnail - always try to load from database
			// (GetMediaForPerson doesn't include the blob for performance)
			fullMedia, err := s.GetMediaByID(m.ID)
			if err == nil && len(fullMedia.Thumbnail) > 0 {
				img, _, err := image.Decode(bytes.NewReader(fullMedia.Thumbnail))
				if err == nil {
					thumb.Image = img
					thumb.Refresh()
				}
			} else {
				// No thumbnail - clear image
				thumb.Image = nil
				thumb.Refresh()
			}
			
			// Get link count to determine action button label
			linkCount, _ := s.GetMediaLinkCount(m.ID)
			
			// Set title and info
			if len(textBox.Objects) >= 2 {
				title := textBox.Objects[0].(*widget.Label)
				info := textBox.Objects[1].(*widget.Label)
				
				// Check for broken external links
				brokenLink := false
				if m.IsExternal {
					if _, err := os.Stat(m.ExternalPath); err != nil {
						brokenLink = true
					}
				}
				
				// Set title with visual indicators
				if brokenLink {
					title.SetText("⚠️ " + m.Title + " (BROKEN LINK)")
				} else if m.IsExternal {
					title.SetText("🔗 " + m.Title)
				} else {
					title.SetText(m.Title)
				}
				
				sizeStr := fmt.Sprintf("%.1f KB", float64(m.FileSize)/1024.0)
				linkInfo := ""
				if linkCount > 1 {
					linkInfo = fmt.Sprintf(" - Linked to %d people", linkCount)
				}
				if brokenLink {
					info.SetText(fmt.Sprintf("⚠️ External (FILE NOT FOUND) - %s - %s%s", m.MediaType, sizeStr, linkInfo))
				} else if m.IsExternal {
					info.SetText(fmt.Sprintf("🔗 External - %s - %s%s", m.MediaType, sizeStr, linkInfo))
				} else {
					info.SetText(fmt.Sprintf("Database - %s - %s%s", m.MediaType, sizeStr, linkInfo))
				}
			}
			
			// Set button actions
			viewBtn.OnTapped = func() {
				showMediaViewer(w, s, m.ID, personID)
			}
			
			// If multiple links, show "Unlink". If last link, show "Delete"
			if linkCount > 1 {
				actionBtn.SetText("Unlink")
				actionBtn.OnTapped = func() {
					dialog.ShowConfirm("Unlink Media",
						fmt.Sprintf("Remove this link from this person?\n\nThe media will remain linked to %d other %s.\n\n%s", 
							linkCount-1, 
							map[bool]string{true: "person", false: "people"}[linkCount == 2],
							m.Title),
						func(confirmed bool) {
							if !confirmed {
								return
							}
							if err := s.UnlinkMediaFromPerson(m.ID, personID); err != nil {
								dialog.ShowError(err, w)
								return
							}
							// Reload media list
							mediaList, _ = s.GetMediaForPerson(personID)
							listWidget.Refresh()
						}, w)
				}
			} else {
				actionBtn.SetText("Delete")
				actionBtn.OnTapped = func() {
					dialog.ShowConfirm("Delete Media",
						fmt.Sprintf("Permanently delete this media from the database?\n\n%s", m.Title),
						func(confirmed bool) {
							if !confirmed {
								return
							}
							if err := s.DeleteMedia(m.ID); err != nil {
								dialog.ShowError(err, w)
								return
							}
							// Reload media list
							mediaList, _ = s.GetMediaForPerson(personID)
							listWidget.Refresh()
						}, w)
				}
			}
		},
	)

	// Add media button
	addBtn := widget.NewButton("Add Media from File", func() {
		showAddMediaDialog(w, s, personID, func() {
			// Reload media list after adding
			mediaList, _ = s.GetMediaForPerson(personID)
			listWidget.Refresh()
		})
	})

	content := container.NewBorder(
		container.NewVBox(
			widget.NewLabelWithStyle(fmt.Sprintf("Media for %s", personName), 
				fyne.TextAlignCenter, fyne.TextStyle{Bold: true}),
			widget.NewSeparator(),
			addBtn,
		),
		nil, nil, nil,
		listWidget,
	)

	mediaDialog := dialog.NewCustom("Media Manager", "Close", content, w)
	mediaDialog.Resize(fyne.NewSize(700, 500))
	mediaDialog.Show()
}

// showAddMediaDialog shows dialog to add media from file
func showAddMediaDialog(w fyne.Window, s *store.Store, personID int64, onSuccess func()) {
	fd := dialog.NewFileOpen(func(reader fyne.URIReadCloser, err error) {
		if err != nil || reader == nil {
			return
		}
		defer reader.Close()
		
		imagePath := reader.URI().Path()
		filename := filepath.Base(imagePath)
		
		// Determine media type
		ext := strings.ToLower(filepath.Ext(filename))
		isImage := ext == ".jpg" || ext == ".jpeg" || ext == ".png" || ext == ".gif" || ext == ".webp"
		
		// Show progress
		progress := dialog.NewInformation("Processing", "Loading media...", w)
		progress.Show()
		
		var thumbnail []byte
		var fullImage []byte
		
		if isImage {
			// Generate thumbnail for images
			thumbnail, err = generateThumbnail(imagePath)
			if err != nil {
				progress.Hide()
				dialog.ShowError(fmt.Errorf("Failed to generate thumbnail: %w", err), w)
				return
			}
			
			// Read full image
			fullImage, err = readFullImage(imagePath)
			if err != nil {
				progress.Hide()
				dialog.ShowError(fmt.Errorf("Failed to read image: %w", err), w)
				return
			}
		} else {
			// For non-images (PDFs, videos), just read the file
			fullImage, err = readFullImage(imagePath)
			if err != nil {
				progress.Hide()
				dialog.ShowError(fmt.Errorf("Failed to read file: %w", err), w)
				return
			}
			// Generate placeholder thumbnail for non-images
			ext := strings.ToLower(filepath.Ext(imagePath))
			mediaType := "document"
			if ext == ".mp4" || ext == ".mov" {
				mediaType = "video"
			}
			thumbnail = createPlaceholderThumbnail(mediaType)
		}
		
		progress.Hide()
		
		// Show metadata dialog
		showMediaMetadataDialog(w, s, personID, filename, imagePath, thumbnail, fullImage, onSuccess)
	}, w)
	
	fd.SetFilter(storage.NewExtensionFileFilter([]string{
		".jpg", ".jpeg", ".png", ".gif", ".webp", 
		".mp4", ".mov", 
		".pdf", ".doc", ".docx", ".xls", ".xlsx", ".ppt", ".pptx"}))
	fd.Show()
}

// showMediaMetadataDialog shows dialog to enter media metadata
func showMediaMetadataDialog(w fyne.Window, s *store.Store, personID int64, filename, fullPath string, thumbnail, fullImage []byte, onSuccess func()) {
	titleEntry := widget.NewEntry()
	titleEntry.SetPlaceHolder("Title/Caption")
	titleEntry.SetText(strings.TrimSuffix(filename, filepath.Ext(filename)))
	
	descEntry := widget.NewMultiLineEntry()
	descEntry.SetPlaceHolder("Description (optional)")
	descEntry.SetMinRowsVisible(3)
	
	dateEntry := widget.NewEntry()
	dateEntry.SetPlaceHolder("YYYY-MM-DD (optional)")
	
	// Storage option
	storageOptions := []string{"Store in Database (recommended)", "Link to External File"}
	storageRadio := widget.NewRadioGroup(storageOptions, nil)
	storageRadio.SetSelected(storageOptions[0])
	storageRadio.Horizontal = false
	
	storageInfo := widget.NewLabel("External files remain in their original location. Database storage ensures portability.")
	storageInfo.Wrapping = fyne.TextWrapWord
	storageInfo.TextStyle = fyne.TextStyle{Italic: true}
	
	form := container.NewVBox(
		widget.NewLabel("Title:"),
		titleEntry,
		widget.NewLabel("Description:"),
		descEntry,
		widget.NewLabel("Date Taken:"),
		dateEntry,
		widget.NewSeparator(),
		widget.NewLabel("Storage:"),
		storageRadio,
		storageInfo,
	)
	
	dialog.ShowCustomConfirm("Media Details", "Save", "Cancel", form, func(save bool) {
		if !save {
			return
		}
		
		// Determine storage type
		isExternal := storageRadio.Selected == storageOptions[1]
		
		// Determine media type from file extension
		ext := strings.ToLower(filepath.Ext(filename))
		mediaType := "image" // default
		if ext == ".mp4" || ext == ".mov" {
			mediaType = "video"
		} else if ext == ".pdf" || ext == ".doc" || ext == ".docx" || ext == ".xls" || ext == ".xlsx" || ext == ".ppt" || ext == ".pptx" {
			mediaType = "document"
		}
		
		// Create media record
		media := &store.Media{
			Title:       titleEntry.Text,
			Description: descEntry.Text,
			MediaType:   mediaType,
			MimeType:    getMimeType(filename),
			Thumbnail:   thumbnail,
			DateTaken:   dateEntry.Text,
		}
		
		if isExternal {
			// Store as external reference
			media.IsExternal = true
			media.ExternalPath = fullPath
			media.FullImage = nil // Don't store the blob
			// Get file size from disk
			if fileInfo, err := os.Stat(fullPath); err == nil {
				media.FileSize = fileInfo.Size()
			}
		} else {
			// Store in database
			media.IsExternal = false
			media.ExternalPath = ""
			media.FullImage = fullImage
			media.FileSize = int64(len(fullImage))
		}
		
		if err := s.CreateMedia(media); err != nil {
			dialog.ShowError(fmt.Errorf("Failed to save media: %w", err), w)
			return
		}
		
		// Link media to person
		if err := s.LinkMediaToPerson(media.ID, personID); err != nil {
			dialog.ShowError(fmt.Errorf("Failed to link media to person: %w", err), w)
			return
		}
		
		dialog.ShowInformation("Success", "Media added successfully", w)
		if onSuccess != nil {
			onSuccess()
		}
	}, w)
}

// showEditMediaDialog shows dialog to edit media metadata
func showEditMediaDialog(w fyne.Window, s *store.Store, mediaID int64, onSuccess func()) {
	media, err := s.GetMediaByID(mediaID)
	if err != nil {
		dialog.ShowError(err, w)
		return
	}
	
	titleEntry := widget.NewEntry()
	titleEntry.SetPlaceHolder("Title/Caption")
	titleEntry.SetText(media.Title)
	
	descEntry := widget.NewMultiLineEntry()
	descEntry.SetPlaceHolder("Description")
	descEntry.SetText(media.Description)
	descEntry.SetMinRowsVisible(3)
	
	dateEntry := widget.NewEntry()
	dateEntry.SetPlaceHolder("YYYY-MM-DD")
	dateEntry.SetText(media.DateTaken)
	
	form := container.NewVBox(
		widget.NewLabel("Title:"),
		titleEntry,
		widget.NewLabel("Description:"),
		descEntry,
		widget.NewLabel("Date Taken:"),
		dateEntry,
	)
	
	dialog.ShowCustomConfirm("Edit Media", "Save", "Cancel", form, func(save bool) {
		if !save {
			return
		}
		
		media.Title = titleEntry.Text
		media.Description = descEntry.Text
		media.DateTaken = dateEntry.Text
		
		if err := s.UpdateMedia(media); err != nil {
			dialog.ShowError(err, w)
			return
		}
		
		dialog.ShowInformation("Success", "Media updated successfully", w)
		if onSuccess != nil {
			onSuccess()
		}
	}, w)
}

// showManageLinksDialog shows dialog to add/remove person links
func showManageLinksDialog(w fyne.Window, s *store.Store, mediaID int64, mediaTitle string, onSuccess func()) {
	// Get all people
	allPeople, err := s.GetPeople()
	if err != nil {
		dialog.ShowError(err, w)
		return
	}
	
	// Get currently linked people
	linkedPeopleIDs, err := s.GetLinkedPeopleForMedia(mediaID)
	if err != nil {
		dialog.ShowError(err, w)
		return
	}
	
	// Create map of linked people for quick lookup
	linkedMap := make(map[int64]bool)
	for _, id := range linkedPeopleIDs {
		linkedMap[id] = true
	}
	
	// Filter for searching
	filterEntry := widget.NewEntry()
	filterEntry.SetPlaceHolder("Filter by name...")
	
	filteredPeople := allPeople
	
	var peopleList *widget.List
	
	peopleList = widget.NewList(
		func() int {
			return len(filteredPeople)
		},
		func() fyne.CanvasObject {
			check := widget.NewCheck("", nil)
			label := widget.NewLabel("Name")
			return container.NewHBox(check, label)
		},
		func(id widget.ListItemID, item fyne.CanvasObject) {
			if id >= len(filteredPeople) {
				return
			}
			p := filteredPeople[id]
			
			hbox := item.(*fyne.Container)
			check := hbox.Objects[0].(*widget.Check)
			label := hbox.Objects[1].(*widget.Label)
			
			personName := fmt.Sprintf("%s %s", p.GivenName, p.Surname)
			if p.BirthDate != "" {
				personName += fmt.Sprintf(" (b. %s)", p.BirthDate)
			}
			label.SetText(personName)
			
			check.Checked = linkedMap[p.ID]
			check.OnChanged = func(checked bool) {
				linkedMap[p.ID] = checked
			}
		},
	)
	
	filterEntry.OnChanged = func(filterText string) {
		filterText = strings.ToLower(strings.TrimSpace(filterText))
		if filterText == "" {
			filteredPeople = allPeople
		} else {
			filteredPeople = []store.Person{}
			for _, p := range allPeople {
				fullName := strings.ToLower(fmt.Sprintf("%s %s", p.GivenName, p.Surname))
				if strings.Contains(fullName, filterText) {
					filteredPeople = append(filteredPeople, p)
				}
			}
		}
		peopleList.Refresh()
	}
	
	peopleScroll := container.NewScroll(peopleList)
	peopleScroll.SetMinSize(fyne.NewSize(400, 300))
	
	form := container.NewVBox(
		widget.NewLabelWithStyle(fmt.Sprintf("Manage links for: %s", mediaTitle),
			fyne.TextAlignCenter, fyne.TextStyle{Bold: true}),
		widget.NewSeparator(),
		widget.NewLabel("Check people to link, uncheck to unlink:"),
		filterEntry,
		peopleScroll,
	)
	
	dialog.ShowCustomConfirm("Manage Links", "Save", "Cancel", form, func(save bool) {
		if !save {
			return
		}
		
		// Find changes and apply them
		addedCount := 0
		removedCount := 0
		
		for _, person := range allPeople {
			wasLinked := false
			for _, linkedID := range linkedPeopleIDs {
				if person.ID == linkedID {
					wasLinked = true
					break
				}
			}
			
			isNowLinked := linkedMap[person.ID]
			
			if !wasLinked && isNowLinked {
				// Add link
				if err := s.LinkMediaToPerson(mediaID, person.ID); err == nil {
					addedCount++
				}
			} else if wasLinked && !isNowLinked {
				// Remove link
				if err := s.UnlinkMediaFromPerson(mediaID, person.ID); err == nil {
					removedCount++
				}
			}
		}
		
		msg := fmt.Sprintf("Links updated!\nAdded: %d\nRemoved: %d", addedCount, removedCount)
		dialog.ShowInformation("Success", msg, w)
		
		if onSuccess != nil {
			onSuccess()
		}
	}, w)
}

// showMediaLibraryViewer displays a full-size media viewer from the library
func showMediaLibraryViewer(w fyne.Window, s *store.Store, mediaID int64, onRefresh func()) {
	media, err := s.GetMediaByID(mediaID)
	if err != nil {
		dialog.ShowError(fmt.Errorf("Failed to load media: %w", err), w)
		return
	}
	
	// For non-images (PDFs, videos), open in default OS viewer
	if media.MediaType != "image" {
		if err := openMediaInViewer(w, media); err != nil {
			dialog.ShowError(fmt.Errorf("Failed to open media: %w", err), w)
		}
		return
	}
	
	// For images, display in our viewer
	var imageWidget *canvas.Image
	
	if media.IsExternal {
		// Try to load from external path
		if _, err := os.Stat(media.ExternalPath); err == nil {
			imageWidget = canvas.NewImageFromFile(media.ExternalPath)
		} else {
			dialog.ShowError(fmt.Errorf("External file not found: %s", media.ExternalPath), w)
			return
		}
	} else {
		// Load from database
		img, _, err := image.Decode(bytes.NewReader(media.FullImage))
		if err != nil {
			dialog.ShowError(fmt.Errorf("Failed to decode image: %w", err), w)
			return
		}
		imageWidget = canvas.NewImageFromImage(img)
	}
	
	imageWidget.FillMode = canvas.ImageFillContain
	
	// Create info labels
	titleLabel := widget.NewLabelWithStyle(media.Title, fyne.TextAlignCenter, fyne.TextStyle{Bold: true})
	
	infoText := fmt.Sprintf("Type: %s | Size: %.1f KB", media.MediaType, float64(media.FileSize)/1024.0)
	if media.DateTaken != "" {
		infoText += fmt.Sprintf(" | Date: %s", media.DateTaken)
	}
	infoLabel := widget.NewLabel(infoText)
	infoLabel.Alignment = fyne.TextAlignCenter
	
	descLabel := widget.NewLabel(media.Description)
	descLabel.Wrapping = fyne.TextWrapWord
	descLabel.Alignment = fyne.TextAlignCenter
	
	// Show linked people
	linkedPeopleText := ""
	if len(media.LinkedPeople) > 0 {
		linkedPeopleText = "Linked to: "
		names := []string{}
		for _, personID := range media.LinkedPeople {
			person, err := s.GetPersonByID(personID)
			if err == nil {
				names = append(names, fmt.Sprintf("%s %s", person.GivenName, person.Surname))
			}
		}
		linkedPeopleText += strings.Join(names, ", ")
	} else {
		linkedPeopleText = "Not linked to anyone"
	}
	
	linkedLabel := widget.NewLabel(linkedPeopleText)
	linkedLabel.Alignment = fyne.TextAlignCenter
	linkedLabel.TextStyle = fyne.TextStyle{Italic: true}
	
	content := container.NewBorder(
		container.NewVBox(titleLabel, infoLabel, descLabel, linkedLabel, widget.NewSeparator()),
		nil, nil, nil,
		container.NewScroll(imageWidget),
	)
	
	viewerDialog := dialog.NewCustom("Media Viewer", "Close", content, w)
	viewerDialog.Resize(fyne.NewSize(800, 600))
	viewerDialog.Show()
}

// showMediaViewer displays a full-size media viewer
func showMediaViewer(w fyne.Window, s *store.Store, mediaID int64, currentPersonID int64) {
	// Load full media
	media, err := s.GetMediaByID(mediaID)
	if err != nil {
		dialog.ShowError(fmt.Errorf("Failed to load media: %w", err), w)
		return
	}
	
	// For non-images (PDFs, videos), open in default OS viewer
	if media.MediaType != "image" {
		if err := openMediaInViewer(w, media); err != nil {
			dialog.ShowError(fmt.Errorf("Failed to open media: %w", err), w)
		}
		return
	}
	
	// For images, display in our viewer
	var imageWidget *canvas.Image
	
	if media.IsExternal {
		// Try to load from external path
		if _, err := os.Stat(media.ExternalPath); err == nil {
			imageWidget = canvas.NewImageFromFile(media.ExternalPath)
		} else {
			dialog.ShowError(fmt.Errorf("External file not found: %s", media.ExternalPath), w)
			return
		}
	} else {
		// Load from database
		img, _, err := image.Decode(bytes.NewReader(media.FullImage))
		if err != nil {
			dialog.ShowError(fmt.Errorf("Failed to decode image: %w", err), w)
			return
		}
		imageWidget = canvas.NewImageFromImage(img)
	}
	
	imageWidget.FillMode = canvas.ImageFillContain
	
	// Create info labels
	titleLabel := widget.NewLabelWithStyle(media.Title, fyne.TextAlignCenter, fyne.TextStyle{Bold: true})
	
	infoText := fmt.Sprintf("Type: %s | Size: %.1f KB", media.MediaType, float64(media.FileSize)/1024.0)
	if media.DateTaken != "" {
		infoText += fmt.Sprintf(" | Date: %s", media.DateTaken)
	}
	infoLabel := widget.NewLabel(infoText)
	infoLabel.Alignment = fyne.TextAlignCenter
	
	descLabel := widget.NewLabel(media.Description)
	descLabel.Wrapping = fyne.TextWrapWord
	descLabel.Alignment = fyne.TextAlignCenter
	
	// Show linked people
	linkedPeopleText := ""
	if len(media.LinkedPeople) > 0 {
		linkedPeopleText = "Also linked to: "
		names := []string{}
		for _, personID := range media.LinkedPeople {
			if personID != currentPersonID {
				person, err := s.GetPersonByID(personID)
				if err == nil {
					names = append(names, fmt.Sprintf("%s %s", person.GivenName, person.Surname))
				}
			}
		}
		if len(names) > 0 {
			linkedPeopleText += strings.Join(names, ", ")
		} else {
			linkedPeopleText = ""
		}
	}
	
	headerContent := container.NewVBox(titleLabel, infoLabel, descLabel)
	if linkedPeopleText != "" {
		linkedLabel := widget.NewLabel(linkedPeopleText)
		linkedLabel.Alignment = fyne.TextAlignCenter
		linkedLabel.TextStyle = fyne.TextStyle{Italic: true}
		headerContent.Add(linkedLabel)
	}
	headerContent.Add(widget.NewSeparator())
	
	content := container.NewBorder(
		headerContent,
		nil, nil, nil,
		container.NewScroll(imageWidget),
	)
	
	viewerDialog := dialog.NewCustom("Media Viewer", "Close", content, w)
	viewerDialog.Resize(fyne.NewSize(800, 600))
	viewerDialog.Show()
}
