package mars_test

import (
	"testing"

	"github.com/davecheney/corewar/mars"
	"github.com/davecheney/corewar/redcode"
)

func load(t *testing.T, srcs ...string) *mars.MARS {
	t.Helper()
	cfg := mars.DefaultConfig()
	m := mars.New(cfg, len(srcs))
	for i, src := range srcs {
		w, err := redcode.Assemble(src, cfg)
		if err != nil {
			t.Fatal(err)
		}
		if err := m.Load(w, i*4000); err != nil {
			t.Fatal(err)
		}
	}
	return m
}

func run(m *mars.MARS, steps int) {
	for range steps {
		m.Step()
	}
}

func TestImp(t *testing.T) {
	m := load(t, "MOV 0, 1")
	run(m, 100)
	for i := range 101 {
		if got := m.Core[i]; got.Op != mars.MOV || got.A != 0 || got.B != 1 {
			t.Fatalf("core[%d] = %v", i, got)
		}
	}
	if pc := m.PC(0); pc != 100 {
		t.Fatalf("pc = %d, want 100", pc)
	}
}

func TestDatKills(t *testing.T) {
	m := load(t, "DAT 0, 0")
	m.Step()
	if done, winner := m.Done(); !done || winner != -1 {
		t.Fatalf("Done() = %v, %v", done, winner)
	}
}

func TestDwarfBeatsSittingDuck(t *testing.T) {
	dwarf := `
        ORG loop
loop    ADD.AB  #4, bomb
        MOV.I   bomb, @bomb
        JMP     loop
bomb    DAT     #0, #0`
	// A target that loops at an address the dwarf will bomb.
	duck := "DAT 0, 0\nDAT 0, 0\nDAT 0, 0\nstart JMP 0\nEND start"
	m := load(t, dwarf, duck)
	for {
		if done, winner := m.Done(); done {
			if winner != 0 {
				t.Fatalf("winner = %d, want 0", winner)
			}
			break
		}
		m.Step()
	}
}

func TestSplit(t *testing.T) {
	m := load(t, "SPL 0\nJMP -1")
	run(m, 10)
	if n := m.Processes(0); n != 7 {
		t.Fatalf("processes = %d, want 7", n)
	}
}

func TestMaxProcesses(t *testing.T) {
	cfg := mars.DefaultConfig()
	cfg.MaxProcesses = 16
	m := mars.New(cfg, 1)
	w, _ := redcode.Assemble("SPL 0\nJMP -1", cfg)
	m.Load(w, 0)
	run(m, 1000)
	if n := m.Processes(0); n != 16 {
		t.Fatalf("processes = %d, want 16", n)
	}
}

func TestTie(t *testing.T) {
	cfg := mars.DefaultConfig()
	cfg.MaxCycles = 50
	m := mars.New(cfg, 2)
	for i := range 2 {
		w, _ := redcode.Assemble("JMP 0", cfg)
		m.Load(w, i*100)
	}
	run(m, 1000)
	if done, winner := m.Done(); !done || winner != -1 || m.Cycle != 50 {
		t.Fatalf("Done() = %v, %v at cycle %d", done, winner, m.Cycle)
	}
}

// step runs the first instruction of src and returns the core.
func step(t *testing.T, src string) []mars.Instruction {
	t.Helper()
	m := load(t, src)
	m.Step()
	return m.Core
}

func TestModifiers(t *testing.T) {
	tests := []struct {
		src  string
		addr int
		a, b uint16
	}{
		{"MOV.A 1, 2\nDAT 5, 6\nDAT 0, 0", 2, 5, 0},
		{"MOV.B 1, 2\nDAT 5, 6\nDAT 0, 0", 2, 0, 6},
		{"MOV.AB 1, 2\nDAT 5, 6\nDAT 0, 0", 2, 0, 5},
		{"MOV.BA 1, 2\nDAT 5, 6\nDAT 0, 0", 2, 6, 0},
		{"MOV.F 1, 2\nDAT 5, 6\nDAT 0, 0", 2, 5, 6},
		{"MOV.X 1, 2\nDAT 5, 6\nDAT 0, 0", 2, 6, 5},
		{"ADD.F 1, 2\nDAT 5, 6\nDAT 10, 20", 2, 15, 26},
		{"ADD.X 1, 2\nDAT 5, 6\nDAT 10, 20", 2, 16, 25},
		{"SUB.AB #3, 1\nDAT 0, 1", 1, 0, 7998},
		{"MUL.F 1, 2\nDAT 5, 6\nDAT 10, 20", 2, 50, 120},
		{"DIV.F 1, 2\nDAT 5, 6\nDAT 12, 20", 2, 2, 3},
		{"MOD.F 1, 2\nDAT 5, 6\nDAT 12, 20", 2, 2, 2},
		{"DJN.F 0, 1\nDAT 5, 6", 1, 4, 5},
	}
	for _, tt := range tests {
		core := step(t, tt.src)
		if got := core[tt.addr]; got.A != tt.a || got.B != tt.b {
			t.Errorf("%q: core[%d] = %v, want A=%d B=%d", tt.src, tt.addr, got, tt.a, tt.b)
		}
	}
}

func TestDivideByZeroKills(t *testing.T) {
	m := load(t, "DIV.F 1, 2\nDAT 0, 2\nDAT 10, 10")
	m.Step()
	if m.Processes(0) != 0 {
		t.Fatal("process survived division by zero")
	}
	if got := m.Core[2]; got.A != 10 || got.B != 5 {
		t.Fatalf("core[2] = %v, want the non-zero half written", got)
	}
}

