package startup

// MesaFallbackFlagName is the internal CLI flag main.go registers and appends
// when relaunching itself after a failed Windows hardware OpenGL probe. See
// EnsureWindowsOpenGLReady for the full mechanism; a no-op on other platforms.
const MesaFallbackFlagName = "mesa-gl-fallback"
