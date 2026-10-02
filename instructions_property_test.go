package main

import (
	"io"
	"log/slog"
	"os"
	"slices"
	"strings"
	"testing"

	"hegel.dev/go/hegel"
)

// trapHalt logs at Info level, which would print once per generated case.
func TestMain(m *testing.M) {
	slog.SetLogLoggerLevel(slog.LevelWarn)
	os.Exit(m.Run())
}

// CI environments derandomize by default. To catch test failures,
// increase number of test cases per run.
var propertyOpts = []hegel.Option{hegel.WithTestCases(300)}

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

// Fails if any register outside allowed changed.
func assertOnlyChanged(ht *hegel.T, before, after []uint16, allowed ...Register) {
	for r := R0; r < RCount; r++ {
		if slices.Contains(allowed, r) {
			continue
		}
		if before[r] != after[r] {
			ht.Fatalf("register %d changed: %#04x -> %#04x", r, before[r], after[r])
		}
	}
}

// Fails at the first memory cell that does not hold the expected value.
func assertMemory(ht *hegel.T, want, got []uint16) {
	for address, value := range want {
		if got[address] != value {
			ht.Fatalf("memory[%#04x] = %#04x, want %#04x", address, got[address], value)
		}
	}
}

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

func TestPropertyAdd(t *testing.T) {
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
		// Property 3: no other register is modified
		assertOnlyChanged(ht, before, vm.Registers, Register(dr), RCond)
	}, propertyOpts...)
}

func TestPropertyAnd(t *testing.T) {
	hegel.Test(t, func(ht *hegel.T) {
		vm := &VM{Registers: hegel.Draw(ht, genRegContents)}
		dr := hegel.Draw(ht, genRegIndex)
		sr1 := hegel.Draw(ht, genRegIndex)

		a := vm.Registers[sr1]
		var b, instr uint16

		if hegel.Draw(ht, hegel.Booleans()) {
			imm := hegel.Draw(ht, hegel.Integers[int16](-16, 15))
			b = uint16(imm)
			instr = convertInstructionToUInt16(OpAnd, dr, sr1, 0, true, b)
		} else {
			sr2 := hegel.Draw(ht, genRegIndex)
			b = vm.Registers[sr2]
			instr = convertInstructionToUInt16(OpAnd, dr, sr1, sr2, false, 0)
		}

		before := slices.Clone(vm.Registers)
		vm.and(instr)

		want := a & b
		// Property 1: Destination Register holds the bitwise AND
		if got := vm.Registers[dr]; got != want {
			ht.Fatalf("AND R%d: %#04x & %#04x gave %#04x, want %#04x", dr, a, b, got, want)
		}
		// Property 2: Conditional Flag is set as per the result
		if got := vm.Registers[RCond]; got != wantCC(want) {
			ht.Fatalf("CC for %#04x: got %03b, want %03b", want, got, wantCC(want))
		}
		// Property 3: no other register is modified
		assertOnlyChanged(ht, before, vm.Registers, Register(dr), RCond)
	}, propertyOpts...)
}

func TestPropertyNot(t *testing.T) {
	hegel.Test(t, func(ht *hegel.T) {
		vm := &VM{Registers: hegel.Draw(ht, genRegContents)}
		dr := hegel.Draw(ht, genRegIndex)
		sr := hegel.Draw(ht, genRegIndex)

		a := vm.Registers[sr]
		before := slices.Clone(vm.Registers)
		vm.not(convertInstructionToUInt16(OpNot, dr, sr, 0, false, 0))

		want := ^a
		// Property 1: Destination Register holds every bit of the source, flipped
		if got := vm.Registers[dr]; got != want {
			ht.Fatalf("NOT R%d: ^%#04x gave %#04x, want %#04x", dr, a, got, want)
		}
		// Property 2: Conditional Flag is set as per the result
		if got := vm.Registers[RCond]; got != wantCC(want) {
			ht.Fatalf("CC for %#04x: got %03b, want %03b", want, got, wantCC(want))
		}
		// Property 3: no other register is modified
		assertOnlyChanged(ht, before, vm.Registers, Register(dr), RCond)
	}, propertyOpts...)
}

