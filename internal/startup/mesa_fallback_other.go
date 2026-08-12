//go:build !windows

package startup

// EnsureWindowsOpenGLReady is a no-op on non-Windows platforms; the Mesa
// software-OpenGL fallback (see mesa_fallback_windows.go) only applies to
// Windows installs.
func EnsureWindowsOpenGLReady(alreadyUsingFallback bool) {}
