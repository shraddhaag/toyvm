package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func convertInstructionToUInt16(opCode uint16, r0, r1, r2 uint16,
	immediateMode bool,
	offset uint16,
) uint16 {
	instr := (opCode & 0xF) << 12
	instr |= (r0 & 0x7) << 9
	instr |= (r1 & 0x7) << 6
	if immediateMode {
		instr |= 1 << 5
		instr |= offset & 0x1F
	} else {
		instr |= r2 & 0x7
	}
	return instr
}

// pcRelative selects the two JSR encodings: true uses value as the raw 11-bit
// PCoffset11 pattern (JSR), false uses value as a 3-bit BaseR register index (JSRR).
func convertJsrInstructionToUInt16(pcRelative bool, value uint16) uint16 {
	instr := uint16(OpJsr&0xF) << 12
	if pcRelative {
		instr |= 1 << 11
		instr |= value & 0x7FF
	} else {
		instr |= (value & 0x7) << 6
	}
	return instr
}

// Shared by LD/LDI/LEA: opcode(4) | DR(3, bits 11-9) | PCoffset9(9, bits 8-0).
func convertDrPCOffset9InstructionToUInt16(opCode, dr, pcOffset9 uint16) uint16 {
	instr := (opCode & 0xF) << 12
	instr |= (dr & 0x7) << 9
	instr |= pcOffset9 & 0x1FF
	return instr
}

func convertBrInstructionToUInt16(n, z, p bool, pcOffset9 uint16) uint16 {
	instr := uint16(OpBr&0xF) << 12
	if n {
		instr |= 1 << 11
	}
	if z {
		instr |= 1 << 10
	}
	if p {
		instr |= 1 << 9
	}
	instr |= pcOffset9 & 0x1FF
	return instr
}

func TestAdd(t *testing.T) {
	tests := []struct {
		name          string
		registers     []uint16
		r0, r1, r2    uint16
		immediateMode bool
		offset        uint16
		output        []uint16
		cc            uint16
	}{
		// register mode
		{
			"register mode: positive result",
			[]uint16{0, 2, 3},
			0, 1, 2,
			false, 0,
			[]uint16{5, 2, 3},
			CondPos,
		},
		{
			"register mode: zero result via cancellation",
			[]uint16{0, 5, 0xFFFB}, // 0xFFFB == -5
			0, 1, 2,
			false, 0,
			[]uint16{0, 5, 0xFFFB},
			CondZro,
		},
		{
			"register mode: positive overflow into negative",
			[]uint16{0, 0x7FFF, 1},
			0, 1, 2,
			false, 0,
			[]uint16{0x8000, 0x7FFF, 1},
			CondNeg,
		},
		{
			"register mode: unsigned wraparound to zero",
			[]uint16{0, 0xFFFF, 1},
			0, 1, 2,
			false, 0,
			[]uint16{0, 0xFFFF, 1},
			CondZro,
		},
		{
			"register mode: two negative operands",
			[]uint16{0, 0xFFFF, 0xFFFF},
			0, 1, 2,
			false, 0,
			[]uint16{0xFFFE, 0xFFFF, 0xFFFF},
			CondNeg,
		},
		{
			"register mode: R0 = R2+R2",
			[]uint16{0, 5},
			0, 1, 1,
			false, 0,
			[]uint16{10, 5},
			CondPos,
		},
		{
			"register mode: R0 += R2",
			[]uint16{4, 0, 6},
			0, 0, 2,
			false, 0,
			[]uint16{10, 0, 6},
			CondPos,
		},
		{
			"register mode: R0 += R1",
			[]uint16{4, 6},
			0, 1, 0,
			false, 0,
			[]uint16{10, 6},
			CondPos,
		},
		{
			"register mode: DR == SR1 == SR2",
			[]uint16{3},
			0, 0, 0,
			false, 0,
			[]uint16{6},
			CondPos,
		},
		// immediate mode
		{
			"immediate mode: positive offset",
			[]uint16{0, 10},
			0, 1, 0,
			true, 5,
			[]uint16{15, 10},
			CondPos,
		},
		{
			"immediate mode: zero offset",
			[]uint16{0, 7},
			0, 1, 0,
			true, 0,
			[]uint16{7, 7},
			CondPos,
		},
		{
			"immediate mode: max positive imm (+15)",
			[]uint16{0, 1},
			0, 1, 0,
			true, 15,
			[]uint16{16, 1},
			CondPos,
		},
		{
			"immediate mode: max negative imm (-16), positive result",
			[]uint16{0, 20},
			0, 1, 0,
			true, 16,
			[]uint16{4, 20},
			CondPos,
		},
		{
			"immediate mode: max negative imm (-16), zero result",
			[]uint16{0, 16},
			0, 1, 0,
			true, 16,
			[]uint16{0, 16},
			CondZro,
		},
		{
			"immediate mode: max negative imm (-16), negative result",
			[]uint16{0, 10},
			0, 1, 0,
			true, 16,
			[]uint16{0xFFFA, 10},
			CondNeg,
		},
		{
			"immediate mode: DR == SR1, negative offset, positive result",
			[]uint16{16384},
			0, 0, 0,
			true, 29,
			[]uint16{16381},
			CondPos,
		},
		{
			"immediate mode: negative offset, negative result",
			[]uint16{0, 2},
			0, 1, 0,
			true, 29,
			[]uint16{0xFFFF, 2},
			CondNeg,
		},
		{
			"immediate mode: negative offset, zero result",
			[]uint16{0, 3},
			0, 1, 0,
			true, 29,
			[]uint16{0, 3},
			CondZro,
		},
		{
			"immediate mode: unsigned wraparound to zero",
			[]uint16{0, 0xFFFF},
			0, 1, 0,
			true, 1,
			[]uint16{0, 0xFFFF},
			CondZro,
		},
		{
			"immediate mode: DR == SR1, positive imm",
			[]uint16{5},
			0, 0, 0,
			true, 3,
			[]uint16{8},
			CondPos,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			vm := VM{Registers: make([]uint16, RCount)}
			copy(vm.Registers, tt.registers)

			instr := convertInstructionToUInt16(OpAdd, tt.r0, tt.r1, tt.r2, tt.immediateMode, tt.offset)
			vm.add(instr)

			expected := make([]uint16, RCount)
			copy(expected, tt.output)
			expected[RCond] = tt.cc
			assert.Equal(t, expected, vm.Registers)
		})
	}
}

