package main

import (
	"flag"
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
	"genealogy/internal/startup"
	"genealogy/store"
	"genealogy/ui"
)

const (
	// appName    = "KrankyBear Genealogy"
	appVersion = "1.7.0" // see FyneApp.toml
	appAuthor  = "Allan Marillier"
)

var appName = "KrankyBear Genealogy"
var appCopyright = "Copyright (c) Allan Marillier, 2026-" + strconv.Itoa(time.Now().Year())

// Window tracking variables (set to nil when closed)
var aboutWindow fyne.Window
var helpWindow fyne.Window
var updateWindow fyne.Window

func main() {
	mesaFallbackFlag := flag.Bool(startup.MesaFallbackFlagName, false, "internal: relaunch flag for the Mesa3D OpenGL fallback (Windows only)")
	flag.Parse()

	// Probes hardware OpenGL and relaunches under a bundled Mesa3D software
	// renderer if the probe fails (Windows only; no-op elsewhere). Must run
	// before app.NewWithID -- GLFW allows one Init/Terminate cycle per
	// process, shared with Fyne's own driver.
	startup.EnsureWindowsOpenGLReady(*mesaFallbackFlag)

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

	// Check for updates at startup (quiet, throttled once/day; dialog only if an update exists)
	checkForUpdatesAuto(a)

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
		func() { checkForUpdatesManual(a) },
	)

	ui.RunApp(a, s, cfg, dbPath)
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// "Now this is not the end. It is not even the beginning of the end. But it is, perhaps, the end of the beginning." Winston Churchill, November 10, 1942
