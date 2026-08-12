//go:build windows

package startup

import (
	"os"
	"os/exec"
	"path/filepath"
)

// mesaFallbackDirName is the installer-staged subfolder (see the project's
// Inno .iss) holding Mesa3D's software OpenGL DLLs (opengl32.dll +
// libgallium_wgl.dll). It is kept out of the exe's own directory so ordinary
// launches keep using the real system OpenGL. Only on a confirmed
// hardware-probe failure does moveMesaFallbackIntoAppDir move the two files
// out of here and into the app's own directory, where Windows' default DLL
// search order picks them up.
//
// windows.SetDllDirectory was tried first (in ../TaniumMigrator, where this
// mechanism originates) as a way to avoid ever touching files in the
// (possibly non-admin-writable) install directory, but proved unreliable in
// the field — GLFW's plain LoadLibraryA("opengl32.dll") kept resolving to
// the real system DLL even with SetDllDirectory pointed at this folder, on
// a host where moving the DLLs into the app dir by hand fixed it
// immediately. Moving the files is the mechanism actually proven to work.
const mesaFallbackDirName = "mesa-fallback"

// mesaForceMarkerFileName, if present inside mesaFallbackDirName, skips the
// hardware probe and relaunch entirely and activates the fallback directly.
// For a host already confirmed to need Mesa (e.g. a golden VM template
// reused to spin up many identical no-GPU VMs), this saves the one-time
// probe+relaunch cost on every single launch. Create it with e.g. (from a
// user PowerShell, no admin needed since it's just a file in the app's own
// install dir):
//
//	New-Item "C:\Program Files\<App>\mesa-fallback\.force-mesa-fallback" -ItemType File
const mesaForceMarkerFileName = ".force-mesa-fallback"

// mesaFallbackFiles are the Mesa3D files moved from mesaFallbackDirName into
// the app's own directory once a hardware OpenGL probe has failed.
var mesaFallbackFiles = []string{"opengl32.dll", "libgallium_wgl.dll"}

// EnsureWindowsOpenGLReady implements the Mesa3D software-OpenGL fallback:
// some VMs and locked-down hosts have no usable hardware OpenGL, which
// otherwise crashes or hangs Fyne's GLFW driver with no useful diagnostic.
//
// If a prior launch (or this one) already moved the Mesa DLLs into the
// app's own directory — see moveMesaFallbackIntoAppDir — this skips
// straight to forcing the safe rasterizer with no probing at all:
// Windows' default DLL search order will find them next to the exe on its
// own.
//
// alreadyUsingFallback is true when this process was relaunched by a prior
// call to this function (main.go passes -mesa-gl-fallback back to itself)
// after the hardware probe below failed. In that case — and likewise if
// mesaForceMarkerFileName exists — the Mesa DLLs are moved into the app
// directory unconditionally with no further probing, so a persistently
// broken host can't loop forever.
//
// Otherwise, this probes real hardware OpenGL first (preferring it —
// software rendering is noticeably slower for interactive use, and leaving
// the app directory untouched means a later driver fix is recovered from by
// simply deleting the two moved files, resuming hardware detection on the
// next launch). On failure it relaunches itself with the fallback flag and
// exits; Windows only resolves a DLL's search path once per process, so
// switching opengl32.dll implementations requires a fresh process, not just
// a file move.
//
// Must run before fyne.io/fyne/v2/app.New()/NewWithID(): both this probe and
// Fyne's driver use a single GLFW Init/Terminate cycle per process (see
// probeOpenGL's doc comment).
func EnsureWindowsOpenGLReady(alreadyUsingFallback bool) {
	if mesaFallbackAlreadyDeployed() {
		os.Setenv("GALLIUM_DRIVER", mesaGalliumDriver)
		return
	}

	if alreadyUsingFallback || mesaForceMarkerExists() {
		moveMesaFallbackIntoAppDir()
		// Relaunch once more even though the files are now in place, rather
		// than continuing in this same process: in testing (see
		// ../TaniumMigrator, where this mechanism originates), a process
		// that had just moved the DLLs itself still failed its own
		// immediate probe, while a brand-new process started after the
		// move succeeded every time (not fully root-caused, but
		// empirically reliable). Only falls through to continuing in this
		// process if the relaunch itself can't even start.
		relaunchSelf("-" + MesaFallbackFlagName)
		os.Setenv("GALLIUM_DRIVER", mesaGalliumDriver)
		return
	}

	if probeOpenGL() == nil {
		return // real hardware OpenGL works — nothing to do, keep using it
	}

	relaunchSelf("-" + MesaFallbackFlagName) // does not return if the relaunch succeeds
}

