// Package main provides update checking dialog
// Note: About and Help dialogs have been moved to separate files:
//   - about.go: About dialog (reusable)
//   - help.go: Help dialog (reusable)
//   - update.go: Update checker dialog (this file)
package main

import (
	"net/url"
	"sync/atomic"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

// aheadOfLatestRelease caches the most recent update check's verdict on
// whether this build is newer than the latest published GitHub release (an
// unpublished/dev build) -- set by checkForUpdatesManual/Auto below, and read
// by about.go's showAbout and help.go's showHelp so their HardHat badge
// reflects the last known state without running its own network check.
var aheadOfLatestRelease atomic.Bool

// Repo checked by "Check for Updates".
const (
	updateRepoOwner = "amarillier"
	updateRepo      = "KrankyBearGenealogy"
	updateRepoDL    = "https://github.com/amarillier/KrankyBearGenealogy/releases/latest"
)

// checkForUpdatesAuto runs a quiet, throttled (once/day) check on launch and
// only pops a dialog when an update is actually available.
func checkForUpdatesAuto(a fyne.App) {
	go func() {
		msg, available, remoteTag := updateChecker(updateRepoOwner, updateRepo, appName, updateRepoDL, 1)
		ahead := versionIsNewer(appVersion, remoteTag)
		aheadOfLatestRelease.Store(ahead)
		if !available {
			return
		}
		fyne.Do(func() { showUpdateDialog(a, msg, available, ahead) })
	}()
}

// checkForUpdatesManual runs an unthrottled check and always shows the
// result, for the Help menu "Check for Updates" action.
func checkForUpdatesManual(a fyne.App) {
	go func() {
		msg, available, remoteTag := updateChecker(updateRepoOwner, updateRepo, appName, updateRepoDL, 0)
		ahead := versionIsNewer(appVersion, remoteTag)
		aheadOfLatestRelease.Store(ahead)
		fyne.Do(func() { showUpdateDialog(a, msg, available, ahead) })
	}()
}

// showUpdateDialog shows the update-check result. When ahead is true (this
// build is newer than the latest published release -- an unpublished/dev
// build), a small HardHat badge appears beside the normal app icon rather
// than replacing it, so the dialog still reads as this app with a highlight,
// not a different app.
func showUpdateDialog(a fyne.App, message string, updateAvailable bool, ahead bool) {
	if updateWindow != nil && updateWindow.Content().Visible() {
		updateWindow.Show()
		updateWindow.RequestFocus()
		return
	}

	updateWindow = a.NewWindow(appName + " - Update Check")
	updateWindow.SetIcon(resourceKrankyBearGenealogyPng)

	// App icon, scaled to fit at a fixed 256x256.
	icon := newBrandingDialogImage(resourceKrankyBearGenealogyPng)

	var iconDisplay fyne.CanvasObject = icon
	if ahead {
		badge := newBrandingBadgeImage(resourceKrankyBearHardHatPng)
		iconDisplay = container.NewHBox(icon, badge)
	}

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
			container.NewCenter(iconDisplay),
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
			container.NewCenter(iconDisplay),
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
