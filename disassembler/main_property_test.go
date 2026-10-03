package main

import (
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"hegel.dev/go/hegel"
)

// CI environments derandomize by default. To catch test failures,
// increase number of test cases per run.
var propertyOpts = []hegel.Option{hegel.WithTestCases(300)}

var genWord = hegel.Integers[uint16](0, 0xFFFF)

var genReg = hegel.Integers[uint16](0, 7)

func field(max uint16) hegel.Generator[uint16] {
	return hegel.Integers[uint16](0, max)
}

var trapVectors = map[uint16]string{
	0x20: "GETC", 0x21: "OUT", 0x22: "PUTS", 0x23: "IN", 0x24: "PUTSP", 0x25: "HALT",
}

var mnemonics = map[uint16]string{
	0: "BR", 1: "ADD", 2: "LD", 3: "ST", 4: "JSR", 5: "AND", 6: "LDR", 7: "STR",
	8: "RTI", 9: "NOT", 10: "LDI", 11: "STI", 12: "JMP", 13: ".FILL", 14: "LEA", 15: "TRAP",
}

func wantMnemonic(instr uint16) string {
	if instr>>12 == 4 && instr&(1<<11) == 0 {
		return "JSRR"
	}
	return mnemonics[instr>>12]
}

// Canonical LC-3 words: every field the ISA defines is drawn at random, and every
// bit the ISA fixes (unused or required to be 1) holds its canonical value.
// Reserved opcode 13 words are data, so all 12 low bits are random.
var genCanonicalInstruction = hegel.Composite(func(tc hegel.TestCase) uint16 {
	op := hegel.Draw(tc, field(0xF))
	instr := op << 12
	switch op {
	case 0: // BR: nzp | PCoffset9
		instr |= hegel.Draw(tc, field(0x7))<<9 | hegel.Draw(tc, field(0x1FF))
	case 1, 5: // ADD, AND: DR | SR1 | (0 00 SR2 | 1 imm5)
		instr |= hegel.Draw(tc, genReg)<<9 | hegel.Draw(tc, genReg)<<6
		if hegel.Draw(tc, hegel.Booleans()) {
			instr |= 1<<5 | hegel.Draw(tc, field(0x1F))
		} else {
			instr |= hegel.Draw(tc, genReg)
		}
	case 2, 3, 10, 11, 14: // LD, ST, LDI, STI, LEA: DR/SR | PCoffset9
		instr |= hegel.Draw(tc, genReg)<<9 | hegel.Draw(tc, field(0x1FF))
	case 4: // JSR: 1 | PCoffset11, JSRR: 0 00 BaseR 000000
		if hegel.Draw(tc, hegel.Booleans()) {
			instr |= 1<<11 | hegel.Draw(tc, field(0x7FF))
		} else {
			instr |= hegel.Draw(tc, genReg) << 6
		}
	case 6, 7: // LDR, STR: DR/SR | BaseR | offset6
		instr |= hegel.Draw(tc, genReg)<<9 | hegel.Draw(tc, genReg)<<6 | hegel.Draw(tc, field(0x3F))
	case 8: // RTI
	case 9: // NOT: DR | SR | 111111
		instr |= hegel.Draw(tc, genReg)<<9 | hegel.Draw(tc, genReg)<<6 | 0x3F
	case 12: // JMP: 000 BaseR 000000
		instr |= hegel.Draw(tc, genReg) << 6
	case 13: // reserved
		instr |= hegel.Draw(tc, field(0xFFF))
	case 15: // TRAP: 0000 trapvect8
		instr |= hegel.Draw(tc, field(0xFF))
	}
	return instr
})