// mesaGalliumDriver forces Mesa's non-JIT reference rasterizer. llvmpipe
// (Mesa's default Gallium driver) JIT-compiles its rasterizer per detected
// CPU features, and hangs after the first frame under some hypervisors'
// conservative default vCPU models (confirmed on Proxmox's kvm64/qemu64).
// softpipe is slower, but immune to this. Harmless if a real GPU driver ends
// up loaded instead: that driver ignores this Mesa-only env var.
const mesaGalliumDriver = "softpipe"

// mesaForceMarkerExists reports whether mesaForceMarkerFileName is present
// next to the bundled Mesa DLLs. Best-effort: any error (no exe path, no
// such file) is treated as "not forced," falling through to the normal
// probe.
func mesaForceMarkerExists() bool {
	exePath, err := os.Executable()
	if err != nil {
		return false
	}
	_, err = os.Stat(filepath.Join(filepath.Dir(exePath), mesaFallbackDirName, mesaForceMarkerFileName))
	return err == nil
}

// mesaFallbackAlreadyDeployed reports whether the Mesa DLLs already sit in
// the app's own directory — from a prior launch's
// moveMesaFallbackIntoAppDir, or an operator having placed them there
// manually. Checking just the first file is enough:
// moveMesaFallbackIntoAppDir moves both together.
func mesaFallbackAlreadyDeployed() bool {
	exePath, err := os.Executable()
	if err != nil {
		return false
	}
	_, err = os.Stat(filepath.Join(filepath.Dir(exePath), mesaFallbackFiles[0]))
	return err == nil
}

// moveMesaFallbackIntoAppDir moves the Mesa DLLs out of mesaFallbackDirName
// and into the app's own directory, where Windows' default DLL search order
// (app directory before System32) picks them up with no further
// configuration. Best-effort: if this fails (e.g. no write access to the
// install directory), the DLLs simply stay in mesaFallbackDirName, unused,
// and the probe that runs right after this in main.go fails again with the
// normal, actionable fatal message — the correct degradation for a
// locked-down host where an operator needs to intervene (e.g. a separate
// elevated helper, or manually copying the two files themselves).
func moveMesaFallbackIntoAppDir() {
	exePath, err := os.Executable()
	if err != nil {
		return
	}
	appDir := filepath.Dir(exePath)
	fallbackDir := filepath.Join(appDir, mesaFallbackDirName)
	for _, name := range mesaFallbackFiles {
		dst := filepath.Join(appDir, name)
		if _, err := os.Stat(dst); err == nil {
			continue // already moved (e.g. a partially-completed prior attempt)
		}
		_ = os.Rename(filepath.Join(fallbackDir, name), dst)
	}
}

// relaunchSelf spawns a fresh copy of this process with extraArgs appended
// to the current arguments, and exits this process if the spawn succeeds —
// the new process picks up wherever this one left off (e.g.
// mesaFallbackAlreadyDeployed finding the Mesa DLLs this process just moved
// into place). If the spawn can't even start (os.Executable fails, or
// exec.Cmd.Start fails), this returns instead of exiting, so the caller can
// fall back to continuing in this same process as a last resort.
func relaunchSelf(extraArgs ...string) {
	exePath, err := os.Executable()
	if err != nil {
		return // can't relaunch; caller falls back to continuing in this process
	}
	args := append(append([]string{}, os.Args[1:]...), extraArgs...)
	cmd := exec.Command(exePath, args...)
	cmd.Stdout, cmd.Stderr, cmd.Stdin = os.Stdout, os.Stderr, os.Stdin
	if err := cmd.Start(); err != nil {
		return // failed to relaunch; caller falls back to continuing in this process
	}
	os.Exit(0)
}
