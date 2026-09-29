package ui

import (
	"math/rand/v2"

	"github.com/davecheney/corewar/mars"
)

// Buttons is the set of inputs held down this frame.
type Buttons uint8

const (
	ButtonSkip     Buttons = 1 << iota // abandon this match and start another
	ButtonFaster                       // more MARS steps per frame
	ButtonSlower                       // fewer MARS steps per frame
	ButtonPause                        // toggle pause
	ButtonNewLeft                      // replace the left (teal) warrior
	ButtonNewRight                     // replace the right (red) warrior
)

// Speeds are the MARS steps (single instructions) run per frame.
var Speeds = []int{1, 2, 5, 10, 20, 50, 100, 200, 500, 1000, 2000, 5000}

// DefaultSpeed indexes Speeds.
const DefaultSpeed = 4 // 20 MARS instructions per displayed frame

// Match settings.
const (
	RoundsPerMatch = 3
	winsNeeded     = RoundsPerMatch/2 + 1
	roundPause     = 120 // frames the round result stays up
	matchPause     = 240 // frames the match result stays up
)

type state uint8

const (
	running state = iota
	roundOver
	matchOver
)

// Roster is the set of warriors a game picks from. Warrior may assemble
// on demand; a warrior that fails is left out for the rest of the game.
type Roster interface {
	Len() int
	Warrior(i int) (*mars.Warrior, error)
}

// Warriors is a Roster of warriors already assembled.
type Warriors []*mars.Warrior

func (ws Warriors) Len() int                             { return len(ws) }
func (ws Warriors) Warrior(i int) (*mars.Warrior, error) { return ws[i], nil }

// Game is the attract-mode demo: random pairs of warriors fight best of
// three, forever. The winner of a match stays on and a random challenger
// replaces the loser; after a drawn match both are replaced.
type Game struct {
	// Warn, if set, is called with the error for each warrior that fails
	// to assemble and is left out.
	Warn func(error)

	roster Roster
	bad    []bool // warriors that failed to assemble
	nbad   int
	m      *mars.MARS
	rng    *rand.Rand

	pair     [2]int // indexes into roster; pair[0] is teal, pair[1] red
	fighters [2]*mars.Warrior
	wins     [2]int
	round    int
	winner   int // of the last round or match, -1 for a tie

	state  state
	timer  int
	speed  int
	paused bool
	prev   Buttons

	core coreView

	redrawAll bool
	names     [2]field
	scores    [2]field
	procs     [2]field
	title     field
	info      field
	message   field
}

// New creates a game over the given warriors, which must number at least
// two. cfg.CoreSize must equal CoreCells for the core to fill the screen.
func New(warriors []*mars.Warrior, cfg mars.Config, seed uint64) *Game {
	g := NewGame(cfg, seed)
	g.Start(Warriors(warriors))
	return g
}

// NewGame allocates a game without starting it. On a small heap, call it
// before assembling warriors so its large buffers are allocated first,
// then call Start.
func NewGame(cfg mars.Config, seed uint64) *Game {
	g := &Game{
		m:         mars.New(cfg, 2),
		rng:       rand.New(rand.NewPCG(seed, seed^0x9e3779b97f4a7c15)),
		speed:     DefaultSpeed,
		redrawAll: true,
	}
	g.m.Touch = g.core.touch

	const (
		x0     = (ScreenWidth - 45*glyphWidth) / 2
		side   = 16
		center = 13
		xc     = x0 + side*glyphWidth
		xr     = xc + center*glyphWidth
	)
	for row, fs := range [][3]*field{
		{&g.names[0], &g.title, &g.names[1]},
		{&g.scores[0], &g.info, &g.scores[1]},
		{&g.procs[0], &g.message, &g.procs[1]},
	} {
		y := row * glyphHeight
		*fs[0] = field{x: x0, y: y, width: side, align: alignLeft}
		*fs[1] = field{x: xc, y: y, width: center, align: alignCenter}
		*fs[2] = field{x: xr, y: y, width: side, align: alignRight}
	}

	return g
}

// Start begins the attract mode over r, at least two of whose warriors
// must assemble. Only the two warriors fighting are held assembled.
func (g *Game) Start(r Roster) {
	g.roster = r
	g.bad = make([]bool, r.Len())
	g.newPair()
}

// Speed returns the current MARS steps per frame.
func (g *Game) Speed() int { return Speeds[g.speed] }

// Paused reports whether the simulation is paused.
func (g *Game) Paused() bool { return g.paused }

// newPair starts a match between two random warriors.
func (g *Game) newPair() {
	g.fighters = [2]*mars.Warrior{} // let the old pair be collected first
	g.pair[0], g.fighters[0] = g.pick(-1, -1)
	g.pair[1], g.fighters[1] = g.pick(g.pair[0], -1)
	g.startMatch()
}

// replace starts a match with side's warrior swapped for a random one,
// different from both current warriors when the roster allows.
func (g *Game) replace(side int) {
	keep, old := g.pair[1-side], g.pair[side]
	g.fighters[side] = nil
	g.pair[side], g.fighters[side] = g.pick(keep, old)
	g.startMatch()
}

