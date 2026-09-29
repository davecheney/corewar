package mars

import "errors"

// Config holds the battle parameters.
type Config struct {
	CoreSize     int
	MaxCycles    int
	MaxProcesses int
	MaxLength    int
	MinDistance  int
	PSpaceSize   int // cells of private P-space per warrior
}

// DefaultConfig is the classic '94 hill setup.
func DefaultConfig() Config {
	return Config{
		CoreSize:     8000,
		MaxCycles:    80000,
		MaxProcesses: 8000,
		MaxLength:    100,
		MinDistance:  100,
		PSpaceSize:   500,
	}
}

// Warrior is an assembled program ready to load into the core.
type Warrior struct {
	Name   string
	Author string
	Code   []Instruction
	Start  int // offset of the first instruction to execute
}

// TouchKind says how a core cell was touched.
type TouchKind uint8

const (
	Write TouchKind = iota
	Exec
)

// TouchFunc is called whenever a warrior's process writes to or executes a
// cell. It is the only hook the display needs into the simulation.
type TouchFunc func(addr int, warrior int, kind TouchKind)

// queue is a fixed-size ring buffer of process counters.
type queue struct {
	buf  []uint16
	head int
	n    int
}

func (q *queue) push(pc int) {
	if q.n == len(q.buf) {
		return
	}
	i := q.head + q.n
	if i >= len(q.buf) {
		i -= len(q.buf)
	}
	q.buf[i] = uint16(pc)
	q.n++
}

func (q *queue) pop() int {
	pc := q.buf[q.head]
	q.head++
	if q.head == len(q.buf) {
		q.head = 0
	}
	q.n--
	return int(pc)
}

func (q *queue) peek() int { return int(q.buf[q.head]) }

// MARS is the simulator. Create it with New; all storage is allocated then
// and reused across battles.
type MARS struct {
	Config
	Core []Instruction

	// Touch, if non-nil, observes every write and execute.
	Touch TouchFunc

	// Cycle counts completed rounds of turns, one instruction per living
	// warrior per cycle.
	Cycle int

	queues []queue
	pspace [][]uint16
	loaded int
	turn   int
}

// New allocates a simulator for up to warriors combatants.
func New(cfg Config, warriors int) *MARS {
	m := &MARS{
		Config: cfg,
		Core:   make([]Instruction, cfg.CoreSize),
		queues: make([]queue, warriors),
	}
	if m.PSpaceSize < 1 {
		m.PSpaceSize = 1
	}
	m.pspace = make([][]uint16, warriors)
	for i := range m.queues {
		m.queues[i].buf = make([]uint16, cfg.MaxProcesses)
		m.pspace[i] = make([]uint16, m.PSpaceSize)
	}
	m.Reset()
	m.ClearPSpace()
	return m
}

// ClearPSpace zeroes every warrior's P-space ready for a new match, and sets
// each result cell, P-space 0, to -1 to say no round has been fought.
func (m *MARS) ClearPSpace() {
	for _, p := range m.pspace {
		clear(p)
		p[0] = uint16(m.CoreSize - 1)
	}
}

// RecordResults stores the outcome of the finished round in each warrior's
// result cell, P-space 0, as pMARS does: 0 for a warrior that died, and the
// number of survivors for one that lived.
func (m *MARS) RecordResults() {
	alive := m.Alive()
	for i := range m.loaded {
		v := 0
		if m.queues[i].n > 0 {
			v = alive
		}
		m.pspace[i][0] = uint16(v)
	}
}

// PSpace returns warrior w's P-space.
func (m *MARS) PSpace(w int) []uint16 { return m.pspace[w] }

// Reset clears the core to DAT.F $0, $0 and removes every process, ready
// for the next round. P-space is kept; see ClearPSpace.
func (m *MARS) Reset() {
	blank := Instruction{Op: DAT, Mod: ModF, AMode: Direct, BMode: Direct}
	for i := range m.Core {
		m.Core[i] = blank
	}
	for i := range m.queues {
		m.queues[i].head, m.queues[i].n = 0, 0
	}
	m.loaded, m.turn, m.Cycle = 0, 0, 0
}

var (
	ErrTooLong     = errors.New("mars: warrior exceeds MaxLength")
	ErrTooMany     = errors.New("mars: too many warriors")
	ErrEmptyWarior = errors.New("mars: warrior has no code")
)

// Load copies w into the core at addr and gives it a single process at its
// start. Warriors take turns in the order they are loaded.
func (m *MARS) Load(w *Warrior, addr int) error {
	if m.loaded == len(m.queues) {
		return ErrTooMany
	}
	if len(w.Code) == 0 {
		return ErrEmptyWarior
	}
	if len(w.Code) > m.MaxLength {
		return ErrTooLong
	}
	id := m.loaded
	for i, ins := range w.Code {
		a := m.fold(addr + i)
		m.Core[a] = ins
		m.touch(a, id, Write)
	}
	m.queues[id].push(m.fold(addr + w.Start))
	m.loaded++
	return nil
}

