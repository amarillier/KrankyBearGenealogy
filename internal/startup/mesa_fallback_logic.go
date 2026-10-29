package startup

// The decision logic of the Mesa3D software-OpenGL fallback, with every Windows-only call (probing OpenGL,
// relaunching, the CPU check, the executable's path) injected through mfEnv. That keeps it compiled and unit-tested on
// every OS; mesa_fallback_windows.go supplies the real bindings. See EnsureWindowsOpenGLReady for the mechanism.

import (
	"os"
	"path/filepath"
	"strings"
)

// mesaFallbackDirName is the installer-staged subfolder (see the project's Inno .iss / installerbear.yaml) holding
// Mesa3D's software OpenGL DLLs (opengl32.dll + libgallium_wgl.dll). It is kept out of the exe's own directory so
// ordinary launches keep using the real system OpenGL. Only on a confirmed hardware-probe failure are the two files moved
// out of here and into the app's own directory, where Windows' default DLL search order picks them up.
//
// windows.SetDllDirectory was tried first (in ../TaniumMigrator, where this mechanism originates) as a way to avoid
// touching files in the (possibly non-admin-writable) install directory, but proved unreliable in the field: GLFW's plain
// LoadLibraryA("opengl32.dll") kept resolving to the real system DLL even with SetDllDirectory pointed at this folder.
// Moving the files is the mechanism actually proven to work.
const mesaFallbackDirName = "mesa-fallback"

// Marker files inside mesaFallbackDirName. Only their names matter; the text in the .sample files is never read. They are
// checked on every launch, not just the first, so they also work as a recovery path: if llvmpipe hangs after it was already
// chosen, force-quit the app (Task Manager), create .force-mesa-compat, relaunch. No admin needed, e.g.:
//
//	New-Item "C:\Program Files\<App>\mesa-fallback\.force-mesa-compat" -ItemType File
const (
	mesaForceMarkerFileName            = ".force-mesa-fallback"    // skip the hardware probe and always use Mesa
	mesaForceCompatMarkerFileName      = ".force-mesa-compat"      // use the safe softpipe rasterizer
	mesaForcePerformanceMarkerFileName = ".force-mesa-performance" // use the fast llvmpipe rasterizer even on a conservative CPU
)

// mesaFallbackFiles are the Mesa3D files moved from mesaFallbackDirName into the app's own directory.
var mesaFallbackFiles = []string{"opengl32.dll", "libgallium_wgl.dll"}

// mesaGalliumDriverFast is Mesa's default Gallium driver: it JIT-compiles its rasterizer per detected CPU features, which
// is why it is much faster than mesaGalliumDriverSafe for interactive use (software rendering with softpipe on every
// launch was the performance problem found in TaniumMigrator). See the next constant for the risk the JIT path carries.
const mesaGalliumDriverFast = "llvmpipe"

// mesaGalliumDriverSafe is Mesa's non-JIT reference rasterizer. llvmpipe hangs after the first frame under some
// hypervisors' conservative default vCPU models (confirmed on Proxmox's kvm64/qemu64, both of which predate SSE4.1),
// because its JIT compiles against whatever instruction set it detects. softpipe is slower but immune. Harmless if a real GPU
// driver ends up loaded instead: that driver ignores this Mesa-only environment variable.
const mesaGalliumDriverSafe = "softpipe"

// mfEnv is everything the decision touches outside plain file operations, so tests can replace it.
type mfEnv struct {
	exeDir   func() (string, error)
	hasSSE41 func() bool
	probe    func() error // nil = real hardware OpenGL works
	// relaunch starts a fresh copy of this program with args appended and exits this one. It returns only if the copy
	// could not be started.
	relaunch func(args ...string)
	getenv   func(string) string
	setenv   func(k, v string)
}

func (e mfEnv) marker(name string) bool {
	dir, err := e.exeDir()
	if err != nil {
		return false
	}
	_, err = os.Stat(filepath.Join(dir, mesaFallbackDirName, name))
	return err == nil
}

