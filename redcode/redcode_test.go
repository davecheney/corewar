package redcode

import (
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/davecheney/corewar/mars"
)

func TestAssemble(t *testing.T) {
	src := `;redcode-94
;name Dwarf
;author A. K. Dewdney
step    EQU     4
        ORG     loop
bomb    DAT     #0
loop:   ADD     #step, bomb
        MOV     bomb, @bomb
        JMP     loop
        cmp.x   #1+2*3, (bomb-2)%5
        END
        DAT     1, 1
`
	w, err := Assemble(src, mars.DefaultConfig())
	if err != nil {
		t.Fatal(err)
	}
	if w.Name != "Dwarf" || w.Author != "A. K. Dewdney" {
		t.Errorf("metadata = %q, %q", w.Name, w.Author)
	}
	if w.Start != 1 {
		t.Errorf("start = %d, want 1", w.Start)
	}
	want := []mars.Instruction{
		{Op: mars.DAT, Mod: mars.ModF, AMode: mars.Immediate, BMode: mars.Immediate, A: 0, B: 0},
		{Op: mars.ADD, Mod: mars.ModAB, AMode: mars.Immediate, BMode: mars.Direct, A: 4, B: 7999},
		{Op: mars.MOV, Mod: mars.ModI, AMode: mars.Direct, BMode: mars.BIndirect, A: 7998, B: 7998},
		{Op: mars.JMP, Mod: mars.ModB, AMode: mars.Direct, BMode: mars.Direct, A: 7998, B: 0},
		{Op: mars.SEQ, Mod: mars.ModX, AMode: mars.Immediate, BMode: mars.Direct, A: 7, B: 7999},
	}
	if len(w.Code) != len(want) {
		t.Fatalf("got %d instructions, want %d", len(w.Code), len(want))
	}
	for i := range want {
		if w.Code[i] != want[i] {
			t.Errorf("%d: got %v, want %v", i, w.Code[i], want[i])
		}
	}
}

func TestModes(t *testing.T) {
	w, err := Assemble("MOV #1, $2\nMOV *1, @2\nMOV {1, <2\nMOV }1, >2\n", mars.DefaultConfig())
	if err != nil {
		t.Fatal(err)
	}
	modes := [][2]mars.Mode{
		{mars.Immediate, mars.Direct},
		{mars.AIndirect, mars.BIndirect},
		{mars.APredec, mars.BPredec},
		{mars.APostinc, mars.BPostinc},
	}
	for i, m := range modes {
		if w.Code[i].AMode != m[0] || w.Code[i].BMode != m[1] {
			t.Errorf("%d: modes %v %v, want %v %v", i, w.Code[i].AMode, w.Code[i].BMode, m[0], m[1])
		}
	}
}

func TestNameComment(t *testing.T) {
	cfg := mars.DefaultConfig()
	tests := []struct {
		src  string
		want string
	}{
		{";name Comment\nMOV 0, 1", "Comment"},
		{"  ;name   Two Words  \nMOV 0, 1", "Two Words"},
		{";name First\n;name Second\nMOV 0, 1", "First"},
		{";named thing\nMOV 0, 1", ""},
		{"MOV 0, 1", ""},
	}
	for _, tt := range tests {
		w, err := Assemble(tt.src, cfg)
		if err != nil {
			t.Errorf("%q: %v", tt.src, err)
			continue
		}
		if w.Name != tt.want {
			t.Errorf("%q: name = %q, want %q", tt.src, w.Name, tt.want)
		}
	}
}

// zork.red is a 42-school Corewar champion, not ICWS'94 Redcode. It must be
// rejected at its first line.
func TestRejectsNonRedcode(t *testing.T) {
	src, err := os.ReadFile("testdata/zork.red")
	if err != nil {
		t.Fatal(err)
	}
	_, err = Assemble(string(src), mars.DefaultConfig())
	var aerr *Error
	if !errors.As(err, &aerr) || aerr.Line != 1 || !strings.Contains(aerr.Msg, ".name") {
		t.Fatalf("got %v, want an unknown directive .name error on line 1", err)
	}
}

func TestErrors(t *testing.T) {
	for _, src := range []string{
		"",
		"FOO 1, 2",
		"MOV.Q 1, 2",
		"MOV 1, 2, 3",
		"MOV nowhere, 1",
		"x EQU x\nMOV x, 1",
		"a DAT 1\na DAT 2",
		"MOV 1/0, 1",
		".name \"Zork\"\nMOV 0, 1",
		".comment \"hi\"\nMOV 0, 1",
		"l2: sti r1, %:live, %1",
	} {
		if _, err := Assemble(src, mars.DefaultConfig()); err == nil {
			t.Errorf("%q: expected error", src)
		}
	}
}

func TestSkipsTextBeforeRedcodeLine(t *testing.T) {
	src := "From: someone@example.com\nSubject: my entry\n\n;redcode-94\n;name Mailed\nMOV 0, 1\nEND\nThanks, bye!\n"
	w, err := Assemble(src, mars.DefaultConfig())
	if err != nil {
		t.Fatal(err)
	}
	if w.Name != "Mailed" || len(w.Code) != 1 {
		t.Errorf("got %q with %d instructions", w.Name, len(w.Code))
	}
	// Errors still report the line in the original file.
	_, err = Assemble("junk\n;redcode\nMOV 0, 1\nbad!\n", mars.DefaultConfig())
	var e *Error
	if !errors.As(err, &e) || e.Line != 4 {
		t.Errorf("err = %v, want an error on line 4", err)
	}
}

