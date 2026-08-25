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