// Processes returns how many processes warrior w has.
func (m *MARS) Processes(w int) int { return m.queues[w].n }

// PC returns the address warrior w will execute next, or -1 if it is dead.
func (m *MARS) PC(w int) int {
	if m.queues[w].n == 0 {
		return -1
	}
	return m.queues[w].peek()
}

// Alive returns how many loaded warriors still have processes.
func (m *MARS) Alive() int {
	n := 0
	for i := range m.loaded {
		if m.queues[i].n > 0 {
			n++
		}
	}
	return n
}

// Done reports whether the battle is over and, if so, the winner: the index
// of the sole survivor, or -1 for a tie.
func (m *MARS) Done() (bool, int) {
	alive, last := 0, -1
	for i := range m.loaded {
		if m.queues[i].n > 0 {
			alive++
			last = i
		}
	}
	switch {
	case m.loaded > 1 && alive <= 1:
		return true, last
	case m.loaded == 1 && alive == 0:
		return true, -1
	case m.Cycle >= m.MaxCycles:
		return true, -1
	}
	return false, -1
}

// Step executes one instruction for the warrior whose turn it is. It does
// nothing if the battle is over.
func (m *MARS) Step() {
	if done, _ := m.Done(); done {
		return
	}
	for m.queues[m.turn].n == 0 {
		m.nextTurn()
	}
	w := m.turn
	m.exec(w, m.queues[w].pop())
	m.nextTurn()
}

func (m *MARS) nextTurn() {
	m.turn++
	if m.turn >= m.loaded {
		m.turn = 0
		m.Cycle++
	}
}

func (m *MARS) fold(a int) int {
	a %= m.CoreSize
	if a < 0 {
		a += m.CoreSize
	}
	return a
}

func (m *MARS) touch(addr, w int, k TouchKind) {
	if m.Touch != nil {
		m.Touch(addr, w, k)
	}
}

func (m *MARS) inc(v uint16) uint16 {
	if int(v)+1 == m.CoreSize {
		return 0
	}
	return v + 1
}

func (m *MARS) dec(v uint16) uint16 {
	if v == 0 {
		return uint16(m.CoreSize - 1)
	}
	return v - 1
}

// operand evaluates one operand in the ICWS'94 order: predecrement, compute
// the pointer, copy the target, then postincrement. It returns the offset
// from pc and a copy of the cell it addresses.
func (m *MARS) operand(w, pc int, mode Mode, field uint16) (int, Instruction) {
	if mode == Immediate {
		return 0, m.Core[pc]
	}
	off := int(field)
	if mode == Direct {
		return off, m.Core[m.fold(pc+off)]
	}
	p := m.fold(pc + off)
	cell := &m.Core[p]
	switch mode {
	case APredec:
		cell.A = m.dec(cell.A)
		m.touch(p, w, Write)
	case BPredec:
		cell.B = m.dec(cell.B)
		m.touch(p, w, Write)
	}
	switch mode {
	case AIndirect, APredec, APostinc:
		off += int(cell.A)
	default:
		off += int(cell.B)
	}
	off %= m.CoreSize
	ir := m.Core[m.fold(pc+off)]
	switch mode {
	case APostinc:
		cell.A = m.inc(cell.A)
		m.touch(p, w, Write)
	case BPostinc:
		cell.B = m.inc(cell.B)
		m.touch(p, w, Write)
	}
	return off, ir
}

const (
	opAdd = iota
	opSub
	opMul
	opDiv
	opMod
)

// arith computes b op a folded into the core, reporting false on division
// by zero.
func (m *MARS) arith(op int, b, a uint16) (uint16, bool) {
	size := uint32(m.CoreSize)
	x, y := uint32(b), uint32(a)
	switch op {
	case opAdd:
		return uint16((x + y) % size), true
	case opSub:
		return uint16((x + size - y) % size), true
	case opMul:
		return uint16((x * y) % size), true
	case opDiv:
		if y == 0 {
			return 0, false
		}
		return uint16(x / y), true
	default:
		if y == 0 {
			return 0, false
		}
		return uint16(x % y), true
	}
}