func TestForRof(t *testing.T) {
	if !ForEnabled {
		t.Skip("FOR/ROF is disabled")
	}
	src := `N    EQU 2
     FOR 0
this block is a comment, and need not be Redcode: for rof!
     ROF
i    FOR N
x&i  DAT #i, #x&i-x01
j      FOR 2
       DAT.I #j, #i
       ROF
     ROF
     JMP x02
`
	w, err := Assemble(src, mars.DefaultConfig())
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, ins := range w.Code {
		got = append(got, ins.String())
	}
	want := []string{
		"DAT.F #1, #0",
		"DAT.I #1, #1",
		"DAT.I #2, #1",
		"DAT.F #2, #3",
		"DAT.I #1, #2",
		"DAT.I #2, #2",
		"JMP.B $7997, $0",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("got\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

func TestForErrors(t *testing.T) {
	if !ForEnabled {
		t.Skip("FOR/ROF is disabled")
	}
	cfg := mars.DefaultConfig()
	for _, src := range []string{
		"FOR 2\nDAT 0\n",
		"DAT 0\nROF\n",
		"FOR 100000\nDAT 0\nDAT 0\nROF\n",
	} {
		if _, err := Assemble(src, cfg); err == nil {
			t.Errorf("%q: no error", src)
		}
	}
}

func TestPSpace(t *testing.T) {
	src := `PIN 42
slot EQU #7
     LDP #0, res
     STP.AB res, slot
     ldp.a PSPACESIZE-1, 0
res  DAT 0, 0
`
	w, err := Assemble(src, mars.DefaultConfig())
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"LDP.AB #0, $3",
		"STP.AB $2, #7",
		"LDP.A $499, $0",
		"DAT.F $0, $0",
	}
	for i, ins := range w.Code {
		if ins.String() != want[i] {
			t.Errorf("%d: got %v, want %s", i, ins, want[i])
		}
	}
}

func TestAssertIgnored(t *testing.T) {
	w, err := Assemble(";assert CORESIZE==8192\nMOV 0, 1\n", mars.DefaultConfig())
	if err != nil || len(w.Code) != 1 {
		t.Fatalf("got %v, %v; want the warrior assembled", w, err)
	}
}

func TestComparisons(t *testing.T) {
	cfg := mars.DefaultConfig()
	for _, tt := range []struct {
		expr string
		want int
	}{
		{"CORESIZE == 8000", 1},
		{"CORESIZE==8192", 0},
		{"CORESIZE > 1 && MAXLENGTH >= 100", 1},
		{"CORESIZE < 1 || !(VERSION >= 80)", 0},
		{"CORESIZE != 8000", 0},
		{"2 + 3 * 4 == 14", 1},
	} {
		w, err := Assemble("DAT #("+tt.expr+"), #0\n", cfg)
		if err != nil {
			t.Errorf("%s: %v", tt.expr, err)
			continue
		}
		if got := int(w.Code[0].A); got != tt.want {
			t.Errorf("%s = %d, want %d", tt.expr, got, tt.want)
		}
	}
}

func TestEQUWithMode(t *testing.T) {
	w, err := Assemble("i EQU #5\nMOV.I i, i+1\n", mars.DefaultConfig())
	if err != nil {
		t.Fatal(err)
	}
	if got := w.Code[0].String(); got != "MOV.I #5, #6" {
		t.Errorf("got %s", got)
	}
}

func TestEQUWithComma(t *testing.T) {
	w, err := Assemble("v EQU 2,0\nDAT v\nk EQU #1, $2\nMOV k\n", mars.DefaultConfig())
	if err != nil {
		t.Fatal(err)
	}
	for i, want := range []string{"DAT.F $2, $0", "MOV.AB #1, $2"} {
		if got := w.Code[i].String(); got != want {
			t.Errorf("%d: got %s, want %s", i, got, want)
		}
	}
}

func TestCommaless(t *testing.T) {
	src := "start spl 1 1\n mov < src # 10\nsrc dat 5\n jmz @ start -1\n jmp start\n end start\n"
	w, err := Assemble(src, mars.DefaultConfig())
	if err != nil {
		t.Fatal(err)
	}
	for i, want := range []string{"SPL.B $1, $1", "MOV.B <1, #10", "DAT.F #0, $5", "JMZ.B @7997, $7999", "JMP.B $7996, $0"} {
		if got := w.Code[i].String(); got != want {
			t.Errorf("%d: got %s, want %s", i, got, want)
		}
	}
	// With a comma anywhere, whitespace does not separate operands.
	w, err = Assemble("x dat 1, 2\n jmp x +1\n", mars.DefaultConfig())
	if err != nil {
		t.Fatal(err)
	}
	if got := w.Code[1].String(); got != "JMP.B $0, $0" {
		t.Errorf("got %s, want JMP.B $0, $0", got)
	}
}

func TestForDisabled(t *testing.T) {
	if ForEnabled {
		t.Skip("FOR/ROF is enabled")
	}
	for _, src := range []string{"i FOR 2\nDAT 0\nROF\n", "DAT 0\nROF\n"} {
		_, err := Assemble(src, mars.DefaultConfig())
		var e *Error
		if !errors.As(err, &e) || !strings.Contains(e.Msg, "not supported") {
			t.Errorf("%q: err = %v, want FOR/ROF not supported", src, err)
		}
	}
}
