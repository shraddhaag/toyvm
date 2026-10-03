package main

import (
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConvertHexToLC3Instruction(t *testing.T) {
	tests := []struct {
		name        string
		instruction uint16
		want        string
	}{
		// Immediates and offsets show their raw bits in hex, zero-padded to the
		// digits the field needs: imm5, offset6 and trapvect8 use 2 digits,
		// PCoffset9 and PCoffset11 use 3, a .FILL word uses 4.

		// BR: 0000 | n z p | PCoffset9
		{"BR nzp with offset", 0x0E05, "BR 111 0x005"},
		{"BRz", 0x0402, "BR 010 0x002"},
		{"BRp with all bits set", 0x03FF, "BR 001 0x1ff"},
		{"BRp with 0x0ff offset", 0x02FF, "BR 001 0x0ff"},
		{"BRn with 0x100 offset", 0x0900, "BR 100 0x100"},
		{"BRn with zero offset", 0x0800, "BR 100 0x000"},
		{"BR with no condition bits (NOP)", 0x0000, "BR 000 0x000"},

		// ADD: 0001 | DR | SR1 | 0 | 00 | SR2  or  0001 | DR | SR1 | 1 | imm5
		{"ADD register mode", 0x1042, "ADD R0 R1 R2"},
		{"ADD register mode, all R7", 0x1FC7, "ADD R7 R7 R7"},
		{"ADD immediate mode", 0x1265, "ADD R1 R1 0x05"},
		{"ADD immediate mode, zero", 0x1260, "ADD R1 R1 0x00"},
		{"ADD immediate mode, all bits set", 0x103F, "ADD R0 R0 0x1f"},
		{"ADD immediate mode, 0b01111 imm5", 0x102F, "ADD R0 R0 0x0f"},
		{"ADD immediate mode, 0b10000 imm5", 0x1030, "ADD R0 R0 0x10"},

		// LD: 0010 | DR | PCoffset9
		{"LD", 0x2C17, "LD R6 0x017"},
		{"LD all bits set", 0x21FF, "LD R0 0x1ff"},
		{"LD 0x0ff offset", 0x20FF, "LD R0 0x0ff"},
		{"LD 0x100 offset", 0x2100, "LD R0 0x100"},

		// ST: 0011 | SR | PCoffset9
		{"ST", 0x3205, "ST R1 0x005"},
		{"ST all bits set", 0x3FFF, "ST R7 0x1ff"},

		// JSR: 0100 | 1 | PCoffset11  /  JSRR: 0100 | 0 | 00 | BaseR | 000000
		{"JSR pc-relative", 0x4A7D, "JSR 0x27d"},
		{"JSR pc-relative all bits set", 0x4FFF, "JSR 0x7ff"},
		{"JSR pc-relative 0x3ff offset", 0x4BFF, "JSR 0x3ff"},
		{"JSR pc-relative 0x400 offset", 0x4C00, "JSR 0x400"},
		{"JSRR register", 0x40C0, "JSRR R3"},
		{"JSRR R7", 0x41C0, "JSRR R7"},

		// AND: 0101 | DR | SR1 | 0 | 00 | SR2  or  0101 | DR | SR1 | 1 | imm5
		{"AND register mode", 0x5A83, "AND R5 R2 R3"},
		{"AND immediate mode, clear register", 0x5260, "AND R1 R1 0x00"},
		{"AND immediate mode, all bits set", 0x507F, "AND R0 R1 0x1f"},
		{"AND immediate mode, 0b01111 imm5", 0x506F, "AND R0 R1 0x0f"},

		// LDR: 0110 | DR | BaseR | offset6
		{"LDR", 0x6283, "LDR R1 R2 0x03"},
		{"LDR all bits set", 0x6FFF, "LDR R7 R7 0x3f"},
		{"LDR 0x1f offset", 0x605F, "LDR R0 R1 0x1f"},
		{"LDR 0x20 offset", 0x6060, "LDR R0 R1 0x20"},

		// STR: 0111 | SR | BaseR | offset6
		{"STR", 0x7440, "STR R2 R1 0x00"},
		{"STR all bits set", 0x7FBF, "STR R7 R6 0x3f"},

		// RTI: 1000 | 000000000000
		{"RTI", 0x8000, "RTI"},

		// NOT: 1001 | DR | SR | 111111
		{"NOT", 0x987F, "NOT R4 R1"},
		{"NOT same register", 0x9FFF, "NOT R7 R7"},

		// LDI: 1010 | DR | PCoffset9
		{"LDI", 0xA405, "LDI R2 0x005"},
		{"LDI all bits set", 0xA5FF, "LDI R2 0x1ff"},

		// STI: 1011 | SR | PCoffset9
		{"STI", 0xB022, "STI R0 0x022"},
		{"STI all bits set", 0xB1FF, "STI R0 0x1ff"},

		// JMP: 1100 | 000 | BaseR | 000000
		{"JMP", 0xC080, "JMP R2"},
		{"JMP R7 (RET)", 0xC1C0, "JMP R7"},

		// 1101 is reserved: the word can only be data.
		{"reserved opcode", 0xD000, ".FILL 0xd000"},
		{"reserved opcode, all bits set", 0xDFFF, ".FILL 0xdfff"},

		// LEA: 1110 | DR | PCoffset9
		{"LEA", 0xEA18, "LEA R5 0x018"},
		{"LEA all bits set", 0xE1FF, "LEA R0 0x1ff"},

		// TRAP: 1111 | 0000 | trapvect8
		{"TRAP GETC", 0xF020, "TRAP GETC"},
		{"TRAP OUT", 0xF021, "TRAP OUT"},
		{"TRAP PUTS", 0xF022, "TRAP PUTS"},
		{"TRAP IN", 0xF023, "TRAP IN"},
		{"TRAP PUTSP", 0xF024, "TRAP PUTSP"},
		{"TRAP HALT", 0xF025, "TRAP HALT"},
		{"TRAP unknown vector", 0xF026, "TRAP 0x26"},
		{"TRAP vector zero", 0xF000, "TRAP 0x00"},
		{"TRAP max vector", 0xF0FF, "TRAP 0xff"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, convertHexToLC3Instruction(tt.instruction))
		})
	}
}

func TestReadImageInHex(t *testing.T) {
	tests := []struct {
		name    string
		bytes   []byte
		want    []uint16
		wantErr error
	}{
		{"empty file", []byte{}, nil, nil},
		{"origin only", []byte{0x30, 0x00}, []uint16{0x3000}, nil},
		{
			"origin and instructions are big-endian",
			[]byte{0x30, 0x00, 0x2C, 0x17, 0xF0, 0x25},
			[]uint16{0x3000, 0x2C17, 0xF025},
			nil,
		},
		{
			"trailing odd byte",
			[]byte{0x30, 0x00, 0xF0},
			[]uint16{0x3000},
			io.ErrUnexpectedEOF,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "image.obj")
			require.NoError(t, os.WriteFile(path, tt.bytes, 0o600))

			got, err := readImageInHex(path)
			assert.ErrorIs(t, err, tt.wantErr)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestReadImageInHexMissingFile(t *testing.T) {
	_, err := readImageInHex(filepath.Join(t.TempDir(), "missing.obj"))
	assert.ErrorIs(t, err, os.ErrNotExist)
}
