package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/driver/desktop"

	"genealogy/config"
	"genealogy/store"
	"genealogy/ui"

	updatechecker "github.com/amarillier/go-update-checker"
)

const (
	// appName    = "KrankyBear Genealogy"
	appVersion = "1.4.0" // see FyneApp.toml
	appAuthor  = "Allan Marillier"
)

var appName = "KrankyBear Genealogy"
var appCopyright = "Copyright (c) Allan Marillier, 2026-" + strconv.Itoa(time.Now().Year())

// Window tracking variables (set to nil when closed)
var aboutWindow fyne.Window
var helpWindow fyne.Window
var updateWindow fyne.Window

func main() {
	// Create Fyne application
	a := app.NewWithID("com.github.amarillier.KrankyBearGenealogy")

	// Load theme preference
	loadTheme(a)

	// Load config to get last database
	cfg, err := config.Load()
	if err != nil {
		log.Printf("config load warning: %v", err)
		cfg = &config.Config{}
	}

	// Determine database path
	var dbPath string
	if cfg.LastDatabase != "" && fileExists(cfg.LastDatabase) {
		dbPath = cfg.LastDatabase
		fmt.Printf("Opening last database: %s\n", dbPath)
	} else {
		// Default to data/genealogy.db in current directory
		cwd, err := os.Getwd()
		if err != nil {
			log.Fatalf("getwd: %v", err)
		}
		dbPath = filepath.Join(cwd, "data", "genealogy.db")
		fmt.Printf("Opening default database: %s\n", dbPath)
	}

	s, err := store.Open(dbPath)
	if err != nil {
		log.Fatalf("open db: %v", err)
	}
	defer s.Close()

	if err := s.InitSchema(); err != nil {
		log.Fatalf("init schema: %v", err)
	}

	// Apply any pending migrations
	if err := s.MigrateSchema(); err != nil {
		log.Fatalf("migrate schema: %v", err)
	}

	if n, err := s.CountPeople(); err == nil && n == 0 {
		// seed minimal sample data
		_ = s.CreatePerson(&store.Person{GivenName: "John", Surname: "Doe", Gender: "M"})
		_ = s.CreatePerson(&store.Person{GivenName: "Jane", Surname: "Doe", Gender: "F"})
		fmt.Println("Seeded sample people")
	}

	// Save this database as the last used
	cfg.AddRecentDatabase(dbPath)
	_ = cfg.Save()

	// Check for updates at startup
	updtmsg, updateAvail := updateChecker("amarillier", "KrankyBearGenealogy", appName, "https://github.com/amarillier/KrankyBearGenealogy/releases/latest")
	if updateAvail {
		showUpdateDialog(a, updtmsg, true)
	}

	// Setup system tray icon (if supported)
	if desk, ok := a.(desktop.App); ok {
		desk.SetSystemTrayIcon(resourceKrankyBearGenealogy64Png)
	}

	// Pass theme and dialog functions to UI
	ui.SetThemeFunctions(
		func() { setLightTheme(a) },
		func() { setDarkTheme(a) },
		func() { setSystemTheme(a) },
	)
	ui.SetDialogFunctions(
		func() { showAbout(a) },
		func() { showHelp(a) },
		func() {
			updtmsg, updateAvail := updateChecker("amarillier", "KrankyBearGenealogy", appName, "https://github.com/amarillier/KrankyBearGenealogy/releases/latest")
			showUpdateDialog(a, updtmsg, updateAvail)
		},
	)

	ui.RunApp(a, s, cfg, dbPath)
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func updateChecker(repoOwner string, repo string, repoName string, repodl string) (string, bool) {
	uc := updatechecker.New(repoOwner, repo, repoName, repodl, 0, false)
	uc.CheckForUpdate(appVersion)
	updtmsg := uc.Message
	return updtmsg, uc.UpdateAvailable
}

// "Now this is not the end. It is not even the beginning of the end. But it is, perhaps, the end of the beginning." Winston Churchill, November 10, 1942
