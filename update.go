// Package main provides update checking dialog
// Note: About and Help dialogs have been moved to separate files:
//   - about.go: About dialog (reusable)
//   - help.go: Help dialog (reusable)
//   - dialogs.go: Update checker dialog (this file)
package main

import (
	"net/url"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

func showUpdateDialog(a fyne.App, message string, updateAvailable bool) {
	if updateWindow != nil && updateWindow.Content().Visible() {
		updateWindow.Show()
		updateWindow.RequestFocus()
		return
	}

	updateWindow = a.NewWindow(appName + " - Update Check")
	updateWindow.SetIcon(resourceKrankyBearGenealogyPng)

	// Detect if running newer version (development/testing)
	isNewerVersion := strings.Contains(message, "newer version")

	// Choose icon based on status
	var iconResource *fyne.StaticResource
	if isNewerVersion {
		iconResource = resourceKrankyBearHardHatPng // Hard hat for development version
	} else {
		iconResource = resourceKrankyBearGenealogyPng // Normal icon for update available or up to date
	}

	icon := canvas.NewImageFromResource(iconResource)
	icon.FillMode = canvas.ImageFillContain
	icon.SetMinSize(fyne.NewSize(256, 256))

	messageLabel := widget.NewLabel(message)
	messageLabel.Wrapping = fyne.TextWrapWord
	messageLabel.Alignment = fyne.TextAlignCenter

	var content *fyne.Container
	if updateAvailable {
		releaseURL, _ := url.Parse("https://github.com/amarillier/KrankyBearGenealogy/releases/latest")
		releaseLink := widget.NewHyperlink("Download Latest Release", releaseURL)
		releaseLink.Alignment = fyne.TextAlignCenter

		notesURL, _ := url.Parse("https://github.com/amarillier/KrankyBearGenealogy/blob/allanm/ReleaseNotes.txt")
		notesLink := widget.NewHyperlink("View Release Notes", notesURL)
		notesLink.Alignment = fyne.TextAlignCenter

		// Links - update URLs for your project
		licenseURL, _ := url.Parse("https://github.com/amarillier/KrankyBearGenealogy/blob/allanm/LICENSE")
		licenseLink := widget.NewHyperlink("License Information", licenseURL)
		licenseLink.Alignment = fyne.TextAlignCenter

		githubURL, _ := url.Parse("https://github.com/amarillier/KrankyBearGenealogy")
		githubLink := widget.NewHyperlink("GitHub Repository", githubURL)
		githubLink.Alignment = fyne.TextAlignCenter

		content = container.NewVBox(
			container.NewCenter(icon),
			widget.NewSeparator(),
			messageLabel,
			widget.NewSeparator(),
			container.NewCenter(releaseLink),
			container.NewCenter(notesLink),
			container.NewCenter(licenseLink),
			container.NewCenter(githubLink),
		)
	} else {
		// Create releases link for "up to date" message
		releasesURL, _ := url.Parse("https://github.com/amarillier/KrankyBearGenealogy/releases")
		releasesLink := widget.NewHyperlink("View Releases", releasesURL)
		releasesLink.Alignment = fyne.TextAlignCenter

		notesURL, _ := url.Parse("https://github.com/amarillier/KrankyBearGenealogy/blob/allanm/ReleaseNotes.txt")
		notesLink := widget.NewHyperlink("View Release Notes", notesURL)
		notesLink.Alignment = fyne.TextAlignCenter

		// Links - update URLs for your project
		licenseURL, _ := url.Parse("https://github.com/amarillier/KrankyBearGenealogy/blob/allanm/LICENSE")
		licenseLink := widget.NewHyperlink("License Information", licenseURL)
		licenseLink.Alignment = fyne.TextAlignCenter

		githubURL, _ := url.Parse("https://github.com/amarillier/KrankyBearGenealogy")
		githubLink := widget.NewHyperlink("GitHub Repository", githubURL)
		githubLink.Alignment = fyne.TextAlignCenter

		content = container.NewVBox(
			container.NewCenter(icon),
			widget.NewSeparator(),
			messageLabel,
			widget.NewSeparator(),
			container.NewCenter(releasesLink),
			container.NewCenter(notesLink),
			container.NewCenter(licenseLink),
			container.NewCenter(githubLink),
		)
	}

	updateWindow.SetContent(container.NewPadded(content))
	updateWindow.Resize(fyne.NewSize(550, 300))

	updateWindow.SetCloseIntercept(func() {
		updateWindow.Hide()
	})

	updateWindow.Show()
	updateWindow.RequestFocus() // Bring window to front
}

// "Now this is not the end. It is not even the beginning of the end. But it is, perhaps, the end of the beginning." Winston Churchill, November 10, 1942
