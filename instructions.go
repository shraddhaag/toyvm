package main

import (
	"bufio"
	"fmt"
	"log/slog"
	"os"
)

func (vm *VM) add(instr uint16) {
	slog.Debug("processing ADD instruction")
	r0 := (instr >> 9) & 0x7
	r1 := (instr >> 5) & 0x7
	immediateMode := (instr >> 5) & 0x1
	if immediateMode == 1 {
		imm5 := signExtend(instr&0x1F, 5)
		vm.Registers[r0] = vm.Registers[r1] + imm5
	} else {
		r2 := instr & 0x7
		vm.Registers[r0] = vm.Registers[r1] + vm.Registers[r2]
	}
	updateConditionFlags(vm.Registers, r0)
}

func (vm *VM) and(instr uint16) {
	slog.Debug("processing AND instruction")
	r0 := (instr >> 9) & 0x7
	r1 := (instr >> 5) & 0x7
	immediateMode := (instr >> 5) & 0x1
	if immediateMode == 1 {
		imm5 := signExtend(instr&0x1F, 5)
		vm.Registers[r0] = vm.Registers[r1] & imm5
	} else {
		r2 := instr & 0x7
		vm.Registers[r0] = vm.Registers[r1] & vm.Registers[r2]
	}
	updateConditionFlags(vm.Registers, r0)
}

func (vm *VM) not(instr uint16) {
	slog.Debug("processing NOT instruction")
	r0 := (instr >> 9) & 0x7
	r1 := (instr >> 6) & 0x7
	vm.Registers[r0] = ^vm.Registers[r1]
	updateConditionFlags(vm.Registers, r0)
}

func (vm *VM) br(instr uint16) {
	slog.Debug("processing BR instruction")
	n := (instr >> 11) & 0x1
	z := (instr >> 10) & 0x1
	p := (instr >> 9) & 0x1
	offset := signExtend(instr&0x1FF, 9)
	if (n == 1 && vm.Registers[RCond] == CondNeg) || (z == 1 && vm.Registers[RCond] == CondZro) ||
		(p == 1 && vm.Registers[RCond] == CondPos) {
		vm.Registers[PC] += offset
	}
}

func (vm *VM) jmp(instr uint16) {
	slog.Debug("processing JMP instruction")
	baseRegister := (instr >> 6) & 0x7
	vm.Registers[PC] = vm.Registers[baseRegister]
}

func (vm *VM) jsr(instr uint16) {
	slog.Debug("processing JSR instruction")
	vm.Registers[R7] = vm.Registers[PC]
	bit11 := (instr >> 11) & 0x1
	if bit11 == 0 {
		// JSRR: Jump to Subroutine Register
		vm.Registers[PC] = vm.Registers[(instr>>6)&0x7]
	} else {
		// JSR: Jump to Subroutine
		vm.Registers[PC] += signExtend(instr&0x7FF, 11)
	}
}

func (vm *VM) ld(instr uint16) {
	slog.Debug("processing LD instruction")
	r0 := (instr >> 9) & 0x7
	offset := signExtend(instr&0x1FF, 9)
	vm.Registers[r0] = memRead(vm.Memory, vm.Registers[PC]+offset)
	updateConditionFlags(vm.Registers, r0)
}

func (vm *VM) ldi(instr uint16) {
	slog.Debug("processing LDI instruction")
	r0 := (instr >> 9) & 0x7
	offset := signExtend(instr&0x1FF, 9)
	vm.Registers[r0] = memRead(vm.Memory, memRead(vm.Memory, vm.Registers[PC]+offset))
	updateConditionFlags(vm.Registers, r0)
}

func (vm *VM) ldr(instr uint16) {
	slog.Debug("processing LDR instruction")
	r0 := (instr >> 9) & 0x7
	r1 := (instr >> 6) & 0x7
	offset := signExtend(instr&0x3F, 5)
	vm.Registers[r0] = memRead(vm.Memory, vm.Registers[r1]+offset)
	updateConditionFlags(vm.Registers, r0)
}

func (vm *VM) lea(instr uint16) {
	slog.Debug("processing LEA instruction")
	r0 := (instr >> 9) & 0x7
	offset := signExtend(instr&0x1FF, 9)
	vm.Registers[r0] = vm.Registers[PC] + offset
	updateConditionFlags(vm.Registers, r0)
}

func (vm *VM) st(instr uint16) {
	slog.Debug("processing ST instruction")
	r0 := (instr >> 9) & 0x7
	offset := signExtend(instr&0x1FF, 9)
	memWrite(vm.Memory, vm.Registers[PC]+offset, vm.Registers[r0])
}

func (vm *VM) sti(instr uint16) {
	slog.Debug("processing STI instruction")
	r0 := (instr >> 9) & 0x7
	offset := signExtend(instr&0x1FF, 9)
	memWrite(vm.Memory, memRead(vm.Memory, vm.Registers[PC]+offset), vm.Registers[r0])
}

func (vm *VM) str(instr uint16) {
	slog.Debug("processing STR instruction")
	r0 := (instr >> 9) & 0x7
	r1 := (instr >> 6) & 0x7
	offset := signExtend(instr&0x3F, 6)
	memWrite(vm.Memory, vm.Registers[r1]+offset, vm.Registers[r0])
}

func (vm *VM) trap(instr uint16) {
	slog.Debug("processing Trap instruction")
	vm.Registers[R7] = vm.Registers[PC]
	switch instr & 0xFF {
	case TrapGetC:
		vm.trapGetC()
	case TrapOut:
		vm.trapOut()
	case TrapPutS:
		vm.trapPutS()
	case TrapIn:
		vm.trapIn()
	case TrapPutSP:
		vm.trapPutSP()
	case TrapHalt:
		vm.trapHalt()
	}
}

func (vm *VM) trapGetC() {
	slog.Debug("processing Trap GETC instruction")
	reader := bufio.NewReader(os.Stdin)
	inputChar, _, _ := reader.ReadRune()
	vm.Registers[R0] = uint16(inputChar)
	updateConditionFlags(vm.Registers, uint16(R0))
}

func (vm *VM) trapOut() {
	slog.Debug("processing Trap OUT instruction")
	fmt.Println(rune(vm.Registers[R0]))
}

func (vm *VM) trapPutS() {
	slog.Debug("processing Trap PUTS instruction")
	addr := vm.Registers[R0]
	for memRead(vm.Memory, addr) != 0 {
		fmt.Printf("%c", rune(memRead(vm.Memory, addr)))
		addr++
	}
}

func (vm *VM) trapIn() {
	slog.Debug("processing Trap IN instruction")
	fmt.Printf("Enter a character: ")
	reader := bufio.NewReader(os.Stdin)
	inputChar, _, _ := reader.ReadRune()
	fmt.Print(string(inputChar))
	vm.Registers[R0] = uint16(inputChar)
	updateConditionFlags(vm.Registers, uint16(R0))
}

func (vm *VM) trapPutSP() {
	slog.Debug("processing Trap PUTSP instruction")
	addr := vm.Registers[R0]
	for memRead(vm.Memory, addr) != 0x0000 {
		memoryContents := memRead(vm.Memory, addr)
		fmt.Print(memoryContents&0xFF, memoryContents>>8)
		addr++
	}
}

func (vm *VM) trapHalt() {
	slog.Debug("processing Trap HALT instruction")
	slog.Info("Halt Program!")
	vm.Executing = Halted
}

func (vm *VM) rti(instr uint16) {}

func (vm *VM) res(instr uint16) {}