func (m *MARS) exec(w, pc int) {
	ir := m.Core[pc]
	m.touch(pc, w, Exec)

	rpa, air := m.operand(w, pc, ir.AMode, ir.A)
	rpb, bir := m.operand(w, pc, ir.BMode, ir.B)
	wpb := m.fold(pc + rpb)
	dst := &m.Core[wpb]
	q := &m.queues[w]
	next := m.fold(pc + 1)

	switch ir.Op {
	case DAT:
		return

	case MOV:
		switch ir.Mod {
		case ModA:
			dst.A = air.A
		case ModB:
			dst.B = air.B
		case ModAB:
			dst.B = air.A
		case ModBA:
			dst.A = air.B
		case ModF:
			dst.A, dst.B = air.A, air.B
		case ModX:
			dst.A, dst.B = air.B, air.A
		case ModI:
			*dst = air
		}
		m.touch(wpb, w, Write)

	case ADD, SUB, MUL, DIV, MOD:
		op := int(ir.Op - ADD)
		ok := true
		set := func(f *uint16, b, a uint16) {
			v, good := m.arith(op, b, a)
			if good {
				*f = v
			} else {
				ok = false
			}
		}
		switch ir.Mod {
		case ModA:
			set(&dst.A, bir.A, air.A)
		case ModB:
			set(&dst.B, bir.B, air.B)
		case ModAB:
			set(&dst.B, bir.B, air.A)
		case ModBA:
			set(&dst.A, bir.A, air.B)
		case ModF, ModI:
			set(&dst.A, bir.A, air.A)
			set(&dst.B, bir.B, air.B)
		case ModX:
			set(&dst.A, bir.A, air.B)
			set(&dst.B, bir.B, air.A)
		}
		m.touch(wpb, w, Write)
		if !ok {
			return
		}

	case JMP:
		next = m.fold(pc + rpa)

	case JMZ:
		if m.zero(ir.Mod, bir) {
			next = m.fold(pc + rpa)
		}

	case JMN:
		if !m.zero(ir.Mod, bir) {
			next = m.fold(pc + rpa)
		}

	case DJN:
		switch ir.Mod {
		case ModA, ModBA:
			dst.A = m.dec(dst.A)
			bir.A = m.dec(bir.A)
		case ModB, ModAB:
			dst.B = m.dec(dst.B)
			bir.B = m.dec(bir.B)
		default:
			dst.A, dst.B = m.dec(dst.A), m.dec(dst.B)
			bir.A, bir.B = m.dec(bir.A), m.dec(bir.B)
		}
		m.touch(wpb, w, Write)
		if !m.zero(ir.Mod, bir) {
			next = m.fold(pc + rpa)
		}

	case SPL:
		q.push(next)
		q.push(m.fold(pc + rpa))
		return

	case SEQ:
		if m.equal(ir.Mod, air, bir) {
			next = m.fold(pc + 2)
		}

	case SNE:
		if !m.equal(ir.Mod, air, bir) {
			next = m.fold(pc + 2)
		}

	case SLT:
		var lt bool
		switch ir.Mod {
		case ModA:
			lt = air.A < bir.A
		case ModB:
			lt = air.B < bir.B
		case ModAB:
			lt = air.A < bir.B
		case ModBA:
			lt = air.B < bir.A
		case ModF, ModI:
			lt = air.A < bir.A && air.B < bir.B
		case ModX:
			lt = air.A < bir.B && air.B < bir.A
		}
		if lt {
			next = m.fold(pc + 2)
		}

	case LDP:
		p := m.pspace[w]
		switch ir.Mod {
		case ModA:
			dst.A = p[int(air.A)%len(p)]
		case ModAB:
			dst.B = p[int(air.A)%len(p)]
		case ModBA:
			dst.A = p[int(air.B)%len(p)]
		default:
			dst.B = p[int(air.B)%len(p)]
		}
		m.touch(wpb, w, Write)

	case STP:
		p := m.pspace[w]
		switch ir.Mod {
		case ModA:
			p[int(bir.A)%len(p)] = air.A
		case ModAB:
			p[int(bir.B)%len(p)] = air.A
		case ModBA:
			p[int(bir.A)%len(p)] = air.B
		default:
			p[int(bir.B)%len(p)] = air.B
		}

	case NOP:
	}
	q.push(next)
}

// zero reports whether the fields of ir selected by mod are all zero.
func (m *MARS) zero(mod Modifier, ir Instruction) bool {
	switch mod {
	case ModA, ModBA:
		return ir.A == 0
	case ModB, ModAB:
		return ir.B == 0
	}
	return ir.A == 0 && ir.B == 0
}

func (m *MARS) equal(mod Modifier, a, b Instruction) bool {
	switch mod {
	case ModA:
		return a.A == b.A
	case ModB:
		return a.B == b.B
	case ModAB:
		return a.A == b.B
	case ModBA:
		return a.B == b.A
	case ModF:
		return a.A == b.A && a.B == b.B
	case ModX:
		return a.A == b.B && a.B == b.A
	}
	return a == b
}
