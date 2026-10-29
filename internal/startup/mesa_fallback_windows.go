//go:build windows

package startup

import (
	"os"
	"os/exec"
	"path/filepath"

	"golang.org/x/sys/cpu"
)

func realMesaEnv() mfEnv {
	return mfEnv{
		exeDir: func() (string, error) {
			p, err := os.Executable()
			if err != nil {
				return "", err
			}
			return filepath.Dir(p), nil
		},
		hasSSE41: func() bool { return cpu.X86.HasSSE41 },
		probe:    probeOpenGL,
		relaunch: relaunchSelf,
		getenv:   os.Getenv,
		setenv:   func(k, v string) { os.Setenv(k, v) },
	}
}

// EnsureWindowsOpenGLReady implements the Mesa3D software-OpenGL fallback: some VMs and locked-down hosts have no usable
// hardware OpenGL, which otherwise crashes or hangs Fyne's GLFW driver with no useful diagnostic.
//
// It prefers real hardware OpenGL: software rendering is noticeably slower for interactive use, and leaving the app directory
// untouched means a later driver fix is recovered from by simply deleting the two moved files, which resumes hardware
// detection on the next launch. If the hardware probe fails it relaunches itself with -mesa-gl-fallback and exits; Windows
// resolves a DLL's search path once per process, so switching opengl32.dll implementations needs a fresh process.
//
// alreadyUsingFallback is true when this process is that relaunch (main.go passes -mesa-gl-fallback back to itself). In that
// case, and likewise if the .force-mesa-fallback marker exists, the Mesa DLLs are moved into the app directory unconditionally
// with no further probing, so a persistently broken host cannot loop forever. If the DLLs are already in the app directory
// (an earlier launch moved them, or an operator did) it goes straight to choosing the rasterizer.
//
// The rasterizer is llvmpipe (fast) unless the CPU looks like a conservative hypervisor model without SSE4.1, or the operator
// asked for softpipe: see chooseGalliumDriver. forceCompat and forcePerformance come from the -mesa-gl-compat and
// -mesa-gl-performance flags and override that automatic pick in either direction.
//
// Must run before fyne.io/fyne/v2/app.NewWithID and before anything else that initialises GLFW: this probe and Fyne's
// driver share one GLFW Init/Terminate cycle per process (see probeOpenGL).
func EnsureWindowsOpenGLReady(alreadyUsingFallback, forceCompat, forcePerformance bool) {
	realMesaEnv().ensure(alreadyUsingFallback, forceCompat, forcePerformance)
}

// MesaFallbackActive reports whether this process is using the Mesa3D software OpenGL fallback rather than real hardware
// OpenGL, with the pinned Mesa version from mesa-fallback/MESA_VERSION ("" if unreadable) and the rasterizer in use. It is a
// best-effort check of the GALLIUM_DRIVER variable the fallback sets, not a query of the GL context.
func MesaFallbackActive() (active bool, version string, driver string) { return realMesaEnv().status() }

// relaunchSelf spawns a fresh copy of this process with extraArgs appended and exits this one if the spawn succeeds. If the
// copy cannot even start it returns, so the caller can fall back to continuing in this process as a last resort.
func relaunchSelf(extraArgs ...string) {
	exePath, err := os.Executable()
	if err != nil {
		return
	}
	args := append(append([]string{}, os.Args[1:]...), extraArgs...)
	cmd := exec.Command(exePath, args...)
	cmd.Stdout, cmd.Stderr, cmd.Stdin = os.Stdout, os.Stderr, os.Stdin
	if err := cmd.Start(); err != nil {
		return
	}
	os.Exit(0)
}
