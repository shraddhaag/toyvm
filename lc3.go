package main

import (
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
	vm.Registers[RCond] = CondZro

	// set the PC to starting position
	// 0x3000 is the default
	var PcStart uint16 = 0x3000
	vm.Registers[PC] = PcStart

	for vm.Executing == Running {
		// fetch instruction and operation
		instr := memRead(vm.Memory, vm.Registers[PC])
		vm.Registers[PC]++
		op := instr >> 12
		slog.Debug("start processing instruction", "instruction", instr)
		switch op {
		case OpAdd:
			vm.add(instr)
		case OpAnd:
			vm.and(instr)
		case OpNot:
			vm.not(instr)
		case OpBr:
			vm.br(instr)
		case OpJmp:
			vm.jmp(instr)
		case OpJsr:
			vm.jsr(instr)
		case OpLd:
			vm.ld(instr)
		case OpLdi:
			vm.ldi(instr)
		case OpLdr:
			vm.ldr(instr)
		case OpLea:
			vm.lea(instr)
		case OpSt:
			vm.st(instr)
		case OpSti:
			vm.sti(instr)
		case OpStr:
			vm.str(instr)
		case OpTrap:
			vm.trap(instr)
		case OpRes:
			vm.res(instr)
		case OpRti:
			vm.rti(instr)
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
