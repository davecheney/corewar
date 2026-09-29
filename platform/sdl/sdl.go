// Package sdl presents the demo in a desktop window through SDL3.
//
// The backend is ported from github.com/davecheney/tiny64's
// cmd/internal/desktop. It is a hand-rolled cgo shim over the dozen SDL
// calls it needs rather than a binding module, so that TinyGo's cgo can
// build it too. The constraints that shaped it:
//
//   - "#cgo pkg-config:" and per-GOOS #cgo lines are rejected by TinyGo, so
//     the flags are spelled out once for Homebrew and Linux alike;
//     nonexistent -I and -L directories are ignored.
//   - The include path names SDL3's own directory so <SDL.h> resolves
//     without putting /usr/include ahead of TinyGo's clang headers.
//   - SDL_DISABLE_ARM_NEON_H keeps <arm_neon.h>, which TinyGo lacks, out.
//
// The window shows an INDEX8 streaming texture with SDL_SetTexturePalette,
// so the ui.Buffer goes to the GPU as the one byte per pixel it already is.
package sdl

// #cgo CFLAGS: -I/opt/homebrew/include/SDL3 -I/usr/local/include/SDL3 -I/usr/include/SDL3 -I/opt/homebrew/include -I/usr/local/include -D_THREAD_SAFE -DSDL_DISABLE_ARM_NEON_H
// #cgo LDFLAGS: -L/opt/homebrew/lib -L/usr/local/lib -L/usr/lib -L/usr/lib/x86_64-linux-gnu -L/usr/lib/aarch64-linux-gnu -lSDL3
// #define SDL_MAIN_HANDLED
// #include <SDL.h>
// #include <SDL_main.h>
//
// static SDL_WindowFlags corewarWindowFlags(void) {
//     return SDL_WINDOW_RESIZABLE | SDL_WINDOW_HIGH_PIXEL_DENSITY;
// }
import "C"

import (
	"fmt"
	"runtime"
	"unsafe"

	"github.com/davecheney/corewar/ui"
)

// Game is what the window drives once per frame.
type Game interface {
	Update(ui.Buttons)
	Draw(ui.Display)
}

// Scale is the smallest the window opens at, in window pixels per screen
// pixel.
const Scale = 2

// sdlEventSize is sizeof(SDL_Event). Events are polled into a byte buffer
// rather than a C.SDL_Event so the union never needs a Go type; the arrays
// below fail the build if the size is wrong.
const sdlEventSize = 128

var (
	_ [sdlEventSize - unsafe.Sizeof(C.SDL_Event{})]byte
	_ [unsafe.Sizeof(C.SDL_Event{}) - sdlEventSize]byte
)

var sdl struct {
	window   *C.SDL_Window
	renderer *C.SDL_Renderer
	frame    *C.SDL_Texture
	palette  *C.SDL_Palette
}

// cString lays a Go string out as NUL-terminated bytes. SDL copies every
// string it is given here, so Go memory that outlives the call is enough.
func cString(s string) []byte {
	return append([]byte(s), 0)
}

func sdlError(what string) error {
	return fmt.Errorf("%s: %s", what, C.GoString(C.SDL_GetError()))
}

// Run opens a window titled title and drives g until the window is closed,
// Escape or Q is pressed, or the process is interrupted.
func Run(title string, g Game) error {
	closeDisplay, err := openDisplay(title)
	if err != nil {
		return err
	}
	defer closeDisplay()

	var buf ui.Buffer
	for {
		if pumpEvents() {
			return nil
		}
		buttons, quit := pollKeyboard()
		if quit {
			return nil
		}
		g.Update(buttons)
		g.Draw(&buf)
		if err := present(&buf); err != nil {
			return err
		}
	}
}