func TestAnd(t *testing.T) {
	tests := []struct {
		name          string
		registers     []uint16
		r0, r1, r2    uint16
		immediateMode bool
		offset        uint16
		output        []uint16
		cc            uint16
	}{
		// register mode
		{
			"R0 = R1 & R2",
			[]uint16{0, 0b0110, 0b0011},
			0, 1, 2,
			false, 0,
			[]uint16{0b0010, 0b0110, 0b0011},
			CondPos,
		},
		{
			"register mode: zero result (no overlapping bits)",
			[]uint16{0, 0x0F0F, 0xF0F0},
			0, 1, 2,
			false, 0,
			[]uint16{0, 0x0F0F, 0xF0F0},
			CondZro,
		},
		{
			"register mode: negative result (top bit set in both operands)",
			[]uint16{0, 0x8001, 0x8002},
			0, 1, 2,
			false, 0,
			[]uint16{0x8000, 0x8001, 0x8002},
			CondNeg,
		},
		{
			"R0 = R1 & R1", // SR1 == SR2: ANDing a register with itself is a no-op copy
			[]uint16{0, 0x1234},
			0, 1, 1,
			false, 0,
			[]uint16{0x1234, 0x1234},
			CondPos,
		},
		{
			"R0 &= R2", // DR == SR1
			[]uint16{0xFF00, 0, 0x0FF0},
			0, 0, 2,
			false, 0,
			[]uint16{0x0F00, 0, 0x0FF0},
			CondPos,
		},
		{
			"R0 &= R1", // DR == SR2
			[]uint16{0xFF00, 0x0FF0},
			0, 1, 0,
			false, 0,
			[]uint16{0x0F00, 0x0FF0},
			CondPos,
		},
		{
			"R0 &= R0", // DR == SR1 == SR2: also a no-op, but exercises the negative path
			[]uint16{0x8001},
			0, 0, 0,
			false, 0,
			[]uint16{0x8001},
			CondNeg,
		},
		// immediate mode
		{
			"immediate mode: AND with #0 clears the register",
			[]uint16{0, 0x1234},
			0, 1, 0,
			true, 0,
			[]uint16{0, 0x1234},
			CondZro,
		},
		{
			"immediate mode: AND with #-1 preserves a positive value",
			[]uint16{0, 0x1234},
			0, 1, 0,
			true, 0x1F, // imm5 = 0b11111, sign-extends to 0xFFFF (all-ones mask)
			[]uint16{0x1234, 0x1234},
			CondPos,
		},
		{
			"immediate mode: AND with #-1 preserves a negative value",
			[]uint16{0, 0x8001},
			0, 1, 0,
			true, 0x1F,
			[]uint16{0x8001, 0x8001},
			CondNeg,
		},
		{
			"immediate mode: positive imm masks low bits",
			[]uint16{0, 0xFFFF},
			0, 1, 0,
			true, 0x0F, // imm5 = 0b01111 = +15, mask for the low nibble
			[]uint16{0x000F, 0xFFFF},
			CondPos,
		},
		{
			"immediate mode: negative imm masks high bits, nonzero result",
			[]uint16{0, 0xFFFF},
			0, 1, 0,
			true, 0x10, // imm5 = 0b10000 = -16, sign-extends to 0xFFF0 (upper-bits mask)
			[]uint16{0xFFF0, 0xFFFF},
			CondNeg,
		},
		{
			"immediate mode: negative imm masks high bits, zero result",
			[]uint16{0, 0x000F},
			0, 1, 0,
			true, 0x10, // same 0xFFF0 mask, but input has nothing in the upper bits
			[]uint16{0, 0x000F},
			CondZro,
		},
		{
			"immediate mode: DR == SR1",
			[]uint16{0xFF},
			0, 0, 0,
			true, 0x0F,
			[]uint16{0x0F},
			CondPos,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			vm := VM{Registers: make([]uint16, RCount)}
			copy(vm.Registers, tt.registers)

			instr := convertInstructionToUInt16(OpAnd, tt.r0, tt.r1, tt.r2, tt.immediateMode, tt.offset)
			vm.and(instr)

			expected := make([]uint16, RCount)
			copy(expected, tt.output)
			expected[RCond] = tt.cc
			assert.Equal(t, expected, vm.Registers)
		})
	}
}