func TestPropertyBr(t *testing.T) {
	hegel.Test(t, func(ht *hegel.T) {
		vm := &VM{Registers: hegel.Draw(ht, genRegContents)}
		n := hegel.Draw(ht, hegel.Booleans())
		z := hegel.Draw(ht, hegel.Booleans())
		p := hegel.Draw(ht, hegel.Booleans())
		offset := hegel.Draw(ht, hegel.Integers[int16](-256, 255))

		cc := vm.Registers[RCond]
		pc := vm.Registers[PC]
		before := slices.Clone(vm.Registers)

		vm.br(convertBrInstructionToUInt16(n, z, p, uint16(offset)))

		want := pc
		if (n && cc == CondNeg) || (z && cc == CondZro) || (p && cc == CondPos) {
			want = pc + uint16(offset)
		}
		// Property 1: PC moves by the offset only when a set flag matches the condition code
		if got := vm.Registers[PC]; got != want {
			ht.Fatalf("BR n=%v z=%v p=%v cc=%03b offset=%d: PC %#04x, want %#04x",
				n, z, p, cc, offset, got, want)
		}
		// Property 2: no other register is modified, not even the condition code
		assertOnlyChanged(ht, before, vm.Registers, PC)
	}, propertyOpts...)
}

func TestPropertyJmp(t *testing.T) {
	hegel.Test(t, func(ht *hegel.T) {
		vm := &VM{Registers: hegel.Draw(ht, genRegContents)}
		base := hegel.Draw(ht, genRegIndex)

		want := vm.Registers[base]
		before := slices.Clone(vm.Registers)
		vm.jmp(convertInstructionToUInt16(OpJmp, 0, base, 0, false, 0))

		// Property 1: PC holds the address in the base register
		if got := vm.Registers[PC]; got != want {
			ht.Fatalf("JMP R%d: PC %#04x, want %#04x", base, got, want)
		}
		// Property 2: no other register is modified
		assertOnlyChanged(ht, before, vm.Registers, PC)
	}, propertyOpts...)
}

func TestPropertyJsr(t *testing.T) {
	hegel.Test(t, func(ht *hegel.T) {
		vm := &VM{Registers: hegel.Draw(ht, genRegContents)}
		offset := hegel.Draw(ht, hegel.Integers[int16](-1024, 1023))

		pc := vm.Registers[PC]
		before := slices.Clone(vm.Registers)
		vm.jsr(convertJsrInstructionToUInt16(true, uint16(offset)))

		// Property 1: PC moves by the offset
		if want, got := pc+uint16(offset), vm.Registers[PC]; got != want {
			ht.Fatalf("JSR offset %d: PC %#04x, want %#04x", offset, got, want)
		}
		// Property 2: R7 holds the return address
		if got := vm.Registers[R7]; got != pc {
			ht.Fatalf("JSR: R7 %#04x, want old PC %#04x", got, pc)
		}
		// Property 3: no other register is modified
		assertOnlyChanged(ht, before, vm.Registers, PC, R7)
	}, propertyOpts...)
}

func TestPropertyJsrr(t *testing.T) {
	hegel.Test(t, func(ht *hegel.T) {
		vm := &VM{Registers: hegel.Draw(ht, genRegContents)}
		base := hegel.Draw(ht, genRegIndex)

		pc := vm.Registers[PC]
		want := vm.Registers[base]
		before := slices.Clone(vm.Registers)
		vm.jsr(convertJsrInstructionToUInt16(false, base))

		// Property 1: PC holds the address in the base register
		if got := vm.Registers[PC]; got != want {
			ht.Fatalf("JSRR R%d: PC %#04x, want %#04x", base, got, want)
		}
		// Property 2: R7 holds the return address
		if got := vm.Registers[R7]; got != pc {
			ht.Fatalf("JSRR R%d: R7 %#04x, want old PC %#04x", base, got, pc)
		}
		// Property 3: no other register is modified
		assertOnlyChanged(ht, before, vm.Registers, PC, R7)
	}, propertyOpts...)
}

