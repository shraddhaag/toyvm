package main

import (
	"slices"
	"testing"

	"hegel.dev/go/hegel"
)

var genRegIndex = hegel.Integers[uint16](0, 7)

var genWord = hegel.Integers[uint16](0, 0xFFFF)

var genCondFlag = hegel.SampledFrom([]uint16{CondNeg, CondZro, CondPos})

var genRegContents = hegel.Composite(func(tc hegel.TestCase) []uint16 {
	registers := make([]uint16, RCount)
	for r := R0; r <= PC; r++ {
		registers[r] = hegel.Draw(tc, genWord)
	}
	registers[RCond] = hegel.Draw(tc, genCondFlag)
	return registers
})

// Zeroed memory with a few random cells: reading MmapKBSR waits for a key press,
// so tests must keep the addresses they read away from it.
var genMemory = hegel.Composite(func(tc hegel.TestCase) []uint16 {
	memory := make([]uint16, MemoryMax)
	for address, value := range hegel.Draw(tc, hegel.Maps(genWord, genWord).MaxSize(8)) {
		memory[address] = value
	}
	return memory
})

var genVM = hegel.Composite(func(tc hegel.TestCase) *VM {
	return &VM{
		Registers: hegel.Draw(tc, genRegContents),
		Memory:    hegel.Draw(tc, genMemory),
		Executing: Running,
	}
})

func wantCC(v uint16) uint16 {
	switch {
	case v == 0:
		return CondZro
	case int16(v) < 0:
		return CondNeg
	default:
		return CondPos
	}
}

func TestAddProperty(t *testing.T) {
	hegel.Test(t, func(ht *hegel.T) {
		vm := &VM{Registers: hegel.Draw(ht, genRegContents)}
		dr := hegel.Draw(ht, genRegIndex)
		sr1 := hegel.Draw(ht, genRegIndex)

		a := int16(vm.Registers[sr1])
		var b int16
		var instr uint16

		if hegel.Draw(ht, hegel.Booleans()) {
			b = hegel.Draw(ht, hegel.Integers[int16](-16, 15))
			instr = convertInstructionToUInt16(OpAdd, dr, sr1, 0, true, uint16(b))
		} else {
			sr2 := hegel.Draw(ht, genRegIndex)
			b = int16(vm.Registers[sr2])
			instr = convertInstructionToUInt16(OpAdd, dr, sr1, sr2, false, 0)
		}

		before := slices.Clone(vm.Registers)
		vm.add(instr)

		want := uint16(a + b)
		// Property 1: Destination Register should have the expected sum
		if got := vm.Registers[dr]; got != want {
			ht.Fatalf("ADD R%d: %d + %d gave %#04x, want %#04x", dr, a, b, got, want)
		}
		// Property 2: Conditional Flag is set appropriately as per the sum
		if got := vm.Registers[RCond]; got != wantCC(want) {
			ht.Fatalf("CC for %#04x: got %03b, want %03b", want, got, wantCC(want))
		}
		// Apart from dsestination register and condition flag, rest of the registers are unmodified
		for r := R0; r < RCount; r++ {
			if r != Register(dr) && r != RCond && vm.Registers[r] != before[r] {
				ht.Fatalf("register %d changed: %#04x -> %#04x", r, before[r], vm.Registers[r])
			}
		}
	})
}