// pick assembles a random warrior other than keep, and other than avoid
// too unless it is the only alternative, and returns it with its index.
// Warriors that fail to assemble are reported and never picked again.
func (g *Game) pick(keep, avoid int) (int, *mars.Warrior) {
	for {
		usable := g.roster.Len() - g.nbad
		if usable < 2 {
			panic("ui: fewer than two warriors assemble")
		}
		n := g.rng.IntN(g.roster.Len())
		if g.bad[n] || n == keep || n == avoid && usable > 2 {
			continue
		}
		w, err := g.roster.Warrior(n)
		if err != nil {
			g.bad[n] = true
			g.nbad++
			if g.Warn != nil {
				g.Warn(err)
			}
			continue
		}
		return n, w
	}
}

func (g *Game) startMatch() {
	g.wins = [2]int{}
	g.round = 0
	g.m.ClearPSpace()
	g.startRound()
}

func (g *Game) startRound() {
	g.round++
	g.state = running
	g.core.reset()
	g.m.Reset()

	w0, w1 := g.fighters[0], g.fighters[1]
	size, gap := g.m.CoreSize, g.m.MinDistance
	at0 := g.rng.IntN(size)
	span := size - len(w0.Code) - len(w1.Code) - 2*gap
	at1 := at0 + len(w0.Code) + gap + g.rng.IntN(span+1)
	g.m.Load(w0, at0)
	g.m.Load(w1, at1%size)
}

// nextMatch keeps the winner of the match just played and brings on a
// challenger in place of the loser.
func (g *Game) nextMatch() {
	if g.winner < 0 {
		g.newPair()
		return
	}
	g.replace(1 - g.winner)
}

// Update advances the demo by one frame given the buttons held down.
func (g *Game) Update(held Buttons) {
	pressed := held &^ g.prev
	g.prev = held

	if pressed&ButtonPause != 0 {
		g.paused = !g.paused
	}
	if pressed&ButtonFaster != 0 && g.speed < len(Speeds)-1 {
		g.speed++
	}
	if pressed&ButtonSlower != 0 && g.speed > 0 {
		g.speed--
	}
	switch {
	case pressed&ButtonSkip != 0:
		g.paused = false
		g.newPair()
		return
	case pressed&ButtonNewLeft != 0:
		g.paused = false
		g.replace(0)
		return
	case pressed&ButtonNewRight != 0:
		g.paused = false
		g.replace(1)
		return
	}
	if g.paused {
		return
	}

	switch g.state {
	case running:
		for range Speeds[g.speed] {
			if done, _ := g.m.Done(); done {
				break
			}
			g.m.Step()
		}
		if done, winner := g.m.Done(); done {
			g.m.RecordResults()
			g.winner = winner
			if winner >= 0 {
				g.wins[winner]++
			}
			g.state, g.timer = roundOver, roundPause
		}
	case roundOver:
		if g.timer--; g.timer > 0 {
			return
		}
		switch {
		case g.wins[0] >= winsNeeded:
			g.winner = 0
		case g.wins[1] >= winsNeeded:
			g.winner = 1
		case g.round >= RoundsPerMatch:
			g.winner = -1
			if g.wins[0] != g.wins[1] {
				g.winner = 1
				if g.wins[0] > g.wins[1] {
					g.winner = 0
				}
			}
		default:
			g.startRound()
			return
		}
		g.state, g.timer = matchOver, matchPause
	case matchOver:
		if g.timer--; g.timer <= 0 {
			g.nextMatch()
		}
	}
}

// Draw brings the display up to date, touching only what has changed since
// the last call.
func (g *Game) Draw(d Display) {
	if g.redrawAll {
		d.FillRect(0, 0, ScreenWidth, StatusHeight, ColorBackground)
		d.FillRect(0, StatusHeight-1, ScreenWidth, 1, ColorDivider)
		for _, f := range g.fields() {
			f.valid = false
		}
		g.core.cleared = true
		g.redrawAll = false
	}
	g.core.draw(d)
	g.updateStatus()
	for _, f := range g.fields() {
		f.draw(d)
	}
}

func (g *Game) fields() [9]*field {
	return [9]*field{
		&g.names[0], &g.names[1], &g.scores[0], &g.scores[1],
		&g.procs[0], &g.procs[1], &g.title, &g.info, &g.message,
	}
}

func (g *Game) updateStatus() {
	for i := range 2 {
		c := WarriorColor(i)
		g.names[i].set(c, g.fighters[i].Name)
		g.scores[i].setInt(ColorText, "WINS ", g.wins[i])
		g.procs[i].setInt(ColorTextDim, "PROC ", g.m.Processes(i))
	}
	g.title.setInt(ColorText, "ROUND ", g.round)
	g.title.add("/")
	g.title.addInt(RoundsPerMatch)
	g.info.setInt(ColorTextDim, "x", Speeds[g.speed])

	switch {
	case g.paused:
		g.message.set(ColorText, "PAUSED")
	case g.state == roundOver && g.winner < 0:
		g.message.set(ColorText, "TIE")
	case g.state == roundOver && g.winner == 0:
		g.message.set(ColorTeal, "<< WINS")
	case g.state == roundOver:
		g.message.set(ColorRed, "WINS >>")
	case g.state == matchOver && g.winner < 0:
		g.message.set(ColorText, "MATCH DRAWN")
	case g.state == matchOver && g.winner == 0:
		g.message.set(ColorTeal, "<< MATCH")
	case g.state == matchOver:
		g.message.set(ColorRed, "MATCH >>")
	default:
		g.message.setInt(ColorTextDim, "CYCLE ", g.m.Cycle)
	}
}