func TestPropertyLd(t *testing.T) {
	hegel.Test(t, func(ht *hegel.T) {
		vm := hegel.Draw(ht, genVM)
		dr := hegel.Draw(ht, genRegIndex)
		offset := hegel.Draw(ht, hegel.Integers[int16](-256, 255))

		address := vm.Registers[PC] + uint16(offset)
		ht.Assume(address != MmapKBSR)

		want := vm.Memory[address]
		beforeRegisters := slices.Clone(vm.Registers)
		beforeMemory := slices.Clone(vm.Memory)

		vm.ld(convertDrPCOffset9InstructionToUInt16(OpLd, dr, uint16(offset)))

		// Property 1: Destination Register holds the word at PC + offset
		if got := vm.Registers[dr]; got != want {
			ht.Fatalf("LD R%d from %#04x: got %#04x, want %#04x", dr, address, got, want)
		}
		// Property 2: Conditional Flag is set as per the loaded value
		if got := vm.Registers[RCond]; got != wantCC(want) {
			ht.Fatalf("CC for %#04x: got %03b, want %03b", want, got, wantCC(want))
		}
		// Property 3: no other register is modified
		assertOnlyChanged(ht, beforeRegisters, vm.Registers, Register(dr), RCond)
		// Property 4: a load never writes to memory
		assertMemory(ht, beforeMemory, vm.Memory)
	}, propertyOpts...)
}

func TestPropertyLdi(t *testing.T) {
	hegel.Test(t, func(ht *hegel.T) {
		vm := hegel.Draw(ht, genVM)
		dr := hegel.Draw(ht, genRegIndex)
		offset := hegel.Draw(ht, hegel.Integers[int16](-256, 255))

		pointerAddress := vm.Registers[PC] + uint16(offset)
		ht.Assume(pointerAddress != MmapKBSR)
		address := vm.Memory[pointerAddress]
		ht.Assume(address != MmapKBSR)

		want := vm.Memory[address]
		beforeRegisters := slices.Clone(vm.Registers)
		beforeMemory := slices.Clone(vm.Memory)

		vm.ldi(convertDrPCOffset9InstructionToUInt16(OpLdi, dr, uint16(offset)))

		// Property 1: Destination Register holds the word the pointer points at
		if got := vm.Registers[dr]; got != want {
			ht.Fatalf("LDI R%d via %#04x -> %#04x: got %#04x, want %#04x",
				dr, pointerAddress, address, got, want)
		}
		// Property 2: Conditional Flag is set as per the loaded value
		if got := vm.Registers[RCond]; got != wantCC(want) {
			ht.Fatalf("CC for %#04x: got %03b, want %03b", want, got, wantCC(want))
		}
		// Property 3: no other register is modified
		assertOnlyChanged(ht, beforeRegisters, vm.Registers, Register(dr), RCond)
		// Property 4: a load never writes to memory
		assertMemory(ht, beforeMemory, vm.Memory)
	}, propertyOpts...)
}