// chooseGalliumDriver decides which Gallium rasterizer to force. llvmpipe is preferred for speed. softpipe is used when the
// operator asked for it (forceCompat, or the .force-mesa-compat marker) or, absent either override, when the CPU looks like the
// conservative/compatibility vCPU model class the llvmpipe hang was found on: real CPUs from roughly the last 15+ years all
// have SSE4.1, but a hypervisor's "maximum compatibility" default model (Proxmox kvm64/qemu64) deliberately does not expose it.
// forcePerformance (or .force-mesa-performance) overrides that automatic pick back to llvmpipe, for an operator who has
// confirmed their specific host does not hit the hang. This is a best-effort translation of "conservative vCPU" into something
// checkable at run time, not a verified reproduction on every hypervisor, which is why the flags and markers exist.
func (e mfEnv) chooseGalliumDriver(forceCompat, forcePerformance bool) string {
	if forcePerformance || e.marker(mesaForcePerformanceMarkerFileName) {
		return mesaGalliumDriverFast
	}
	if forceCompat || e.marker(mesaForceCompatMarkerFileName) || !e.hasSSE41() {
		return mesaGalliumDriverSafe
	}
	return mesaGalliumDriverFast
}

// deployed reports whether the DLLs already sit next to the executable: a previous launch moved them, or an operator placed
// them by hand. Checking the first file is enough because they are always moved together.
func (e mfEnv) deployed() bool {
	dir, err := e.exeDir()
	if err != nil {
		return false
	}
	_, err = os.Stat(filepath.Join(dir, mesaFallbackFiles[0]))
	return err == nil
}

// moveIntoAppDir moves the DLLs out of mesaFallbackDirName into the app's own directory. Best effort: with no write access
// they stay where they were, the probe that follows fails again, and the normal fatal message appears, which is the right
// degradation for a locked-down host that needs an administrator (or the two files copied by hand).
func (e mfEnv) moveIntoAppDir() {
	dir, err := e.exeDir()
	if err != nil {
		return
	}
	for _, name := range mesaFallbackFiles {
		dst := filepath.Join(dir, name)
		if _, err := os.Stat(dst); err == nil {
			continue // already there, e.g. a half-finished earlier attempt
		}
		_ = os.Rename(filepath.Join(dir, mesaFallbackDirName, name), dst)
	}
}

// ensure is the whole decision (see EnsureWindowsOpenGLReady for the narrative).
func (e mfEnv) ensure(alreadyUsingFallback, forceCompat, forcePerformance bool) {
	driver := func() { e.setenv("GALLIUM_DRIVER", e.chooseGalliumDriver(forceCompat, forcePerformance)) }
	if e.deployed() {
		driver() // Windows finds the DLLs next to the exe by itself
		return
	}
	if alreadyUsingFallback || e.marker(mesaForceMarkerFileName) {
		e.moveIntoAppDir()
		// Relaunch even though the files are in place: Windows resolves a DLL's search path once per process, and in testing a
		// brand-new process after the move succeeded where the one that did the move did not.
		e.relaunch("-" + MesaFallbackFlagName)
		driver() // only reached if the relaunch could not start: carry on in this process
		return
	}
	if e.probe() == nil {
		return // real hardware OpenGL works: leave everything alone
	}
	e.relaunch("-" + MesaFallbackFlagName) // does not return if the relaunch succeeds
}

// status reports whether this process runs on the Mesa fallback, with the pinned Mesa version when known.
func (e mfEnv) status() (active bool, version, driver string) {
	driver = e.getenv("GALLIUM_DRIVER")
	if driver != mesaGalliumDriverFast && driver != mesaGalliumDriverSafe {
		return false, "", ""
	}
	dir, err := e.exeDir()
	if err != nil {
		return true, "", driver
	}
	b, err := os.ReadFile(filepath.Join(dir, mesaFallbackDirName, "MESA_VERSION"))
	if err != nil {
		return true, "", driver
	}
	return true, strings.TrimSpace(string(b)), driver
}
