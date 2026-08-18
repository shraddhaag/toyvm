package main

import (
	"bufio"
	"fmt"
	"os"
)

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

type TrapCode int

const (
	TrapGetC  TrapCode = 0x20
	TrapOut            = 0x21
	TrapPutS           = 0x22
	TrapIn             = 0x23
	TrapPutSP          = 0x24
	TrapHalt           = 0x25
)

func handleTrapInstructions(instr uint16) {
	registers[R7] = registers[PC]

	switch TrapCode(instr & 0xFF) {

	case TrapGetC:
		reader := bufio.NewReader(os.Stdin)
		inputChar, _, _ := reader.ReadRune()
		registers[R0] = uint16(inputChar)
		updateConditionFlags(uint16(R0))

	case TrapOut:
		fmt.Println(rune(registers[R0]))

	case TrapPutS:
		addr := registers[R0]
		for memRead(addr) != 0x0000 {
			fmt.Print(rune(memRead(addr)))
			addr++
		}
	case TrapIn:
		fmt.Printf("Enter a character: ")
		reader := bufio.NewReader(os.Stdin)
		inputChar, _, _ := reader.ReadRune()
		fmt.Printf(string(inputChar))
		registers[R0] = uint16(inputChar)
		updateConditionFlags(uint16(R0))
	case TrapPutSP:
		addr := registers[R0]
		for memRead(addr) != 0x0000 {
			memoryContents := memRead(addr)
			fmt.Print(memoryContents&0xFF, memoryContents>>8)
			addr++
		}
	case TrapHalt:
		fmt.Println("Halt Program!")
		// running = false
	}
}

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

func and(instr uint16) {
	// Destination Register: 11-9 bits
	var r0 uint16 = (instr >> 9) & 0x7
	// SR1: 8-6 bits
	var r1 uint16 = (instr >> 5) & 0x7
	// check if bit 5 is set:
	// yes - read next number to AND from 0-4 bits
	// no - read next number to AND from register
	var immediateMode uint16 = (instr >> 5) & 0x1

	if immediateMode == 1 {
		var imm5 uint16 = signExtend(instr&0x1F, 5)
		registers[r0] = registers[r1] & imm5
	} else {
		var r2 uint16 = instr & 0x7
		registers[r0] = registers[r1] & registers[r2]
	}

	updateConditionFlags(r0)
}

func branch(instr uint16) {
	var n uint16 = (instr >> 11) & 0x1
	var z uint16 = (instr >> 10) & 0x1
	var p uint16 = (instr >> 9) & 0x1
	var offset uint16 = signExtend(instr&0x1FF, 9)

	if (n == 1 && registers[COND] == NEG) || (z == 1 && registers[COND] == ZRO) ||
		(p == 1 && registers[COND] == uint16(POS)) {
		registers[PC] += offset
	}

	// an alternate, better immplementation of this:
	// instead of getting n, z and p separately, we can get them all together as nzp
	// in such a case, nzp can have values: 000,100,010,001,110,011,111
	// var nzp uint16 = (instr >> 9) * 0x7 <- 0x7 instead of 0x1
	// ...
	// if nzp & registers[COND] != 0 { <- nzp AND condition register result non zero
	// ... increment PC
	// }
	//
}

func jump(instr uint16) {
	// when the value of baseRegister is 7 ie 111
	// the instruction is RET ie return.
	// PC is set to the contents of R7 which stores the linkage back to main.
	var baseRegister uint16 = (instr >> 6) & 0x7
	registers[PC] = registers[baseRegister]
}

func jumpRegister(instr uint16) {
	registers[R7] = registers[PC]

	var bit11 uint16 = (instr >> 11) & 0x1

	if bit11 == 0 {
		// JSRR: Jump to Subroutine Register
		registers[PC] = registers[(instr>>6)&0x7]
	} else {
		// JSR: Jump to Subroutine
		registers[PC] += signExtend(instr&0x7FF, 11)
	}
}

func load(instr uint16) {
	var r0 uint16 = (instr >> 9) & 0x7
	var offset uint16 = signExtend(instr&0x1FF, 9)

	registers[r0] = memRead(registers[PC] + offset)
	updateConditionFlags(r0)
}

// loadIndirect is used when we need to load far away memory
// mem to lead = value at memory (PC + pffset)
func loadIndirect(instr uint16) {
	var r0 uint16 = (instr >> 9) & 0x7
	var offset uint16 = signExtend(instr&0x1FF, 9)

	registers[r0] = memRead(memRead(registers[PC] + offset))
	updateConditionFlags(r0)
}

func loadRegister(instr uint16) {
	var r0 uint16 = (instr >> 9) & 0x7
	var r1 uint16 = (instr >> 6) & 0x7
	var offset uint16 = signExtend(instr&0x3F, 5)

	registers[r0] = memRead(registers[r1] + offset)
	updateConditionFlags(r0)
}

func loadEffectiveAddress(instr uint16) {
	var r0 uint16 = (instr >> 9) & 0x7
	var offset uint16 = signExtend(instr&0x1FF, 9)

	registers[r0] = registers[PC] + offset
	updateConditionFlags(r0)
}

func not(instr uint16) {
	var r0 uint16 = (instr >> 9) & 0x7
	var r1 uint16 = (instr >> 6) & 0x7

	registers[r0] = ^registers[r1]
	updateConditionFlags(r0)
}

func store(instr uint16) {
	var r0 uint16 = (instr >> 9) & 0x7
	var offset uint16 = signExtend(instr&0x1FF, 9)
	memWrite(registers[PC]+offset, registers[r0])
}

func storeIndirect(instr uint16) {
	var r0 uint16 = (instr >> 9) & 0x7
	var offset uint16 = signExtend(instr&0x1FF, 9)
	memWrite(memRead(registers[PC]+offset), registers[r0])
}

func storeRegister(instr uint16) {
	var r0 uint16 = (instr >> 9) & 0x7
	var r1 uint16 = (instr >> 6) & 0x7
	var offset uint16 = signExtend(instr&0x3F, 6)
	memWrite(registers[r1]+offset, registers[r0])
}