func TestPropertyLdr(t *testing.T) {
	hegel.Test(t, func(ht *hegel.T) {
		vm := hegel.Draw(ht, genVM)
		dr := hegel.Draw(ht, genRegIndex)
		base := hegel.Draw(ht, genRegIndex)
		offset := hegel.Draw(ht, hegel.Integers[int16](-32, 31))

		address := vm.Registers[base] + uint16(offset)
		ht.Assume(address != MmapKBSR)

		want := vm.Memory[address]
		beforeRegisters := slices.Clone(vm.Registers)
		beforeMemory := slices.Clone(vm.Memory)

		vm.ldr(convertDrBaseROffset6InstructionToUInt16(OpLdr, dr, base, uint16(offset)))

		// Property 1: Destination Register holds the word at BaseR + offset
		if got := vm.Registers[dr]; got != want {
			ht.Fatalf("LDR R%d from R%d%+d (%#04x): got %#04x, want %#04x",
				dr, base, offset, address, got, want)
		}
		// Property 2: Conditional Flag is set as per the loaded value
		if got := vm.Registers[RCond]; got != wantCC(want) {
			ht.Fatalf("CC for %#04x: got %03b, want %03b", want, got, wantCC(want))
		}
		// Property 3: no other register is modified
		assertOnlyChanged(ht, beforeRegisters, vm.Registers, Register(dr), RCond)
		// Property 4: a load never writes to memory
		assertMemory(ht, beforeMemory, vm.Memory)
	}, propertyOpts...)
}

func TestPropertyLea(t *testing.T) {
	hegel.Test(t, func(ht *hegel.T) {
		// Memory stays nil: LEA computes an address and must never read memory.
		vm := &VM{Registers: hegel.Draw(ht, genRegContents)}
		dr := hegel.Draw(ht, genRegIndex)
		offset := hegel.Draw(ht, hegel.Integers[int16](-256, 255))

		want := vm.Registers[PC] + uint16(offset)
		before := slices.Clone(vm.Registers)
		vm.lea(convertDrPCOffset9InstructionToUInt16(OpLea, dr, uint16(offset)))

		// Property 1: Destination Register holds the address PC + offset
		if got := vm.Registers[dr]; got != want {
			ht.Fatalf("LEA R%d offset %d: got %#04x, want %#04x", dr, offset, got, want)
		}
		// Property 2: Conditional Flag is set as per the computed address
		if got := vm.Registers[RCond]; got != wantCC(want) {
			ht.Fatalf("CC for %#04x: got %03b, want %03b", want, got, wantCC(want))
		}
		// Property 3: no other register is modified
		assertOnlyChanged(ht, before, vm.Registers, Register(dr), RCond)
	}, propertyOpts...)
}

func TestPropertySt(t *testing.T) {
	hegel.Test(t, func(ht *hegel.T) {
		vm := hegel.Draw(ht, genVM)
		sr := hegel.Draw(ht, genRegIndex)
		offset := hegel.Draw(ht, hegel.Integers[int16](-256, 255))

		address := vm.Registers[PC] + uint16(offset)
		beforeRegisters := slices.Clone(vm.Registers)
		wantMemory := slices.Clone(vm.Memory)
		wantMemory[address] = vm.Registers[sr]

		vm.st(convertDrPCOffset9InstructionToUInt16(OpSt, sr, uint16(offset)))

		// Property 1: the word goes to PC + offset, and no other cell changes
		assertMemory(ht, wantMemory, vm.Memory)
		// Property 2: a store modifies no register, not even the condition code
		assertOnlyChanged(ht, beforeRegisters, vm.Registers)
	}, propertyOpts...)
}

func TestPropertySti(t *testing.T) {
	hegel.Test(t, func(ht *hegel.T) {
		vm := hegel.Draw(ht, genVM)
		sr := hegel.Draw(ht, genRegIndex)
		offset := hegel.Draw(ht, hegel.Integers[int16](-256, 255))

		pointerAddress := vm.Registers[PC] + uint16(offset)
		ht.Assume(pointerAddress != MmapKBSR)

		beforeRegisters := slices.Clone(vm.Registers)
		wantMemory := slices.Clone(vm.Memory)
		wantMemory[vm.Memory[pointerAddress]] = vm.Registers[sr]

		vm.sti(convertDrPCOffset9InstructionToUInt16(OpSti, sr, uint16(offset)))

		// Property 1: the word goes to the address the pointer holds, and no other cell changes
		assertMemory(ht, wantMemory, vm.Memory)
		// Property 2: a store modifies no register, not even the condition code
		assertOnlyChanged(ht, beforeRegisters, vm.Registers)
	}, propertyOpts...)
}