func TestNot(t *testing.T) {
	// NOT has a single addressing mode (DR = ^SR), so the full case space is just
	// the 3 condition-code outcomes crossed with DR == SR or DR != SR.
	tests := []struct {
		name      string
		registers []uint16
		r0, r1    uint16
		output    []uint16
		cc        uint16
	}{
		{
			"R0 = NOT(R1)",
			[]uint16{0, 0x1234},
			0, 1,
			[]uint16{0xEDCB, 0x1234},
			CondNeg,
		},
		{
			"R0 = NOT(R1), result positive",
			[]uint16{0, 0x8000},
			0, 1,
			[]uint16{0x7FFF, 0x8000},
			CondPos,
		},
		{
			"R0 = NOT(R1), result zero",
			[]uint16{0, 0xFFFF},
			0, 1,
			[]uint16{0, 0xFFFF},
			CondZro,
		},
		{
			"R0 = NOT(R0)", // DR == SR, result positive
			[]uint16{0x8000},
			0, 0,
			[]uint16{0x7FFF},
			CondPos,
		},
		{
			"R0 = NOT(R0), result negative", // DR == SR, classic NOT(0) == -1
			[]uint16{0x0000},
			0, 0,
			[]uint16{0xFFFF},
			CondNeg,
		},
		{
			"R0 = NOT(R0), result zero", // DR == SR
			[]uint16{0xFFFF},
			0, 0,
			[]uint16{0},
			CondZro,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			vm := VM{Registers: make([]uint16, RCount)}
			copy(vm.Registers, tt.registers)

			instr := convertInstructionToUInt16(OpNot, tt.r0, tt.r1, 0, false, 0)
			vm.not(instr)

			expected := make([]uint16, RCount)
			copy(expected, tt.output)
			expected[RCond] = tt.cc
			assert.Equal(t, expected, vm.Registers)
		})
	}
}

