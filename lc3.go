package main

import (
	"bufio"
	"fmt"
	"os"
)

type OpCode int

const (
	OpBr   OpCode = iota // branch
	OpAdd                // add
	OpLd                 // load
	OpSt                 // store
	OpJsr                // jump register
	OpAnd                // bitwise and
	OpLdr                // load register
	OpStr                // store register
	OpRti                // unused
	OpNot                // bitwise not
	OpLdi                // load indirect
	OpSti                // store indirect
	OpJmp                // jump
	OpRes                // reserved (unused)
	OpLea                // load effective address
	OpTrap               // execute trap
)

type TrapCode int

const (
	TrapGetC  TrapCode = 0x20
	TrapOut   TrapCode = 0x21
	TrapPutS  TrapCode = 0x22
	TrapIn    TrapCode = 0x23
	TrapPutSP TrapCode = 0x24
	TrapHalt  TrapCode = 0x25
)

type Register uint16

const (
	R0 Register = iota
	R1
	R2
	R3
	R4
	R5
	R6
	R7
	PC // program counter
	COND
	COUNT
)

var registers = make([]uint16, COUNT)

type ConditionFlags uint16

const (
	POS ConditionFlags = 1 << 0
	ZRO                = 1 << 1
	NEG                = 1 << 2
)

func updateConditionFlags(result uint16) {
	if registers[result] == 0 {
		registers[COND] = ZRO
	} else if (registers[result] >> 15) == 1 {
		registers[COND] = NEG
	} else {
		registers[result] = uint16(POS)
	}
}

const MemoryMax uint32 = 1 << 16

var memory = make([]uint16, MemoryMax)

func memWrite(address uint16, val uint16) {
	memory[address] = val
}

func memRead(address uint16) uint16 {
	return memory[address]
}

func signExtend(x uint16, bitCount int) uint16 {
	if ((x >> (bitCount - 1)) & 1) == 1 {
		x |= 0xFFFF << bitCount
	}
	return x
}

func main() {
	// Load Arguments
	// Setup

	// since exactly one condition flag should be set at any given time,
	// set the Z flag
	registers[COND] = ZRO

	// set the PC to starting position
	// 0x3000 is the default
	var PcStart uint16 = 0x3000
	registers[PC] = PcStart

	running := true

	for running {
		// fetch instruction and operation
		instr := memRead(registers[PC])
		registers[PC]++
		op := OpCode(instr >> 12)

		switch op {
		case OpAdd:
			r0 := (instr >> 9) & 0x7
			r1 := (instr >> 5) & 0x7
			immediateMode := (instr >> 5) & 0x1
			if immediateMode == 1 {
				imm5 := signExtend(instr&0x1F, 5)
				registers[r0] = registers[r1] + imm5
			} else {
				r2 := instr & 0x7
				registers[r0] = registers[r1] + registers[r2]
			}
			updateConditionFlags(r0)
		case OpAnd:
			r0 := (instr >> 9) & 0x7
			r1 := (instr >> 5) & 0x7
			immediateMode := (instr >> 5) & 0x1
			if immediateMode == 1 {
				imm5 := signExtend(instr&0x1F, 5)
				registers[r0] = registers[r1] & imm5
			} else {
				r2 := instr & 0x7
				registers[r0] = registers[r1] & registers[r2]
			}
			updateConditionFlags(r0)
		case OpNot:
			r0 := (instr >> 9) & 0x7
			r1 := (instr >> 6) & 0x7
			registers[r0] = ^registers[r1]
			updateConditionFlags(r0)
		case OpBr:
			n := (instr >> 11) & 0x1
			z := (instr >> 10) & 0x1
			p := (instr >> 9) & 0x1
			offset := signExtend(instr&0x1FF, 9)
			if (n == 1 && registers[COND] == NEG) || (z == 1 && registers[COND] == ZRO) ||
				(p == 1 && registers[COND] == uint16(POS)) {
				registers[PC] += offset
			}
		case OpJmp:
			baseRegister := (instr >> 6) & 0x7
			registers[PC] = registers[baseRegister]
		case OpJsr:
			registers[R7] = registers[PC]
			bit11 := (instr >> 11) & 0x1
			if bit11 == 0 {
				// JSRR: Jump to Subroutine Register
				registers[PC] = registers[(instr>>6)&0x7]
			} else {
				// JSR: Jump to Subroutine
				registers[PC] += signExtend(instr&0x7FF, 11)
			}
		case OpLd:
			r0 := (instr >> 9) & 0x7
			offset := signExtend(instr&0x1FF, 9)
			registers[r0] = memRead(registers[PC] + offset)
			updateConditionFlags(r0)
		case OpLdi:
			r0 := (instr >> 9) & 0x7
			offset := signExtend(instr&0x1FF, 9)
			registers[r0] = memRead(memRead(registers[PC] + offset))
			updateConditionFlags(r0)
		case OpLdr:
			r0 := (instr >> 9) & 0x7
			r1 := (instr >> 6) & 0x7
			offset := signExtend(instr&0x3F, 5)
			registers[r0] = memRead(registers[r1] + offset)
			updateConditionFlags(r0)
		case OpLea:
			r0 := (instr >> 9) & 0x7
			offset := signExtend(instr&0x1FF, 9)
			registers[r0] = registers[PC] + offset
			updateConditionFlags(r0)
		case OpSt:
			r0 := (instr >> 9) & 0x7
			offset := signExtend(instr&0x1FF, 9)
			memWrite(registers[PC]+offset, registers[r0])
		case OpSti:
			r0 := (instr >> 9) & 0x7
			offset := signExtend(instr&0x1FF, 9)
			memWrite(memRead(registers[PC]+offset), registers[r0])
		case OpStr:
			r0 := (instr >> 9) & 0x7
			r1 := (instr >> 6) & 0x7
			offset := signExtend(instr&0x3F, 6)
			memWrite(registers[r1]+offset, registers[r0])
		case OpTrap:
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
				fmt.Print(string(inputChar))
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
				running = false
			}
		case OpRes:
		case OpRti:
		default:
		}
	}
}