func TestPropertyStr(t *testing.T) {
	hegel.Test(t, func(ht *hegel.T) {
		vm := hegel.Draw(ht, genVM)
		sr := hegel.Draw(ht, genRegIndex)
		base := hegel.Draw(ht, genRegIndex)
		offset := hegel.Draw(ht, hegel.Integers[int16](-32, 31))

		address := vm.Registers[base] + uint16(offset)
		beforeRegisters := slices.Clone(vm.Registers)
		wantMemory := slices.Clone(vm.Memory)
		wantMemory[address] = vm.Registers[sr]

		vm.str(convertDrBaseROffset6InstructionToUInt16(OpStr, sr, base, uint16(offset)))

		// Property 1: the word goes to BaseR + offset, and no other cell changes
		assertMemory(ht, wantMemory, vm.Memory)
		// Property 2: a store modifies no register, not even the condition code
		assertOnlyChanged(ht, beforeRegisters, vm.Registers)
	}, propertyOpts...)
}

func TestPropertyRtiAndRes(t *testing.T) {
	hegel.Test(t, func(ht *hegel.T) {
		vm := hegel.Draw(ht, genVM)
		instr := hegel.Draw(ht, genWord)

		beforeRegisters := slices.Clone(vm.Registers)
		beforeMemory := slices.Clone(vm.Memory)

		vm.rti(instr)
		vm.res(instr)

		// Property 1: the unused opcodes change no register and no memory cell
		assertOnlyChanged(ht, beforeRegisters, vm.Registers)
		assertMemory(ht, beforeMemory, vm.Memory)
	}, propertyOpts...)
}

// The trap handlers read os.Stdin and write os.Stdout on every call, so a test
// can put temp files in their place for the duration of one call. Temp files,
// not pipes, so a long write can never block on a full buffer. hegel runs the
// cases one at a time, so these tests must not call t.Parallel.
func withStdio(ht *hegel.T, input []byte, fn func()) string {
	dir := ht.TempDir()
	in, err := os.CreateTemp(dir, "stdin")
	if err != nil {
		ht.Fatalf("create stdin: %v", err)
	}
	defer in.Close()
	out, err := os.CreateTemp(dir, "stdout")
	if err != nil {
		ht.Fatalf("create stdout: %v", err)
	}
	defer out.Close()
	in.Write(input)
	in.Seek(0, io.SeekStart)

	oldIn, oldOut := os.Stdin, os.Stdout
	os.Stdin, os.Stdout = in, out
	func() {
		defer func() { os.Stdin, os.Stdout = oldIn, oldOut }()
		fn()
	}()

	out.Seek(0, io.SeekStart)
	printed, _ := io.ReadAll(out)
	return string(printed)
}

func trapInstr(vector uint16) uint16 { return uint16(OpTrap)<<12 | vector&0xFF }

// ASCII text with no NUL, since x0000 terminates a string in memory.
func genASCII(minLen, maxLen int) hegel.Generator[[]byte] {
	return hegel.Lists(hegel.Integers[byte](0x01, 0x7F)).MinSize(minLen).MaxSize(maxLen)
}

// placeString picks a start address so that the string and its terminator
// neither wrap past xFFFF nor cover MmapKBSR, which PUTS would read.
func placeString(ht *hegel.T, words int) uint16 {
	start := hegel.Draw(ht, genWord)
	end := int(start) + words // address of the terminator
	ht.Assume(end <= 0xFFFF)
	ht.Assume(end < MmapKBSR || int(start) > MmapKBSR)
	return start
}