// openingScale picks the largest whole multiple of the screen that fits in
// four fifths of the usable desktop, and never less than Scale.
func openingScale() int {
	var usable C.SDL_Rect
	if !C.SDL_GetDisplayUsableBounds(C.SDL_GetPrimaryDisplay(), &usable) {
		return Scale
	}
	fit := min(
		int(usable.w)*4/5/ui.ScreenWidth,
		int(usable.h)*4/5/ui.ScreenHeight,
	)
	return max(fit, Scale)
}

func makePalette() (*C.SDL_Palette, error) {
	palette := C.SDL_CreatePalette(C.int(len(ui.Palette)))
	if palette == nil {
		return nil, sdlError("SDL_CreatePalette")
	}
	colors := make([]C.SDL_Color, len(ui.Palette))
	for i, c := range ui.Palette {
		colors[i] = C.SDL_Color{r: C.Uint8(c.R), g: C.Uint8(c.G), b: C.Uint8(c.B), a: 0xff}
	}
	if !C.SDL_SetPaletteColors(palette, &colors[0], 0, C.int(len(colors))) {
		C.SDL_DestroyPalette(palette)
		return nil, sdlError("SDL_SetPaletteColors")
	}
	return palette, nil
}

func openDisplay(title string) (func(), error) {
	// SDL wants its window and event calls on the main thread on macOS.
	runtime.LockOSThread()

	C.SDL_SetMainReady()

	// SDL3 catches SIGINT and SIGTERM but does not turn them into a quit
	// event, so leave signals to signal.go.
	hint, on := cString(C.SDL_HINT_NO_SIGNAL_HANDLERS), cString("1")
	C.SDL_SetHint((*C.char)(unsafe.Pointer(&hint[0])), (*C.char)(unsafe.Pointer(&on[0])))
	notifyQuitSignals()

	if !C.SDL_Init(C.SDL_INIT_VIDEO) {
		return nil, sdlError("SDL_Init")
	}

	closeDisplay := func() {
		C.SDL_DestroyTexture(sdl.frame)
		C.SDL_DestroyPalette(sdl.palette)
		C.SDL_DestroyRenderer(sdl.renderer)
		C.SDL_DestroyWindow(sdl.window)
		C.SDL_Quit()
	}

	cTitle := cString(title)
	scale := openingScale()
	sdl.window = C.SDL_CreateWindow((*C.char)(unsafe.Pointer(&cTitle[0])),
		C.int(ui.ScreenWidth*scale), C.int(ui.ScreenHeight*scale),
		C.corewarWindowFlags())
	if sdl.window == nil {
		closeDisplay()
		return nil, sdlError("SDL_CreateWindow")
	}
	C.SDL_SetWindowMinimumSize(sdl.window, ui.ScreenWidth, ui.ScreenHeight)

	sdl.renderer = C.SDL_CreateRenderer(sdl.window, nil)
	if sdl.renderer == nil {
		closeDisplay()
		return nil, sdlError("SDL_CreateRenderer")
	}
	// Vsync paces the loop.
	if !C.SDL_SetRenderVSync(sdl.renderer, 1) {
		closeDisplay()
		return nil, sdlError("SDL_SetRenderVSync")
	}
	if !C.SDL_SetRenderLogicalPresentation(sdl.renderer,
		ui.ScreenWidth, ui.ScreenHeight, C.SDL_LOGICAL_PRESENTATION_LETTERBOX) {
		closeDisplay()
		return nil, sdlError("SDL_SetRenderLogicalPresentation")
	}

	palette, err := makePalette()
	if err != nil {
		closeDisplay()
		return nil, err
	}
	sdl.palette = palette

	sdl.frame = C.SDL_CreateTexture(sdl.renderer,
		C.SDL_PIXELFORMAT_INDEX8, C.SDL_TEXTUREACCESS_STREAMING,
		ui.ScreenWidth, ui.ScreenHeight)
	if sdl.frame == nil {
		closeDisplay()
		return nil, sdlError("SDL_CreateTexture")
	}
	if !C.SDL_SetTexturePalette(sdl.frame, sdl.palette) {
		closeDisplay()
		return nil, sdlError("SDL_SetTexturePalette")
	}
	if !C.SDL_SetTextureScaleMode(sdl.frame, C.SDL_SCALEMODE_PIXELART) {
		closeDisplay()
		return nil, sdlError("SDL_SetTextureScaleMode")
	}
	C.SDL_SetTextureBlendMode(sdl.frame, C.SDL_BLENDMODE_NONE)

	return closeDisplay, nil
}