func TestBr(t *testing.T) {
	// BR doesn't touch the condition codes itself, so a case is fully described by
	// which n/z/p bits are set, what CC is active going in, and where PC ends up.
	const startPC uint16 = 0x3000

	tests := []struct {
		name        string
		n, z, p     bool
		initialCond uint16
		offset      uint16 // raw 9-bit PCoffset9 pattern, before sign extension
		expectedPC  uint16
	}{
		// single flag: full taken / not-taken matrix
		{"BRn taken (CC = Neg)", true, false, false, CondNeg, 10, startPC + 10},
		{"BRn not taken (CC = Zro)", true, false, false, CondZro, 10, startPC},
		{"BRn not taken (CC = Pos)", true, false, false, CondPos, 10, startPC},
		{"BRz taken (CC = Zro)", false, true, false, CondZro, 10, startPC + 10},
		{"BRz not taken (CC = Neg)", false, true, false, CondNeg, 10, startPC},
		{"BRz not taken (CC = Pos)", false, true, false, CondPos, 10, startPC},
		{"BRp taken (CC = Pos)", false, false, true, CondPos, 10, startPC + 10},
		{"BRp not taken (CC = Neg)", false, false, true, CondNeg, 10, startPC},
		{"BRp not taken (CC = Zro)", false, false, true, CondZro, 10, startPC},

		// two flags: prove the OR from both trigger paths, plus one non-trigger
		{"BRnz taken via CC = Neg", true, true, false, CondNeg, 10, startPC + 10},
		{"BRnz taken via CC = Zro", true, true, false, CondZro, 10, startPC + 10},
		{"BRnz not taken (CC = Pos)", true, true, false, CondPos, 10, startPC},
		{"BRnp taken via CC = Neg", true, false, true, CondNeg, 10, startPC + 10},
		{"BRnp taken via CC = Pos", true, false, true, CondPos, 10, startPC + 10},
		{"BRzp taken via CC = Zro", false, true, true, CondZro, 10, startPC + 10},
		{"BRzp taken via CC = Pos", false, true, true, CondPos, 10, startPC + 10},

		// unconditional (n=z=p=1): branches no matter what CC is active
		{"BRnzp taken (CC = Neg)", true, true, true, CondNeg, 10, startPC + 10},
		{"BRnzp taken (CC = Zro)", true, true, true, CondZro, 10, startPC + 10},
		{"BRnzp taken (CC = Pos)", true, true, true, CondPos, 10, startPC + 10},

		// n=z=p=0 is a valid encoding too: never branches, regardless of CC
		{"BR with n=z=p=0 never taken", false, false, false, CondZro, 10, startPC},

		// PCoffset9 range boundaries
		{"BRnzp max positive offset (+255)", true, true, true, CondPos, 0x0FF, startPC + 255},
		{"BRnzp max negative offset (-256)", true, true, true, CondPos, 0x100, startPC - 256},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			vm := VM{Registers: make([]uint16, RCount)}
			vm.Registers[RCond] = tt.initialCond
			vm.Registers[PC] = startPC

			instr := convertBrInstructionToUInt16(tt.n, tt.z, tt.p, tt.offset)
			vm.br(instr)

			expected := make([]uint16, RCount)
			expected[RCond] = tt.initialCond
			expected[PC] = tt.expectedPC
			assert.Equal(t, expected, vm.Registers)
		})
	}
}

func TestJmp(t *testing.T) {
	// JMP just does PC = Registers[BaseR]; no condition codes involved, and BaseR
	// sits in the same bit field (8-6) as SR1 in convertInstructionToUInt16.
	tests := []struct {
		name         string
		baseReg      uint16
		baseRegValue uint16
		initialPC    uint16
		expectedPC   uint16
	}{
		{"PC = R1", 1, 0x4000, 0x3000, 0x4000},
		{"PC = R7 (RET idiom)", 7, 0x3050, 0x3060, 0x3050},
		{"PC = R0", 0, 0x3100, 0x3000, 0x3100},
		{"PC = R1, target address 0", 1, 0x0000, 0x3000, 0x0000},
		{"PC = R1, target address max (0xFFFF)", 1, 0xFFFF, 0x3000, 0xFFFF},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			vm := VM{Registers: make([]uint16, RCount)}
			vm.Registers[tt.baseReg] = tt.baseRegValue
			vm.Registers[PC] = tt.initialPC

			instr := convertInstructionToUInt16(OpJmp, 0, tt.baseReg, 0, false, 0)
			vm.jmp(instr)

			expected := make([]uint16, RCount)
			expected[tt.baseReg] = tt.baseRegValue
			expected[PC] = tt.expectedPC
			assert.Equal(t, expected, vm.Registers)
		})
	}
}