func TestAddressingModes(t *testing.T) {
	tests := []struct {
		name string
		src  string
		addr int
		want mars.Instruction
	}{
		// Each copies the NOP to wherever the B operand points.
		{"direct", "MOV 1, 3\nNOP\nDAT 0, 0", 3, nop()},
		{"b-indirect", "MOV 1, @2\nNOP\nDAT 0, 2", 4, nop()},
		{"a-indirect", "MOV 1, *2\nNOP\nDAT 2, 0", 4, nop()},
		{"b-predec", "MOV 1, <2\nNOP\nDAT 0, 3", 4, nop()},
		{"a-predec", "MOV 1, {2\nNOP\nDAT 3, 0", 4, nop()},
	}
	for _, tt := range tests {
		core := step(t, tt.src)
		if got := core[tt.addr]; got != tt.want {
			t.Errorf("%s: core[%d] = %v, want %v", tt.name, tt.addr, got, tt.want)
		}
	}

	// Postincrement reads through the old value, then increments.
	core := step(t, "MOV 1, >2\nNOP\nDAT 0, 2")
	if core[4] != nop() || core[2].B != 3 {
		t.Errorf("b-postinc: core[4] = %v, core[2] = %v", core[4], core[2])
	}
	core = step(t, "MOV 1, }2\nNOP\nDAT 2, 0")
	if core[4] != nop() || core[2].A != 3 {
		t.Errorf("a-postinc: core[4] = %v, core[2] = %v", core[4], core[2])
	}
}

func nop() mars.Instruction {
	return mars.Instruction{Op: mars.NOP, Mod: mars.ModF, AMode: mars.Direct, BMode: mars.Direct}
}

func TestJumpsAndSkips(t *testing.T) {
	tests := []struct {
		src string
		pc  int
	}{
		{"JMP 5", 5},
		{"JMZ 5, 1\nDAT 0, 0", 5},
		{"JMZ 5, 1\nDAT 0, 1", 1},
		{"JMN 5, 1\nDAT 0, 1", 5},
		{"JMN.A 5, 1\nDAT 0, 1", 1},
		{"DJN 5, 1\nDAT 0, 2", 5},
		{"DJN 5, 1\nDAT 0, 1", 1},
		{"SEQ 1, 2\nDAT 1, 2\nDAT 1, 2", 2},
		{"SEQ 1, 2\nDAT 1, 2\nDAT 1, 3", 1},
		{"CMP.X 1, 2\nDAT 1, 2\nDAT 2, 1", 2},
		{"SNE 1, 2\nDAT 1, 2\nDAT 1, 3", 2},
		{"SLT #1, 1\nDAT 0, 2", 2},
		{"SLT #2, 1\nDAT 0, 2", 1},
	}
	for _, tt := range tests {
		m := load(t, tt.src)
		m.Step()
		if pc := m.PC(0); pc != tt.pc {
			t.Errorf("%q: pc = %d, want %d", tt.src, pc, tt.pc)
		}
	}
}

func TestTouch(t *testing.T) {
	cfg := mars.DefaultConfig()
	m := mars.New(cfg, 1)
	var writes, execs []int
	m.Touch = func(addr, w int, k mars.TouchKind) {
		if k == mars.Exec {
			execs = append(execs, addr)
		} else {
			writes = append(writes, addr)
		}
	}
	w, _ := redcode.Assemble("MOV 0, 1", cfg)
	m.Load(w, 10)
	m.Step()
	if len(execs) != 1 || execs[0] != 10 {
		t.Errorf("execs = %v", execs)
	}
	if len(writes) != 2 || writes[0] != 10 || writes[1] != 11 {
		t.Errorf("writes = %v", writes)
	}
}

func BenchmarkStep(b *testing.B) {
	cfg := mars.DefaultConfig()
	m := mars.New(cfg, 1)
	w, _ := redcode.Assemble("SPL 0\nMOV 0, 1", cfg)
	m.Load(w, 0)
	for b.Loop() {
		m.Step()
	}
}

func TestPSpace(t *testing.T) {
	m := load(t, "STP.AB #123, #10\nLDP.AB #10, 2\nLDP.AB #0, 2\nDAT 0, 0\nDAT 0, 0")
	run(m, 3)
	if got := m.PSpace(0)[10]; got != 123 {
		t.Errorf("P-space[10] = %d, want 123", got)
	}
	if got := m.Core[3].B; got != 123 {
		t.Errorf("LDP from P-space 10 loaded %d, want 123", got)
	}
	if got := m.Core[4].B; got != 7999 {
		t.Errorf("result cell before any round = %d, want -1", got)
	}

	// The result cell holds 0 for a loss and the number of survivors
	// otherwise, and survives Reset but not ClearPSpace.
	m = load(t, "JMP 0", "DAT 0, 0")
	run(m, 2)
	m.RecordResults()
	m.Reset()
	if w, l := m.PSpace(0)[0], m.PSpace(1)[0]; w != 1 || l != 0 {
		t.Errorf("results = %d, %d; want 1, 0", w, l)
	}
	m.ClearPSpace()
	if got := m.PSpace(0)[0]; got != 7999 {
		t.Errorf("after ClearPSpace result = %d, want -1", got)
	}
}