// pumpEvents drains SDL's event queue, reporting whether the user asked to
// quit. It must run every frame for the keyboard state to update.
func pumpEvents() bool {
	if quitSignalled() {
		return true
	}
	var event [sdlEventSize]byte
	for C.SDL_PollEvent((*C.SDL_Event)(unsafe.Pointer(&event[0]))) {
		// The type tag is the first Uint32 of every event in the union.
		switch *(*uint32)(unsafe.Pointer(&event[0])) {
		case C.SDL_EVENT_QUIT, C.SDL_EVENT_WINDOW_CLOSE_REQUESTED:
			return true
		}
	}
	return false
}

// keymap maps SDL scancodes to demo buttons.
var keymap = []struct {
	scancode C.SDL_Scancode
	button   ui.Buttons
}{
	{C.SDL_SCANCODE_N, ui.ButtonSkip},
	{C.SDL_SCANCODE_RETURN, ui.ButtonSkip},
	{C.SDL_SCANCODE_SPACE, ui.ButtonPause},
	{C.SDL_SCANCODE_P, ui.ButtonPause},
	{C.SDL_SCANCODE_EQUALS, ui.ButtonFaster},
	{C.SDL_SCANCODE_KP_PLUS, ui.ButtonFaster},
	{C.SDL_SCANCODE_UP, ui.ButtonFaster},
	{C.SDL_SCANCODE_RIGHT, ui.ButtonNewRight},
	{C.SDL_SCANCODE_MINUS, ui.ButtonSlower},
	{C.SDL_SCANCODE_KP_MINUS, ui.ButtonSlower},
	{C.SDL_SCANCODE_DOWN, ui.ButtonSlower},
	{C.SDL_SCANCODE_LEFT, ui.ButtonNewLeft},
}

// pollKeyboard samples the whole keyboard once per frame.
func pollKeyboard() (ui.Buttons, bool) {
	var n C.int
	held := unsafe.Slice((*byte)(unsafe.Pointer(C.SDL_GetKeyboardState(&n))), int(n))
	if held[C.SDL_SCANCODE_ESCAPE] != 0 || held[C.SDL_SCANCODE_Q] != 0 {
		return 0, true
	}
	var b ui.Buttons
	for _, k := range keymap {
		if held[k.scancode] != 0 {
			b |= k.button
		}
	}
	return b, false
}

// present uploads buf into the paletted texture and puts it on screen.
func present(buf *ui.Buffer) error {
	var raw unsafe.Pointer
	var pitch C.int
	if !C.SDL_LockTexture(sdl.frame, nil, &raw, &pitch) {
		return sdlError("SDL_LockTexture")
	}
	dst := unsafe.Slice((*byte)(raw), int(pitch)*ui.ScreenHeight)
	src := unsafe.Slice((*byte)(unsafe.Pointer(&buf[0])), len(buf))
	if int(pitch) == ui.ScreenWidth {
		copy(dst, src)
	} else {
		for y := range ui.ScreenHeight {
			copy(dst[y*int(pitch):], src[y*ui.ScreenWidth:(y+1)*ui.ScreenWidth])
		}
	}
	C.SDL_UnlockTexture(sdl.frame)

	if !C.SDL_RenderClear(sdl.renderer) {
		return sdlError("SDL_RenderClear")
	}
	if !C.SDL_RenderTexture(sdl.renderer, sdl.frame, nil, nil) {
		return sdlError("SDL_RenderTexture")
	}
	C.SDL_RenderPresent(sdl.renderer)
	return nil
}
