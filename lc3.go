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

func updateConditionFlags(registers []uint16, result uint16) {
	if registers[result] == 0 {
		registers[RCond] = CondZro
	} else if (registers[result] >> 15) == 1 {
		registers[RCond] = CondNeg
	} else {
		registers[RCond] = CondPos
	}
}

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

func memWrite(memory []uint16, address uint16, val uint16) {
	memory[address] = val
}

func memRead(memory []uint16, address uint16) uint16 {
	if address == MmapKBSR {
		key := getInputFromKeyBoard()
		if key != 0 {
			memWrite(memory, MmapKBSR, 1<<15)
			memWrite(memory, MmapKBDR, key)
		} else {
			memWrite(memory, MmapKBSR, 0)
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
	vm := VM{
		Memory:    make([]uint16, MemoryMax),
		Registers: make([]uint16, RCount),
		Executing: Running,
	}

	registers := vm.Registers

	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})
	slog.SetDefault(slog.New(handler))

	err := parseArgs(vm.Memory)
	if err != nil {
		slog.Error("encountered an error in parsing arguments", slog.Any("error", err))
		return
	}

	slog.Info("setup done")

	// since exactly one condition flag should be set at any given time,
	// set the Z flag
	registers[RCond] = CondZro

	// set the PC to starting position
	// 0x3000 is the default
	var PcStart uint16 = 0x3000
	registers[PC] = PcStart

	running := true

	for running {
		// fetch instruction and operation
		instr := memRead(vm.Memory, registers[PC])
		registers[PC]++
		op := instr >> 12
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
			updateConditionFlags(registers, r0)
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
			updateConditionFlags(registers, r0)
		case OpNot:
			slog.Debug("processing NOT instruction")
			r0 := (instr >> 9) & 0x7
			r1 := (instr >> 6) & 0x7
			registers[r0] = ^registers[r1]
			updateConditionFlags(registers, r0)
		case OpBr:
			slog.Debug("processing BR instruction")
			n := (instr >> 11) & 0x1
			z := (instr >> 10) & 0x1
			p := (instr >> 9) & 0x1
			offset := signExtend(instr&0x1FF, 9)
			if (n == 1 && registers[RCond] == CondNeg) || (z == 1 && registers[RCond] == CondZro) ||
				(p == 1 && registers[RCond] == CondPos) {
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
			registers[r0] = memRead(vm.Memory, registers[PC]+offset)
			updateConditionFlags(registers, r0)
		case OpLdi:
			slog.Debug("processing LDI instruction")
			r0 := (instr >> 9) & 0x7
			offset := signExtend(instr&0x1FF, 9)
			registers[r0] = memRead(vm.Memory, memRead(vm.Memory, registers[PC]+offset))
			updateConditionFlags(registers, r0)
		case OpLdr:
			slog.Debug("processing LDR instruction")
			r0 := (instr >> 9) & 0x7
			r1 := (instr >> 6) & 0x7
			offset := signExtend(instr&0x3F, 5)
			registers[r0] = memRead(vm.Memory, registers[r1]+offset)
			updateConditionFlags(registers, r0)
		case OpLea:
			slog.Debug("processing LEA instruction")
			r0 := (instr >> 9) & 0x7
			offset := signExtend(instr&0x1FF, 9)
			registers[r0] = registers[PC] + offset
			updateConditionFlags(registers, r0)
		case OpSt:
			slog.Debug("processing ST instruction")
			r0 := (instr >> 9) & 0x7
			offset := signExtend(instr&0x1FF, 9)
			memWrite(vm.Memory, registers[PC]+offset, registers[r0])
		case OpSti:
			slog.Debug("processing STI instruction")
			r0 := (instr >> 9) & 0x7
			offset := signExtend(instr&0x1FF, 9)
			memWrite(vm.Memory, memRead(vm.Memory, registers[PC]+offset), registers[r0])
		case OpStr:
			slog.Debug("processing STR instruction")
			r0 := (instr >> 9) & 0x7
			r1 := (instr >> 6) & 0x7
			offset := signExtend(instr&0x3F, 6)
			memWrite(vm.Memory, registers[r1]+offset, registers[r0])
		case OpTrap:
			slog.Debug("processing Trap instruction")
			registers[R7] = registers[PC]
			switch instr & 0xFF {
			case TrapGetC:
				slog.Debug("processing Trap GETC instruction")
				reader := bufio.NewReader(os.Stdin)
				inputChar, _, _ := reader.ReadRune()
				registers[R0] = uint16(inputChar)
				updateConditionFlags(vm.Memory, uint16(R0))
			case TrapOut:
				slog.Debug("processing Trap OUT instruction")
				fmt.Println(rune(registers[R0]))
			case TrapPutS:
				slog.Debug("processing Trap PUTS instruction")
				addr := registers[R0]
				for memRead(vm.Memory, addr) != 0 {
					fmt.Printf("%c", rune(memRead(vm.Memory, addr)))
					addr++
				}
			case TrapIn:
				slog.Debug("processing Trap IN instruction")
				fmt.Printf("Enter a character: ")
				reader := bufio.NewReader(os.Stdin)
				inputChar, _, _ := reader.ReadRune()
				fmt.Print(string(inputChar))
				registers[R0] = uint16(inputChar)
				updateConditionFlags(registers, uint16(R0))
			case TrapPutSP:
				slog.Debug("processing Trap PUTSP instruction")
				addr := registers[R0]
				for memRead(vm.Memory, addr) != 0x0000 {
					memoryContents := memRead(vm.Memory, addr)
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

func parseArgs(memory []uint16) error {
	if len(os.Args) < 2 {
		fmt.Println("lc3 [image-file] ...")
		return fmt.Errorf("too few arguments")
	}

	for _, path := range os.Args[1:] {
		err := readImage(path, memory)
		if err != nil {
			return err
		}
	}
	return nil
}

func readImage(filePath string, memory []uint16) error {
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
