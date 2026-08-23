package main

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"io"
	"log/slog"
	"os"

	"atomicgo.dev/keyboard"
	"atomicgo.dev/keyboard/keys"
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

const (
	MmapKBSR = 0xFE00
	MmapKBDR = 0xFE02
)

func getInputFromKeyBoard() uint16 {
	var keyPresssed uint16
	keyboard.Listen(func(key keys.Key) (stop bool, err error) {
		switch key.Code {
		case keys.CtrlC, keys.Esc:
			slog.Info("halting")
			os.Exit(1)
		default:
			keyPresssed = uint16(key.Code)
			return true, nil
		}
		return true, nil
	})
	return keyPresssed
}

var memory = make([]uint16, MemoryMax)

func memWrite(address uint16, val uint16) {
	memory[address] = val
}

func memRead(address uint16) uint16 {
	if address == MmapKBSR {
		key := getInputFromKeyBoard()
		if key != 0 {
			memWrite(MmapKBSR, 1<<15)
			memWrite(MmapKBDR, key)
		} else {
			memWrite(MmapKBSR, 0)
		}
	}
	return memory[address]
}

func signExtend(x uint16, bitCount int) uint16 {
	if ((x >> (bitCount - 1)) & 1) == 1 {
		x |= 0xFFFF << bitCount
	}
	return x
}

func main() {
	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	})
	slog.SetDefault(slog.New(handler))

	err := parseArgs()
	if err != nil {
		slog.Error("encountered an error in parsing arguments", slog.Any("error", err))
		return
	}

	slog.Info("setup done")

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
		slog.Debug("start processing instruction", "instruction", instr)
		switch op {
		case OpAdd:
			slog.Debug("processing ADD instruction")
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
			slog.Debug("processing AND instruction")
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
			slog.Debug("processing NOT instruction")
			r0 := (instr >> 9) & 0x7
			r1 := (instr >> 6) & 0x7
			registers[r0] = ^registers[r1]
			updateConditionFlags(r0)
		case OpBr:
			slog.Debug("processing BR instruction")
			n := (instr >> 11) & 0x1
			z := (instr >> 10) & 0x1
			p := (instr >> 9) & 0x1
			offset := signExtend(instr&0x1FF, 9)
			if (n == 1 && registers[COND] == NEG) || (z == 1 && registers[COND] == ZRO) ||
				(p == 1 && registers[COND] == uint16(POS)) {
				registers[PC] += offset
			}
		case OpJmp:
			slog.Debug("processing JMP instruction")
			baseRegister := (instr >> 6) & 0x7
			registers[PC] = registers[baseRegister]
		case OpJsr:
			slog.Debug("processing JSR instruction")
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
			slog.Debug("processing LD instruction")
			r0 := (instr >> 9) & 0x7
			offset := signExtend(instr&0x1FF, 9)
			registers[r0] = memRead(registers[PC] + offset)
			updateConditionFlags(r0)
		case OpLdi:
			slog.Debug("processing LDI instruction")
			r0 := (instr >> 9) & 0x7
			offset := signExtend(instr&0x1FF, 9)
			registers[r0] = memRead(memRead(registers[PC] + offset))
			updateConditionFlags(r0)
		case OpLdr:
			slog.Debug("processing LDR instruction")
			r0 := (instr >> 9) & 0x7
			r1 := (instr >> 6) & 0x7
			offset := signExtend(instr&0x3F, 5)
			registers[r0] = memRead(registers[r1] + offset)
			updateConditionFlags(r0)
		case OpLea:
			slog.Debug("processing LEA instruction")
			r0 := (instr >> 9) & 0x7
			offset := signExtend(instr&0x1FF, 9)
			registers[r0] = registers[PC] + offset
			updateConditionFlags(r0)
		case OpSt:
			slog.Debug("processing ST instruction")
			r0 := (instr >> 9) & 0x7
			offset := signExtend(instr&0x1FF, 9)
			memWrite(registers[PC]+offset, registers[r0])
		case OpSti:
			slog.Debug("processing STI instruction")
			r0 := (instr >> 9) & 0x7
			offset := signExtend(instr&0x1FF, 9)
			memWrite(memRead(registers[PC]+offset), registers[r0])
		case OpStr:
			slog.Debug("processing STR instruction")
			r0 := (instr >> 9) & 0x7
			r1 := (instr >> 6) & 0x7
			offset := signExtend(instr&0x3F, 6)
			memWrite(registers[r1]+offset, registers[r0])
		case OpTrap:
			slog.Debug("processing Trap instruction")
			registers[R7] = registers[PC]
			switch TrapCode(instr & 0xFF) {
			case TrapGetC:
				slog.Debug("processing Trap GETC instruction")
				reader := bufio.NewReader(os.Stdin)
				inputChar, _, _ := reader.ReadRune()
				registers[R0] = uint16(inputChar)
				updateConditionFlags(uint16(R0))
			case TrapOut:
				slog.Debug("processing Trap OUT instruction")
				fmt.Println(rune(registers[R0]))
			case TrapPutS:
				slog.Debug("processing Trap PUTS instruction")
				addr := registers[R0]
				for memRead(addr) != 0 {
					fmt.Print(rune(memRead(addr)))
					addr++
				}
			case TrapIn:
				slog.Debug("processing Trap IN instruction")
				fmt.Printf("Enter a character: ")
				reader := bufio.NewReader(os.Stdin)
				inputChar, _, _ := reader.ReadRune()
				fmt.Print(string(inputChar))
				registers[R0] = uint16(inputChar)
				updateConditionFlags(uint16(R0))
			case TrapPutSP:
				slog.Debug("processing Trap PUTSP instruction")
				addr := registers[R0]
				for memRead(addr) != 0x0000 {
					memoryContents := memRead(addr)
					fmt.Print(memoryContents&0xFF, memoryContents>>8)
					addr++
				}
			case TrapHalt:
				slog.Debug("processing Trap HALT instruction")
				slog.Info("Halt Program!")
				running = false
			}
		case OpRes:
		case OpRti:
		default:
		}
	}
}

func parseArgs() error {
	if len(os.Args) < 2 {
		fmt.Println("lc3 [image-file] ...")
		return fmt.Errorf("too few arguments")
	}

	for _, path := range os.Args[1:] {
		err := readImage(path)
		if err != nil {
			return err
		}
	}
	return nil
}

func readImage(filePath string) error {
	file, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	// read origin first
	var origin uint16
	err = binary.Read(file, binary.BigEndian, &origin)
	if err != nil {
		return err
	}

	slog.Info("origin addr is read", "origin", fmt.Sprintf("%#x", origin))

	var val uint16
	addr := origin
	for {
		err = binary.Read(file, binary.BigEndian, &val)
		if err == io.EOF {
			break
		} else if err != nil {
			return err
		}

		memory[addr] = val
		addr++
	}
	return nil
}
