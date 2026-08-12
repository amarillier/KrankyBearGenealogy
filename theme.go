//go:generate fyne bundle -o bundled.go assets/images/KrankyBearGenealogy.png
//go:generate fyne bundle -o bundled.go -a assets/images/KrankyBearGenealogy64.png

package main

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
)

// Custom theme struct (available for future customization)
// Currently using standard Fyne themes
// Uncomment and use if custom sizing/colors needed:
//
// type appTheme struct {
// 	fyne.Theme
// }
//
// func (a *appTheme) Size(n fyne.ThemeSizeName) float32 {
// 	if n == theme.SizeNameHeadingText {
// 		return a.Theme.Size(n) * 1.5
// 	}
// 	return a.Theme.Size(n)
// }

// Theme switching functions (from KrankyBearLaunchPad)
func setLightTheme(a fyne.App) {
	a.Settings().SetTheme(theme.LightTheme())
	a.Preferences().SetString("theme", "light")
}

func setDarkTheme(a fyne.App) {
	a.Settings().SetTheme(theme.DarkTheme())
	a.Preferences().SetString("theme", "dark")
}

func setSystemTheme(a fyne.App) {
	a.Settings().SetTheme(theme.DefaultTheme())
	a.Preferences().SetString("theme", "system")
}

func loadTheme(a fyne.App) {
	themePref := a.Preferences().StringWithFallback("theme", "system")
	switch themePref {
	case "light":
		a.Settings().SetTheme(theme.LightTheme())
	case "dark":
		a.Settings().SetTheme(theme.DarkTheme())
	default:
		a.Settings().SetTheme(theme.DefaultTheme())
	}
}

// "Now this is not the end. It is not even the beginning of the end. But it is, perhaps, the end of the beginning." Winston Churchill, November 10, 1942