func TestPropertyTrapUnknownVector(t *testing.T) {
	hegel.Test(t, func(ht *hegel.T) {
		vm := hegel.Draw(ht, genVM)
		vector := hegel.Draw(ht, hegel.Filter(hegel.Integers[uint16](0, 0xFF),
			func(v uint16) bool { return v < TrapGetC || v > TrapHalt }))

		before := slices.Clone(vm.Registers)
		printed := withStdio(ht, nil, func() { vm.trap(trapInstr(vector)) })

		// Property 1: R7 holds the return address
		if got := vm.Registers[R7]; got != before[PC] {
			ht.Fatalf("R7 %#04x, want old PC %#04x", got, before[PC])
		}
		// Property 2: no other register is modified
		assertOnlyChanged(ht, before, vm.Registers, R7)
		// Property 3: an unimplemented vector prints nothing and keeps the VM running
		if printed != "" || vm.Executing != Running {
			ht.Fatalf("vector %#02x printed %q, executing=%v", vector, printed, vm.Executing)
		}
	}, propertyOpts...)
}

func TestPropertyTrapPuts(t *testing.T) {
	hegel.Test(t, func(ht *hegel.T) {
		vm := hegel.Draw(ht, genVM)
		text := hegel.Draw(ht, genASCII(0, 40))
		start := placeString(ht, len(text))
		for i, c := range text {
			vm.Memory[start+uint16(i)] = uint16(c)
		}
		vm.Memory[start+uint16(len(text))] = 0
		vm.Registers[R0] = start

		before := slices.Clone(vm.Registers)
		beforeMemory := slices.Clone(vm.Memory)
		printed := withStdio(ht, nil, func() { vm.trap(trapInstr(TrapPutS)) })

		// Property 1: every word up to the terminator is printed as one character
		if printed != string(text) {
			ht.Fatalf("PUTS at %#04x printed %q, want %q", start, printed, text)
		}
		// Property 2: only the return address register is modified
		assertOnlyChanged(ht, before, vm.Registers, R7)
		// Property 3: printing never writes to memory
		assertMemory(ht, beforeMemory, vm.Memory)
	}, propertyOpts...)
}

// Metamorphic: PUTSP on a packed string prints the same text as PUTS on the
// same string with one character per word.
func TestPropertyTrapPutsp(t *testing.T) {
	hegel.Test(t, func(ht *hegel.T) {
		vm := hegel.Draw(ht, genVM)
		text := hegel.Draw(ht, genASCII(0, 40))
		words := (len(text) + 1) / 2
		start := placeString(ht, words)
		for i := range words {
			vm.Memory[start+uint16(i)] = 0
		}
		for i, c := range text {
			vm.Memory[start+uint16(i/2)] |= uint16(c) << (8 * (i % 2))
		}
		vm.Memory[start+uint16(words)] = 0
		vm.Registers[R0] = start

		printed := withStdio(ht, nil, func() { vm.trap(trapInstr(TrapPutSP)) })

		// Property 1: the low byte of each word is printed first, then the high byte
		if printed != string(text) {
			ht.Fatalf("PUTSP at %#04x printed %q, want %q", start, printed, text)
		}
	}, propertyOpts...)
}

func TestPropertyTrapOut(t *testing.T) {
	hegel.Test(t, func(ht *hegel.T) {
		vm := hegel.Draw(ht, genVM)
		r0 := vm.Registers[R0]

		before := slices.Clone(vm.Registers)
		printed := withStdio(ht, nil, func() { vm.trap(trapInstr(TrapOut)) })

		// Property 1: OUT writes the low byte of R0, and nothing else
		if want := string([]byte{byte(r0)}); printed != want {
			ht.Fatalf("OUT with R0=%#04x printed %q, want %q", r0, printed, want)
		}
		// Property 2: only the return address register is modified
		assertOnlyChanged(ht, before, vm.Registers, R7)
	}, propertyOpts...)
}

