package startup

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// fixture is an executable directory with the Mesa files staged in mesa-fallback/, plus a recording env.
type fixture struct {
	dir       string
	e         mfEnv
	probes    int
	relaunch  [][]string
	vars      map[string]string
	probeErr  error
	sse41     bool
	relaunchF bool // simulate a relaunch that could not start (returns instead of exiting)
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	f := &fixture{dir: t.TempDir(), vars: map[string]string{}, sse41: true}
	os.MkdirAll(filepath.Join(f.dir, mesaFallbackDirName), 0o755)
	for _, n := range append([]string{"MESA_VERSION"}, mesaFallbackFiles...) {
		os.WriteFile(filepath.Join(f.dir, mesaFallbackDirName, n), []byte("x"), 0o644)
	}
	os.WriteFile(filepath.Join(f.dir, mesaFallbackDirName, "MESA_VERSION"), []byte("26.2.4\n"), 0o644)
	f.e = mfEnv{
		exeDir:   func() (string, error) { return f.dir, nil },
		hasSSE41: func() bool { return f.sse41 },
		probe:    func() error { f.probes++; return f.probeErr },
		relaunch: func(args ...string) { f.relaunch = append(f.relaunch, args) },
		getenv:   func(k string) string { return f.vars[k] },
		setenv:   func(k, v string) { f.vars[k] = v },
	}
	return f
}

func (f *fixture) has(name string) bool {
	_, err := os.Stat(filepath.Join(f.dir, name))
	return err == nil
}

func (f *fixture) marker(name string) {
	os.WriteFile(filepath.Join(f.dir, mesaFallbackDirName, name), nil, 0o644)
}

func TestWorkingHardwareOpenGLLeavesEverythingAlone(t *testing.T) {
	f := newFixture(t)
	f.e.ensure(false, false, false)
	if f.probes != 1 || len(f.relaunch) != 0 || f.has("opengl32.dll") || f.vars["GALLIUM_DRIVER"] != "" {
		t.Errorf("probes %d relaunch %v moved %v env %v", f.probes, f.relaunch, f.has("opengl32.dll"), f.vars)
	}
}

func TestFailedProbeRelaunchesWithTheFallbackFlagWithoutMovingFiles(t *testing.T) {
	f := newFixture(t)
	f.probeErr = errors.New("no GL")
	f.e.ensure(false, false, false)
	if !reflect.DeepEqual(f.relaunch, [][]string{{"-mesa-gl-fallback"}}) {
		t.Errorf("relaunch: %v", f.relaunch)
	}
	if f.has("opengl32.dll") {
		t.Error("DLLs moved before the relaunch decided they are needed")
	}
}

func TestRelaunchedProcessMovesTheDLLsAndRelaunchesOnceMoreWithoutProbing(t *testing.T) {
	f := newFixture(t)
	f.e.ensure(true, false, false)
	if f.probes != 0 {
		t.Error("probed again: a broken host could loop forever")
	}
	if !f.has("opengl32.dll") || !f.has("libgallium_wgl.dll") || f.has(filepath.Join(mesaFallbackDirName, "opengl32.dll")) {
		t.Error("DLLs not moved into the app directory")
	}
	if len(f.relaunch) != 1 {
		t.Errorf("relaunch: %v", f.relaunch)
	}
	// the relaunch failed to start (it returned): carry on in this process with the driver set
	if f.vars["GALLIUM_DRIVER"] != "llvmpipe" {
		t.Errorf("driver: %q", f.vars["GALLIUM_DRIVER"])
	}
}

func TestAlreadyDeployedSkipsProbingAndJustSetsTheDriver(t *testing.T) {
	f := newFixture(t)
	f.e.moveIntoAppDir()
	f.e.ensure(false, false, false)
	if f.probes != 0 || len(f.relaunch) != 0 || f.vars["GALLIUM_DRIVER"] != "llvmpipe" {
		t.Errorf("probes %d relaunch %v env %v", f.probes, f.relaunch, f.vars)
	}
}

func TestForceMarkerSkipsTheProbe(t *testing.T) {
	f := newFixture(t)
	f.marker(mesaForceMarkerFileName)
	f.e.ensure(false, false, false)
	if f.probes != 0 || !f.has("opengl32.dll") || len(f.relaunch) != 1 {
		t.Errorf("probes %d moved %v relaunch %v", f.probes, f.has("opengl32.dll"), f.relaunch)
	}
}

func TestDriverChoice(t *testing.T) {
	cases := []struct {
		name         string
		sse41        bool
		compat, perf bool
		markers      []string
		want         string
	}{
		{"normal CPU", true, false, false, nil, "llvmpipe"},
		{"conservative vCPU (no SSE4.1)", false, false, false, nil, "softpipe"},
		{"compat flag", true, true, false, nil, "softpipe"},
		{"compat marker", true, false, false, []string{mesaForceCompatMarkerFileName}, "softpipe"},
		{"performance flag beats a conservative CPU", false, false, true, nil, "llvmpipe"},
		{"performance marker beats a conservative CPU", false, false, false, []string{mesaForcePerformanceMarkerFileName}, "llvmpipe"},
		{"performance wins over compat", true, true, true, nil, "llvmpipe"},
	}
	for _, c := range cases {
		f := newFixture(t)
		f.sse41 = c.sse41
		for _, m := range c.markers {
			f.marker(m)
		}
		if got := f.e.chooseGalliumDriver(c.compat, c.perf); got != c.want {
			t.Errorf("%s: got %s want %s", c.name, got, c.want)
		}
	}
}

func TestStatusReportsOnlyWhenTheFallbackDriverIsSet(t *testing.T) {
	f := newFixture(t)
	if a, _, _ := f.e.status(); a {
		t.Error("active with no GALLIUM_DRIVER")
	}
	f.vars["GALLIUM_DRIVER"] = "softpipe"
	a, v, d := f.e.status()
	if !a || v != "26.2.4" || d != "softpipe" {
		t.Errorf("status: %v %q %q", a, v, d)
	}
	f.vars["GALLIUM_DRIVER"] = "radeonsi" // someone else's driver
	if a, _, _ := f.e.status(); a {
		t.Error("a real GPU driver reported as the fallback")
	}
	os.Remove(filepath.Join(f.dir, mesaFallbackDirName, "MESA_VERSION"))
	f.vars["GALLIUM_DRIVER"] = "llvmpipe"
	if a, v, _ := f.e.status(); !a || v != "" {
		t.Errorf("missing version file: %v %q", a, v)
	}
}

func TestMoveDoesNotOverwriteAndToleratesMissingFiles(t *testing.T) {
	f := newFixture(t)
	os.WriteFile(filepath.Join(f.dir, "opengl32.dll"), []byte("placed by hand"), 0o644)
	os.Remove(filepath.Join(f.dir, mesaFallbackDirName, "libgallium_wgl.dll"))
	f.e.moveIntoAppDir() // must not panic or clobber
	if b, _ := os.ReadFile(filepath.Join(f.dir, "opengl32.dll")); string(b) != "placed by hand" {
		t.Errorf("hand-placed DLL overwritten: %q", b)
	}
	if f.has("libgallium_wgl.dll") {
		t.Error("created a file out of nothing")
	}
}