func TestJsr(t *testing.T) {
	// JSR always does R7 = PC first, then either PC = Registers[BaseR] (JSRR, bit 11
	// = 0) or PC += SEXT(PCoffset11) (JSR, bit 11 = 1). Since R7 is written before
	// it's read, JSRR R7 is a real edge case: R7 gets clobbered with the old PC
	// before that same value is read back out as the jump target.
	tests := []struct {
		name         string
		pcRelative   bool
		baseReg      uint16 // used when !pcRelative
		baseRegValue uint16 // used when !pcRelative: initial Registers[baseReg]
		offset       uint16 // used when pcRelative: raw PCoffset11 pattern
		initialPC    uint16
		initialR7    uint16 // sentinel prior R7 content, to prove it gets overwritten
		expectedPC   uint16
		expectedR7   uint16
	}{
		// JSRR (register mode)
		{"JSRR R1", false, 1, 0x4000, 0, 0x3000, 0x9999, 0x4000, 0x3000},
		{"JSRR R0", false, 0, 0x3100, 0, 0x3000, 0x9999, 0x3100, 0x3000},
		{"JSRR R7 (BaseR is R7 itself)", false, 7, 0x5000, 0, 0x3000, 0x5000, 0x3000, 0x3000},
		{"JSRR R1, target address 0", false, 1, 0x0000, 0, 0x3000, 0x9999, 0x0000, 0x3000},
		{"JSRR R1, target address max (0xFFFF)", false, 1, 0xFFFF, 0, 0x3000, 0x9999, 0xFFFF, 0x3000},

		// JSR (PC-relative mode)
		{"JSR positive offset", true, 0, 0, 10, 0x3000, 0x9999, 0x300A, 0x3000},
		{"JSR negative offset", true, 0, 0, 0x7FB, 0x3000, 0x9999, 0x2FFB, 0x3000}, // 0x7FB == -5
		{"JSR max positive offset (+1023)", true, 0, 0, 0x3FF, 0x3000, 0x9999, 0x33FF, 0x3000},
		{"JSR max negative offset (-1024)", true, 0, 0, 0x400, 0x3000, 0x9999, 0x2C00, 0x3000},
		{"JSR zero offset", true, 0, 0, 0, 0x3000, 0x9999, 0x3000, 0x3000},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			vm := VM{Registers: make([]uint16, RCount)}
			vm.Registers[PC] = tt.initialPC
			vm.Registers[R7] = tt.initialR7
			if !tt.pcRelative {
				vm.Registers[tt.baseReg] = tt.baseRegValue
			}

			var instr uint16
			if tt.pcRelative {
				instr = convertJsrInstructionToUInt16(true, tt.offset)
			} else {
				instr = convertJsrInstructionToUInt16(false, tt.baseReg)
			}
			vm.jsr(instr)

			expected := make([]uint16, RCount)
			expected[PC] = tt.expectedPC
			expected[R7] = tt.expectedR7
			if !tt.pcRelative && tt.baseReg != uint16(R7) {
				expected[tt.baseReg] = tt.baseRegValue
			}
			assert.Equal(t, expected, vm.Registers)
		})
	}
}

