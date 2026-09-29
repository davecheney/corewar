// Package mars implements a Memory Array Redcode Simulator for ICWS'94:
// every opcode, modifier and addressing mode, plus the pMARS P-space
// extension (LDP and STP).
//
// It has no platform dependencies and does not allocate once a battle is
// loaded, so the same code runs on the desktop and under TinyGo on an RP2040.
package mars

import "fmt"

//go:generate go tool stringer -type=Opcode
//go:generate go tool stringer -type=Modifier -trimprefix=Mod

// Opcode is a Redcode operation.
type Opcode uint8

const (
	DAT Opcode = iota
	MOV
	ADD
	SUB
	MUL
	DIV
	MOD
	JMP
	JMZ
	JMN
	DJN
	SPL
	SEQ
	SNE
	SLT
	NOP
	LDP
	STP
)

// NumOpcodes is the number of opcodes; CMP is an alias for SEQ.
const NumOpcodes = int(STP) + 1

// Modifier selects which fields of the operands an opcode works on.
type Modifier uint8

const (
	ModA Modifier = iota
	ModB
	ModAB
	ModBA
	ModF
	ModX
	ModI
)

// NumModifiers is the number of modifiers.
const NumModifiers = int(ModI) + 1

// Mode is an operand addressing mode.
type Mode uint8

const (
	Immediate Mode = iota // #
	Direct                // $
	AIndirect             // *
	BIndirect             // @
	APredec               // {
	BPredec               // <
	APostinc              // }
	BPostinc              // >
	numModes
)

// ModeChars maps each Mode to its Redcode prefix character.
const ModeChars = "#$*@{<}>"

func (m Mode) String() string {
	if m < numModes {
		return ModeChars[m : m+1]
	}
	return fmt.Sprintf("Mode(%d)", uint8(m))
}

// Instruction is one core cell. Fields are stored already folded into
// [0, CoreSize).
type Instruction struct {
	Op    Opcode
	Mod   Modifier
	AMode Mode
	BMode Mode
	A     uint16
	B     uint16
}

func (i Instruction) String() string {
	return fmt.Sprintf("%v.%v %v%d, %v%d", i.Op, i.Mod, i.AMode, i.A, i.BMode, i.B)
}

// DefaultModifier returns the ICWS'94 default modifier for an opcode given
// its operand modes, used when the source does not name one.
func DefaultModifier(op Opcode, a, b Mode) Modifier {
	switch op {
	case DAT, NOP:
		return ModF
	case MOV, SEQ, SNE:
		switch {
		case a == Immediate:
			return ModAB
		case b == Immediate:
			return ModB
		}
		return ModI
	case ADD, SUB, MUL, DIV, MOD:
		switch {
		case a == Immediate:
			return ModAB
		case b == Immediate:
			return ModB
		}
		return ModF
	case SLT, LDP, STP:
		if a == Immediate {
			return ModAB
		}
		return ModB
	}
	return ModB // JMP JMZ JMN DJN SPL
}
