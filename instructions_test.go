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
	vm := VM{
		Registers: make([]uint16, RCount),
	}

	tests := []struct {
		name          string
		registers     []uint16
		r0, r1, r2    uint16
		immediateMode bool
		offset        uint16
		output        []uint16
	}{
		{
			"basic add",
			[]uint16{1, 2, 3},
			0, 1, 2,
			false, 0,
			[]uint16{5, 2, 3},
		}, {
			"subtract offset",
			[]uint16{16384},
			0, 0, 0,
			true, 29,
			[]uint16{16381},
		},
	}

	for _, tt := range tests {
		regs := make([]uint16, RCount)
		copy(regs, tt.registers)
		vm.Registers = regs

		instr := convertInstructionToUInt16(OpAdd, tt.r0, tt.r1, tt.r2, tt.immediateMode, tt.offset)
		vm.add(instr)

		expected := make([]uint16, RCount)
		copy(expected, tt.output)
		expected[RCond] = CondPos // result is positive
		assert.Equal(t, expected, vm.Registers)
	}
}
