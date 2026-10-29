//go:build !windows

package startup

// EnsureWindowsOpenGLReady is a no-op on non-Windows platforms; the Mesa software-OpenGL fallback (see
// mesa_fallback_windows.go and mesa_fallback_logic.go) only applies to Windows installs.
func EnsureWindowsOpenGLReady(alreadyUsingFallback, forceCompat, forcePerformance bool) {}

// MesaFallbackActive always reports inactive on non-Windows platforms.
func MesaFallbackActive() (active bool, version string, driver string) { return false, "", "" }