func TestLd(t *testing.T) {
	// LD does DR = mem[PC + SEXT(PCoffset9)] and sets the condition codes based on
	// the loaded value; it must never write to memory. Addresses below are chosen
	// well clear of 0xFE00 (MmapKBSR) - memRead treats that address specially and
	// would block on real keyboard input if a test ever landed on it.
	tests := []struct {
		name       string
		dr         uint16
		initialPC  uint16
		offset     uint16 // raw 9-bit PCoffset9 pattern
		memAddr    uint16
		memValue   uint16
		expectedCC uint16
	}{
		{"LD positive offset, positive value", 3, 0x3000, 10, 0x300A, 0x0042, CondPos},
		{"LD positive offset, zero value", 0, 0x3000, 10, 0x300A, 0x0000, CondZro},
		{"LD positive offset, negative value", 0, 0x3000, 10, 0x300A, 0x8000, CondNeg},
		{"LD negative offset (backward reference)", 0, 0x3000, 0x1FF, 0x2FFF, 0x1234, CondPos}, // 0x1FF == -1
		{"LD zero offset", 0, 0x3000, 0, 0x3000, 0x5555, CondPos},
		{"LD max positive offset (+255)", 0, 0x3000, 0x0FF, 0x30FF, 0x00FF, CondPos},
		{"LD max negative offset (-256)", 0, 0x3000, 0x100, 0x2F00, 0x0001, CondPos},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			vm := VM{
				Registers: make([]uint16, RCount),
				Memory:    make([]uint16, MemoryMax),
			}
			vm.Registers[PC] = tt.initialPC
			vm.Memory[tt.memAddr] = tt.memValue

			memSnapshot := make([]uint16, MemoryMax)
			copy(memSnapshot, vm.Memory)

			instr := convertDrPCOffset9InstructionToUInt16(OpLd, tt.dr, tt.offset)
			vm.ld(instr)

			expected := make([]uint16, RCount)
			expected[PC] = tt.initialPC
			expected[tt.dr] = tt.memValue
			expected[RCond] = tt.expectedCC
			assert.Equal(t, expected, vm.Registers)
			assert.Equal(t, memSnapshot, vm.Memory, "LD must not modify memory")
		})
	}
}

func TestLdi(t *testing.T) {
	// LDI does DR = mem[mem[PC + SEXT(PCoffset9)]] - double indirection, and must
	// never write to memory. Both the pointer location and the address it points
	// to are kept well clear of 0xFE00 (MmapKBSR) for the same reason as TestLd.
	tests := []struct {
		name       string
		dr         uint16
		initialPC  uint16
		offset     uint16 // raw 9-bit PCoffset9 pattern
		ptrAddr    uint16 // = initialPC + SEXT(offset); holds the pointer
		ptrValue   uint16 // the pointer itself: the address the value lives at
		finalValue uint16 // the value at ptrValue, what actually lands in DR
		expectedCC uint16
	}{
		{"LDI positive offset, positive value", 3, 0x3000, 10, 0x300A, 0x4000, 0x0042, CondPos},
		{"LDI positive offset, zero value", 0, 0x3000, 10, 0x300A, 0x4000, 0x0000, CondZro},
		{"LDI positive offset, negative value", 0, 0x3000, 10, 0x300A, 0x4000, 0x8000, CondNeg},
		{"LDI negative offset (backward reference)", 0, 0x3000, 0x1FF, 0x2FFF, 0x4000, 0x1234, CondPos}, // 0x1FF == -1
		{"LDI zero offset", 0, 0x3000, 0, 0x3000, 0x4000, 0x5555, CondPos},
		{"LDI max positive offset (+255)", 0, 0x3000, 0x0FF, 0x30FF, 0x4000, 0x00FF, CondPos},
		{"LDI max negative offset (-256)", 0, 0x3000, 0x100, 0x2F00, 0x4000, 0x0001, CondPos},
		{"LDI pointer targets address 0", 0, 0x3000, 10, 0x300A, 0x0000, 0x2222, CondPos},
		{"LDI pointer targets address 0xFFFF", 0, 0x3000, 10, 0x300A, 0xFFFF, 0x7777, CondPos},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			vm := VM{
				Registers: make([]uint16, RCount),
				Memory:    make([]uint16, MemoryMax),
			}
			vm.Registers[PC] = tt.initialPC
			vm.Memory[tt.ptrAddr] = tt.ptrValue
			vm.Memory[tt.ptrValue] = tt.finalValue

			memSnapshot := make([]uint16, MemoryMax)
			copy(memSnapshot, vm.Memory)

			instr := convertDrPCOffset9InstructionToUInt16(OpLdi, tt.dr, tt.offset)
			vm.ldi(instr)

			expected := make([]uint16, RCount)
			expected[PC] = tt.initialPC
			expected[tt.dr] = tt.finalValue
			expected[RCond] = tt.expectedCC
			assert.Equal(t, expected, vm.Registers)
			assert.Equal(t, memSnapshot, vm.Memory, "LDI must not modify memory")
		})
	}
}
