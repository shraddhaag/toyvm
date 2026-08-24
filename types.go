package main

// LC3 OP codes
const (
	OpBr   = iota // branch
	OpAdd         // add
	OpLd          // load
	OpSt          // store
	OpJsr         // jump register
	OpAnd         // bitwise and
	OpLdr         // load register
	OpStr         // store register
	OpRti         // unused
	OpNot         // bitwise not
	OpLdi         // load indirect
	OpSti         // store indirect
	OpJmp         // jump
	OpRes         // reserved (unused)
	OpLea         // load effective address
	OpTrap        // execute trap
)

// LC3 Trap Codes
const (
	TrapGetC  = 0x20
	TrapOut   = 0x21
	TrapPutS  = 0x22
	TrapIn    = 0x23
	TrapPutSP = 0x24
	TrapHalt  = 0x25
)

// LC3 Memory Mapped Registers
const (
	MmapKBSR = 0xFE00
	MmapKBDR = 0xFE02
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
	RCond
	RCount
)

// LC3 Condition flags
const (
	CondPos = 1 << 0
	CondZro = 1 << 1
	CondNeg = 1 << 2
)

const MemoryMax uint32 = 1 << 16

type ExecutionState bool

const (
	Running ExecutionState = true
	Halted                 = false
)

type VM struct {
	Memory    []uint16
	Registers []uint16
	Executing ExecutionState
}
