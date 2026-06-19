package main

type ConditionFlags int

const (
	POS ConditionFlags = 1 << 0
	ZRO                = 1 << 1
	NEG                = 1 << 2
)
