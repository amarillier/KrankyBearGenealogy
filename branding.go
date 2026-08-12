package main

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
)

// Dialog branding matches my standards about.go / help.go / update.go (256×256).
const brandingImageSizeDialog = 256

// "Ahead of latest release" badge, shown beside -- not instead of -- the
// About/Help/Update dialogs' app icon when this build is newer than the
// latest published GitHub release (see update.go's showUpdateDialog,
// about.go's showAbout, and help.go's showHelp). Sized to read clearly next
// to the dialog icon without competing with it.
const brandingImageSizeBadge = 64

func newBrandingDialogImage(res fyne.Resource) *canvas.Image {
	img := canvas.NewImageFromResource(res)
	img.FillMode = canvas.ImageFillContain
	img.SetMinSize(fyne.NewSize(brandingImageSizeDialog, brandingImageSizeDialog))
	return img
}

func newBrandingBadgeImage(res fyne.Resource) *canvas.Image {
	img := canvas.NewImageFromResource(res)
	img.FillMode = canvas.ImageFillContain
	img.SetMinSize(fyne.NewSize(brandingImageSizeBadge, brandingImageSizeBadge))
	return img
}

// "Now this is not the end. It is not even the beginning of the end. But it is, perhaps, the end of the beginning." Winston Churchill, November 10, 1942
