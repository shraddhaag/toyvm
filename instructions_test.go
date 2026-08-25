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