func TestPropertyTrapGetc(t *testing.T) {
	hegel.Test(t, func(ht *hegel.T) {
		vm := hegel.Draw(ht, genVM)
		input := hegel.Draw(ht, genASCII(1, 20))

		before := slices.Clone(vm.Registers)
		printed := withStdio(ht, input, func() { vm.trap(trapInstr(TrapGetC)) })

		// Property 1: R0 holds the first character of the input
		if got := vm.Registers[R0]; got != uint16(input[0]) {
			ht.Fatalf("GETC with input %q: R0 %#04x, want %#04x", input, got, input[0])
		}
		// Property 2: Conditional Flag is set as per the character
		if got := vm.Registers[RCond]; got != wantCC(uint16(input[0])) {
			ht.Fatalf("CC for %#04x: got %03b, want %03b", input[0], got, wantCC(uint16(input[0])))
		}
		// Property 3: no register apart from R0, R7 and the flag is modified
		assertOnlyChanged(ht, before, vm.Registers, R0, R7, RCond)
		// Property 4: GETC does not echo the character
		if printed != "" {
			ht.Fatalf("GETC printed %q, want nothing", printed)
		}
	}, propertyOpts...)
}

// Two GETC calls must consume two consecutive characters of the input.
func TestPropertyTrapGetcTwice(t *testing.T) {
	hegel.Test(t, func(ht *hegel.T) {
		vm := hegel.Draw(ht, genVM)
		input := hegel.Draw(ht, genASCII(2, 20))

		var first, second uint16
		withStdio(ht, input, func() {
			vm.trap(trapInstr(TrapGetC))
			first = vm.Registers[R0]
			vm.trap(trapInstr(TrapGetC))
			second = vm.Registers[R0]
		})

		// Property 1: the second call reads the character after the first one
		if first != uint16(input[0]) || second != uint16(input[1]) {
			ht.Fatalf("GETC twice with input %q: got %#02x %#02x, want %#02x %#02x",
				input, first, second, input[0], input[1])
		}
	}, propertyOpts...)
}

func TestPropertyTrapIn(t *testing.T) {
	hegel.Test(t, func(ht *hegel.T) {
		vm := hegel.Draw(ht, genVM)
		input := hegel.Draw(ht, genASCII(1, 20))

		before := slices.Clone(vm.Registers)
		printed := withStdio(ht, input, func() { vm.trap(trapInstr(TrapIn)) })

		// Property 1: R0 holds the first character of the input
		if got := vm.Registers[R0]; got != uint16(input[0]) {
			ht.Fatalf("IN with input %q: R0 %#04x, want %#04x", input, got, input[0])
		}
		// Property 2: Conditional Flag is set as per the character
		if got := vm.Registers[RCond]; got != wantCC(uint16(input[0])) {
			ht.Fatalf("CC for %#04x: got %03b, want %03b", input[0], got, wantCC(uint16(input[0])))
		}
		// Property 3: no register apart from R0, R7 and the flag is modified
		assertOnlyChanged(ht, before, vm.Registers, R0, R7, RCond)
		// Property 4: IN prompts first, then echoes the character it read
		if !strings.HasSuffix(printed, string(rune(input[0]))) {
			ht.Fatalf("IN printed %q, want it to end with %q", printed, input[0])
		}
	}, propertyOpts...)
}

func TestPropertyTrapHalt(t *testing.T) {
	hegel.Test(t, func(ht *hegel.T) {
		vm := hegel.Draw(ht, genVM)

		before := slices.Clone(vm.Registers)
		withStdio(ht, nil, func() { vm.trap(trapInstr(TrapHalt)) })

		// Property 1: the VM stops
		if vm.Executing != Halted {
			ht.Fatal("HALT did not stop the VM")
		}
		// Property 2: only the return address register is modified
		assertOnlyChanged(ht, before, vm.Registers, R7)
	}, propertyOpts...)
}
