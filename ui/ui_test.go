package ui

import (
	"testing"

	"github.com/davecheney/corewar/mars"
	"github.com/davecheney/corewar/redcode"
	"github.com/davecheney/corewar/warriors"
)

func newGame(t testing.TB) *Game {
	t.Helper()
	cfg := mars.DefaultConfig()
	// Built-ins that fail to assemble, such as those too long, are skipped.
	ws, _ := warriors.Load(cfg)
	if len(ws) < 2 {
		t.Fatalf("only %d built-in warriors", len(ws))
	}
	return New(ws, cfg, 42)
}

// Incremental drawing must leave the screen exactly as the cell state says.
func TestIncrementalDrawMatchesState(t *testing.T) {
	g := newGame(t)
	var buf Buffer
	for frame := range 3000 {
		g.Update(0)
		g.Draw(&buf)
		if frame%250 != 0 {
			continue
		}
		for addr := range CoreCells {
			x := addr % CoreCols * CellWidth
			y := StatusHeight + addr/CoreCols*CellHeight
			want := g.core.drawn[addr]
			if !g.core.inList[addr] && g.core.owner[addr] != 0 {
				// Settled cells show their owner's terminal colour.
				want = g.core.color(addr)
			}
			for dy := range CellHeight {
				for dx := range CellWidth {
					if got := buf[(y+dy)*ScreenWidth+x+dx]; got != want {
						t.Fatalf("frame %d cell %d: pixel %d, want %d", frame, addr, got, want)
					}
				}
			}
		}
	}
}

func TestSettledCellsLeaveDecayList(t *testing.T) {
	cfg := mars.DefaultConfig()
	w, err := redcode.Assemble("JMP 0", cfg)
	if err != nil {
		t.Fatal(err)
	}
	g := New([]*mars.Warrior{w, w}, cfg, 1)
	var buf Buffer
	for range maxAge + 2 {
		g.Update(0)
		g.Draw(&buf)
	}
	// Two JMP 0s are executed every frame; nothing else changes.
	if g.core.n != 2 {
		t.Fatalf("decay list has %d cells, want 2", g.core.n)
	}
}

func TestButtons(t *testing.T) {
	g := newGame(t)
	g.Update(ButtonFaster)
	g.Update(ButtonFaster) // held, not pressed again
	if g.Speed() != Speeds[DefaultSpeed+1] {
		t.Fatalf("speed = %d", g.Speed())
	}
	g.Update(ButtonSlower)
	g.Update(0)
	g.Update(ButtonPause)
	if !g.Paused() {
		t.Fatal("not paused")
	}
	cycle := g.m.Cycle
	g.Update(0)
	if g.m.Cycle != cycle {
		t.Fatal("ran while paused")
	}
	g.Update(ButtonSkip)
	if g.Paused() || g.round != 1 || g.m.Cycle != 0 {
		t.Fatal("skip did not start a new match")
	}
}

func TestReplace(t *testing.T) {
	g := newGame(t)
	for range 50 {
		for side, b := range [2]Buttons{ButtonNewLeft, ButtonNewRight} {
			before := g.pair
			g.Update(b)
			if g.round != 1 || g.m.Cycle != 0 {
				t.Fatal("replacement did not start a new match")
			}
			g.Update(0)
			if g.pair[1-side] != before[1-side] {
				t.Fatalf("side %d: other warrior changed %v -> %v", side, before, g.pair)
			}
			if g.pair[side] == before[side] || g.pair[side] == g.pair[1-side] {
				t.Fatalf("side %d: pair %v -> %v", side, before, g.pair)
			}
		}
	}
}

func TestWinnerSurvives(t *testing.T) {
	g := newGame(t)
	for side := range 2 {
		g.state, g.timer, g.winner = matchOver, 1, side
		before := g.pair
		g.Update(0)
		if g.pair[side] != before[side] {
			t.Fatalf("winner on side %d replaced: %v -> %v", side, before, g.pair)
		}
		if g.pair[1-side] == before[1-side] {
			t.Fatalf("loser on side %d kept: %v -> %v", 1-side, before, g.pair)
		}
		if g.state != running || g.round != 1 {
			t.Fatal("next match did not start")
		}
	}
}

func TestMatchesProgress(t *testing.T) {
	g := newGame(t)
	var buf Buffer
	matches := 0
	for range 200000 {
		before := g.state
		g.Update(0)
		g.Draw(&buf)
		if before == matchOver && g.state == running {
			matches++
			if matches == 3 {
				return
			}
		}
	}
	t.Fatalf("only %d matches completed", matches)
}

func TestFrameDoesNotAllocate(t *testing.T) {
	g := newGame(t)
	var buf Buffer
	allocs := testing.AllocsPerRun(500, func() {
		g.Update(0)
		g.Draw(&buf)
	})
	if allocs != 0 {
		t.Fatalf("%v allocations per frame", allocs)
	}
}

func TestPalette(t *testing.T) {
	teal, red := Palette[ColorTeal], Palette[ColorRed]
	if teal.G <= teal.R || teal.B <= teal.R {
		t.Errorf("teal = %+v", teal)
	}
	if red.R <= red.G || red.G != red.B {
		t.Errorf("red = %+v", red)
	}
	// Same saturation and lightness: the extremes of each sum match.
	if int(max(teal.R, teal.G, teal.B))+int(min(teal.R, teal.G, teal.B)) !=
		int(max(red.R, red.G, red.B))+int(min(red.R, red.G, red.B)) {
		t.Errorf("teal %+v and red %+v differ in lightness", teal, red)
	}
}

// lazyRoster assembles on demand, counting assemblies; "bad" fails.
type lazyRoster struct {
	srcs  []string
	calls int
}

func (r *lazyRoster) Len() int { return len(r.srcs) }

func (r *lazyRoster) Warrior(i int) (*mars.Warrior, error) {
	r.calls++
	return redcode.Assemble(r.srcs[i], mars.DefaultConfig())
}

func TestLazyRoster(t *testing.T) {
	r := &lazyRoster{srcs: []string{";name A\nJMP 0\n", "bad", ";name B\nJMP 0\n", ";name C\nJMP 0\n"}}
	g := NewGame(mars.DefaultConfig(), 1)
	var warned int
	g.Warn = func(error) { warned++ }
	g.Start(r)
	for range 100 {
		g.Update(ButtonNewLeft)
		g.Update(ButtonNewRight)
		for _, w := range g.fighters {
			if w == nil {
				t.Fatal("no warrior fighting")
			}
		}
		if g.fighters[0] == g.fighters[1] || g.pair[0] == 1 || g.pair[1] == 1 {
			t.Fatalf("pair %v", g.pair)
		}
	}
	if warned != 1 {
		t.Errorf("warned %d times, want 1", warned)
	}
	// Only the pairs picked are assembled: 2 to start, then 1 per swap,
	// plus the single failed attempt.
	if want := 2 + 200 + 1; r.calls != want {
		t.Errorf("assembled %d times, want %d", r.calls, want)
	}
}