// assemble is the inverse of convertHexToLC3Instruction for canonical encodings.
// It rejects any operand that does not fit its field instead of masking it, so a
// disassembler that prints a wrong value cannot round-trip by accident.
func assemble(text string) (uint16, error) {
	tokens := strings.Split(text, " ")
	ops := tokens[1:]

	reg := func(i int) (uint16, error) {
		if i >= len(ops) || len(ops[i]) != 2 || ops[i][0] != 'R' || ops[i][1] < '0' || ops[i][1] > '7' {
			return 0, fmt.Errorf("operand %d of %q is not a register R0-R7", i, text)
		}
		return uint16(ops[i][1] - '0'), nil
	}
	// Raw field bits in hex, zero-padded to exactly the digits a field of bits needs.
	hex := func(i int, bits int) (uint16, error) {
		if i >= len(ops) {
			return 0, fmt.Errorf("operand %d of %q is missing", i, text)
		}
		digits, ok := strings.CutPrefix(ops[i], "0x")
		if !ok {
			return 0, fmt.Errorf("operand %d of %q has no 0x prefix", i, text)
		}
		if want := (bits + 3) / 4; len(digits) != want {
			return 0, fmt.Errorf("operand %d of %q has %d hex digits, want %d for a %d-bit field", i, text, len(digits), want, bits)
		}
		v, err := strconv.ParseUint(digits, 16, bits)
		if err != nil {
			return 0, fmt.Errorf("operand %d of %q does not fit %d bits: %w", i, text, bits, err)
		}
		return uint16(v), nil
	}
	nzp := func(i int) (uint16, error) {
		if i >= len(ops) || len(ops[i]) != 3 {
			return 0, fmt.Errorf("operand %d of %q is not 3 nzp bits", i, text)
		}
		v, err := strconv.ParseUint(ops[i], 2, 3)
		if err != nil {
			return 0, fmt.Errorf("operand %d of %q is not 3 nzp bits: %w", i, text, err)
		}
		return uint16(v), nil
	}
	isReg := func(i int) bool { return i < len(ops) && strings.HasPrefix(ops[i], "R") }
	arity := func(n int) error {
		if len(ops) != n {
			return fmt.Errorf("%q has %d operands, want %d", text, len(ops), n)
		}
		return nil
	}
	// Collects the first error so each case reads as a plain encoding.
	var errs []error
	must := func(v uint16, err error) uint16 {
		errs = append(errs, err)
		return v
	}

	var instr uint16
	switch tokens[0] {
	case "BR":
		errs = append(errs, arity(2))
		instr = must(nzp(0))<<9 | must(hex(1, 9))
	case "ADD", "AND":
		errs = append(errs, arity(3))
		op := uint16(1)
		if tokens[0] == "AND" {
			op = 5
		}
		instr = op<<12 | must(reg(0))<<9 | must(reg(1))<<6
		if isReg(2) {
			instr |= must(reg(2))
		} else {
			instr |= 1<<5 | must(hex(2, 5))
		}
	case "LD", "ST", "LDI", "STI", "LEA":
		errs = append(errs, arity(2))
		op := map[string]uint16{"LD": 2, "ST": 3, "LDI": 10, "STI": 11, "LEA": 14}[tokens[0]]
		instr = op<<12 | must(reg(0))<<9 | must(hex(1, 9))
	case "JSR":
		errs = append(errs, arity(1))
		instr = 4<<12 | 1<<11 | must(hex(0, 11))
	case "JSRR":
		errs = append(errs, arity(1))
		instr = 4<<12 | must(reg(0))<<6
	case "LDR", "STR":
		errs = append(errs, arity(3))
		op := map[string]uint16{"LDR": 6, "STR": 7}[tokens[0]]
		instr = op<<12 | must(reg(0))<<9 | must(reg(1))<<6 | must(hex(2, 6))
	case "RTI":
		errs = append(errs, arity(0))
		instr = 8 << 12
	case "NOT":
		errs = append(errs, arity(2))
		instr = 9<<12 | must(reg(0))<<9 | must(reg(1))<<6 | 0x3F
	case "JMP":
		errs = append(errs, arity(1))
		instr = 12<<12 | must(reg(0))<<6
	case ".FILL":
		errs = append(errs, arity(1))
		instr = must(hex(0, 16))
		if instr>>12 != 13 {
			return 0, fmt.Errorf("%q is a valid instruction, not data", text)
		}
	case "TRAP":
		errs = append(errs, arity(1))
		named := false
		for v, name := range trapVectors {
			if len(ops) == 1 && ops[0] == name {
				instr, named = 15<<12|v, true
			}
		}
		if !named {
			vector := must(hex(0, 8))
			if _, ok := trapVectors[vector]; ok {
				return 0, fmt.Errorf("%q must use the routine name", text)
			}
			instr = 15<<12 | vector
		}
	default:
		return 0, fmt.Errorf("%q has unknown mnemonic", text)
	}
	for _, err := range errs {
		if err != nil {
			return 0, err
		}
	}
	return instr, nil
}

