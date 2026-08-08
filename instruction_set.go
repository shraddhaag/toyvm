package main

type OpCode int

const (
	BR   OpCode = iota // branch
	ADD                // add
	LD                 // load
	ST                 // store
	JSR                // jump register
	AND                // bitwise and
	LDR                // load register
	STR                // store register
	RTI                // unused
	NOT                // bitwise not
	LDI                // load indirect
	STI                // store indirect
	JMP                // jump
	RES                // reserved (unused)
	LEA                // load effective address
	TRAP               // execute trap
)

func signExtend(x uint16, bitCount int) uint16 {
	if ((x >> (bitCount - 1)) & 1) == 1 {
		x |= 0xFFFF << bitCount
	}
	return x
}

func add(instr uint16) {
	// Destination Register: 11-9 bits
	var r0 uint16 = (instr >> 9) & 0x7
	// SR1: 8-6 bits
	var r1 uint16 = (instr >> 5) & 0x7
	// check if bit 5 is set:
	// yes - read next number to add from 0-4 bits
	// no - read next number to add from register
	var immediateMode uint16 = (instr >> 5) & 0x1

	if immediateMode == 1 {
		var imm5 uint16 = signExtend(instr&0x1F, 5)
		registers[r0] = registers[r1] + imm5
	} else {
		var r2 uint16 = instr & 0x7
		registers[r0] = registers[r1] + registers[r2]
	}

	updateConditionFlags(r0)
}
