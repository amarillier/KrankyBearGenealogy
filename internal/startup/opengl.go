// Package startup implements the Windows Mesa3D software-OpenGL fallback,
// ported from ../TaniumMigrator (also used in ../KrankyBearCommander): some
// VMs and locked-down hosts have no usable hardware OpenGL, which otherwise
// crashes or hangs Fyne's GLFW driver with no useful diagnostic.
package startup

import (
	"fmt"

	"github.com/go-gl/glfw/v3.3/glfw"
)

// probeOpenGL performs a real OpenGL capability check: it creates a hidden,
// unfocused window with an OpenGL context using the exact GLFW call Fyne's
// desktop driver makes when the real window is shown (glfw.CreateWindow).
// If this fails here, the main window would fail the same way later.
//
// Must run before fyne.io/fyne/v2/app.New()/NewWithID(): both this probe and
// Fyne's driver use github.com/go-gl/glfw/v3.3/glfw, and GLFW only supports
// one Init/Terminate cycle at a time per process. It must also run on the
// main goroutine — glfw's Fyne driver package pins the main goroutine to its
// OS thread in an init() func, and importing fyne.io/fyne/v2/app (as this
// program does) triggers that init() before main() runs, so by the time this
// is called we're already on the right thread.
func probeOpenGL() error {
	if err := glfw.Init(); err != nil {
		return fmt.Errorf("GLFW failed to initialize: %w", err)
	}
	defer glfw.Terminate()

	glfw.WindowHint(glfw.Visible, glfw.False)
	glfw.WindowHint(glfw.Focused, glfw.False)
	win, err := glfw.CreateWindow(64, 64, "opengl-capability-check", nil, nil)
	if err != nil {
		return fmt.Errorf("failed to create an OpenGL context: %w", err)
	}
	win.Destroy()

	return nil
}