// Every field of a valid instruction is visible in the output: assembling the
// text again gives back the exact same 16 bits.
func TestPropertyRoundTrip(t *testing.T) {
	hegel.Test(t, func(ht *hegel.T) {
		instr := hegel.Draw(ht, genCanonicalInstruction)
		text := convertHexToLC3Instruction(instr)

		got, err := assemble(text)
		if err != nil {
			ht.Fatalf("%#04x disassembled to %q: %v", instr, text, err)
		}
		if got != instr {
			ht.Fatalf("%#04x disassembled to %q, which assembles to %#04x", instr, text, got)
		}
	}, propertyOpts...)
}

// The mnemonic depends only on the opcode, never on the operand bits.
func TestPropertyMnemonicMatchesOpcode(t *testing.T) {
	hegel.Test(t, func(ht *hegel.T) {
		instr := hegel.Draw(ht, genWord)
		want := wantMnemonic(instr)
		text := convertHexToLC3Instruction(instr)
		if got, _, _ := strings.Cut(text, " "); got != want {
			ht.Fatalf("%#04x (opcode %d) disassembled to %q, want mnemonic %s", instr, instr>>12, text, want)
		}
	}, propertyOpts...)
}

// Any 16-bit word, including data words and reserved encodings, gives one
// non-empty line of single-space separated tokens.
func TestPropertyOutputIsWellFormed(t *testing.T) {
	hegel.Test(t, func(ht *hegel.T) {
		instr := hegel.Draw(ht, genWord)
		text := convertHexToLC3Instruction(instr)

		if text == "" {
			ht.Fatalf("%#04x disassembled to an empty string", instr)
		}
		if strings.TrimSpace(text) != text || strings.Contains(text, "  ") || strings.ContainsAny(text, "\n\t") {
			ht.Fatalf("%#04x disassembled to badly spaced %q", instr, text)
		}
	}, propertyOpts...)
}

// Bits that the ISA ignores in a JMP or JSRR do not change which register is shown.
func TestPropertyRegisterJumpsIgnoreUnusedBits(t *testing.T) {
	hegel.Test(t, func(ht *hegel.T) {
		op := hegel.Draw(ht, hegel.SampledFrom([]uint16{4, 12}))
		base := hegel.Draw(ht, genReg)
		canonical := op<<12 | base<<6
		noisy := canonical | hegel.Draw(ht, field(0x3F)) // bits 5-0
		if op == 12 {
			noisy |= hegel.Draw(ht, field(0x7)) << 9 // bits 11-9
		} else {
			noisy |= hegel.Draw(ht, field(0x3)) << 9 // bits 10-9; bit 11 selects JSR
		}

		want := convertHexToLC3Instruction(canonical)
		if got := convertHexToLC3Instruction(noisy); got != want {
			ht.Fatalf("%#04x disassembled to %q, but canonical %#04x gives %q", noisy, got, canonical, want)
		}
	}, propertyOpts...)
}

// Words written big-endian to an image file are read back unchanged and in order.
func TestPropertyReadImageRoundTrip(t *testing.T) {
	dir := t.TempDir()
	hegel.Test(t, func(ht *hegel.T) {
		words := hegel.Draw(ht, hegel.Lists(genWord).MaxSize(64))
		buf := make([]byte, 0, 2*len(words))
		for _, w := range words {
			buf = binary.BigEndian.AppendUint16(buf, w)
		}
		// A trailing half word must give an error, but keep the full words before it.
		oddByte := hegel.Draw(ht, hegel.Booleans())
		if oddByte {
			buf = append(buf, byte(hegel.Draw(ht, field(0xFF))))
		}

		path := filepath.Join(dir, "image.obj")
		if err := os.WriteFile(path, buf, 0o600); err != nil {
			ht.Fatalf("write image: %v", err)
		}

		got, err := readImageInHex(path)
		if oddByte && err == nil {
			ht.Fatalf("image of %d bytes read without error", len(buf))
		}
		if !oddByte && err != nil {
			ht.Fatalf("image of %d bytes: %v", len(buf), err)
		}
		if !slices.Equal(got, words) {
			ht.Fatalf("read %#04x, want %#04x", got, words)
		}
	}, propertyOpts...)
}
