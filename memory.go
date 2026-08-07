package main

const MemoryMax uint32 = 1 << 16

var memory = make([]uint16, MemoryMax)

func memWrite(address uint16, val uint16) {
	memory[address] = val
}

func memRead(address uint16) uint16 {
	return memory[address]
}
