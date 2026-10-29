package startup

// MesaFallbackFlagName is the internal CLI flag main.go registers and appends
// when relaunching itself after a failed Windows hardware OpenGL probe. See
// EnsureWindowsOpenGLReady for the full mechanism; a no-op on other platforms.
const MesaFallbackFlagName = "mesa-gl-fallback"

// MesaCompatFlagName forces the Mesa fallback's safe, non-JIT softpipe
// rasterizer instead of the default llvmpipe — for a host known to hit the
// JIT hang llvmpipe can trigger on some hypervisors' conservative default
// vCPU models. See chooseGalliumDriver in mesa_fallback_logic.go. The flag is
// registered on every platform (so a cross-platform script does not error out)
// but only acts on Windows.
const MesaCompatFlagName = "mesa-gl-compat"

// MesaPerformanceFlagName forces the fast llvmpipe rasterizer even on a host
// the automatic CPU check would otherwise treat as a conservative/compat vCPU
// model — for an operator who has confirmed their specific host does not hit
// the hang. See chooseGalliumDriver in mesa_fallback_logic.go.
const MesaPerformanceFlagName = "mesa-gl-performance"
