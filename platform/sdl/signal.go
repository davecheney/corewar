package sdl

import (
	"os"
	"os/signal"
	"syscall"
)

// SDL3 installs handlers for SIGINT and SIGTERM but no longer posts a quit
// event, so the hint in openDisplay turns them off and these take over.
// The signal only sets a flag; pumpEvents reads it at the top of the next
// frame so shutdown runs on the main thread through the normal teardown.

var quit = make(chan os.Signal, 1)

func notifyQuitSignals() {
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
}

func quitSignalled() bool {
	select {
	case <-quit:
		return true
	default:
		return false
	}
}
