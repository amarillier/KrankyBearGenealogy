package main

import (
	"os"
	"path/filepath"
	"strings"

	updatechecker "github.com/amarillier/go-update-checker"
	"github.com/hashicorp/go-version"
)

// updateCheckStatePath returns the per-user path where the update checker
// caches the last-seen release tag, namespaced by repo (the library only
// caches tag/name, not which repo, so a bare filename could clash if this
// app ever checks more than one repo).
func updateCheckStatePath(repo string) string {
	dir, err := os.UserConfigDir()
	if err != nil {
		return ""
	}
	return filepath.Join(dir, appName, "latestcheck-"+repo+".json")
}

// updateChecker checks repoOwner/repo's latest published GitHub release against
// appVersion. minDaysInterval throttles the check: 0 = never throttled (manual
// "Check for Updates"), >0 for a quiet automatic check on launch (see
// checkForUpdatesAuto in update.go). remoteTag is the latest release's tag, so
// callers can tell via versionIsNewer when this build is *ahead* of it (an
// unpublished/dev build), not just whether an update is available.
func updateChecker(repoOwner string, repo string, repoName string, repodl string, minDaysInterval int) (msg string, updateAvailable bool, remoteTag string) {
	if p := updateCheckStatePath(repo); p != "" {
		updatechecker.SetCheckStatePath(p)
	}
	uc := updatechecker.New(repoOwner, repo, repoName, repodl, minDaysInterval, false)
	uc.CheckForUpdate(appVersion)
	return uc.Message, uc.UpdateAvailable, uc.RemoteTag
}

// versionIsNewer reports whether local is a strictly newer semantic version
// than remote -- i.e. this build is ahead of the latest published release (an
// unpublished/dev build, shown with the HardHat badge rather than reported as
// "up to date"). Returns false if remote is empty (e.g. no published release
// was found, or the check was throttled with no prior cache) or either side
// fails to parse, since neither case can be confidently called "ahead".
func versionIsNewer(local, remote string) bool {
	if strings.TrimSpace(remote) == "" {
		return false
	}
	lv, errL := version.NewVersion(strings.TrimPrefix(strings.TrimSpace(local), "v"))
	rv, errR := version.NewVersion(strings.TrimPrefix(strings.TrimSpace(remote), "v"))
	if errL != nil || errR != nil {
		return false
	}
	return lv.GreaterThan(rv)
}

// "Now this is not the end. It is not even the beginning of the end. But it is, perhaps, the end of the beginning." Winston Churchill, November 10, 1942
