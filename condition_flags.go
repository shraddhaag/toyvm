package main

type ConditionFlags int

const (
	POS ConditionFlags = 1 << 0
	ZRO                = 1 << 1
	NEG                = 1 << 2
)

func updateConditionFlags(result uint16) {
	if registers[result] == 0 {
		registers[COND] = ZRO
	} else if registers[result] >> 15 {
		registers[COND] = NEG
	} else {
		registers[result] = POS
	}
}
