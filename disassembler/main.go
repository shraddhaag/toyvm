package main

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"strings"
)

func main() {
	err := parseArgs()
	if err != nil {
		fmt.Println("error encountered: ", err)
	}

	for _, path := range os.Args[1:] {
		hexDump, err := readImageInHex(path)
		if err != nil {
			fmt.Println("error reading file: ", err)
		}

		for i, hexCode := range hexDump {
			if i == 0 {
				fmt.Printf("origin: %#04x\n", hexCode)
				continue
			}
			fmt.Printf("%#04x %s\n", hexCode, convertHexToLC3Instruction(hexCode))
		}
	}
}

func parseArgs() error {
	if len(os.Args) < 2 {
		fmt.Println("lc3-disassembler [image-file] ...")
		return fmt.Errorf("too few arguments, no image file to disassemble")
	}
	return nil
}

func readImageInHex(path string) (hex []uint16, err error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var val uint16
	for {
		err = binary.Read(file, binary.BigEndian, &val)
		if err == io.EOF {
			break
		} else if err != nil {
			return hex, err
		}
		hex = append(hex, val)
	}
	return hex, nil
}

func convertHexToLC3Instruction(instruction uint16) string {
	var a strings.Builder
	opCode := instruction >> 12
	switch opCode {
	case 0:
		a.WriteString("BR ")
		a.WriteString(fmt.Sprintf("%0.3b ", (instruction>>9)&0xFFF))
		a.WriteString(fmt.Sprintf("%#03x", instruction&0x1FF))
	case 1:
		a.WriteString("ADD ")
		a.WriteString(fmt.Sprintf("R%d ", (instruction>>9)&0x7))
		a.WriteString(fmt.Sprintf("R%d ", (instruction>>6)&0x7))
		switch (instruction >> 5) & 0x1 {
		case 0:
			a.WriteString(fmt.Sprintf("R%d", instruction&0x7))
		case 1:
			a.WriteString(fmt.Sprintf("%#02x", instruction&0x1F))
		}
	case 2:
		a.WriteString("LD ")
		a.WriteString(fmt.Sprintf("R%d ", (instruction>>9)&0x7))
		a.WriteString(fmt.Sprintf("%#03x", instruction&0x1FF))
	case 3:
		a.WriteString("ST ")
		a.WriteString(fmt.Sprintf("R%d ", (instruction>>9)&0x7))
		a.WriteString(fmt.Sprintf("%#03x", instruction&0x1FF))
	case 4:
		switch (instruction >> 11) & 0x1 {
		case 0:
			a.WriteString("JSRR ")
			a.WriteString(fmt.Sprintf("R%d", (instruction>>6)&0x7))
		case 1:
			a.WriteString("JSR ")
			a.WriteString(fmt.Sprintf("%#03x", instruction&0x7FF))
		}
	case 5:
		a.WriteString("AND ")
		a.WriteString(fmt.Sprintf("R%d ", (instruction>>9)&0x7))
		a.WriteString(fmt.Sprintf("R%d ", (instruction>>6)&0x7))
		switch (instruction >> 5) & 0x1 {
		case 0:
			a.WriteString(fmt.Sprintf("R%d", instruction&0x7))
		case 1:
			a.WriteString(fmt.Sprintf("%#02x", instruction&0x1F))
		}
	case 6:
		a.WriteString("LDR ")
		a.WriteString(fmt.Sprintf("R%d ", (instruction>>9)&0x7))
		a.WriteString(fmt.Sprintf("R%d ", (instruction>>6)&0x7))
		a.WriteString(fmt.Sprintf("%#02x", instruction&0x3F))
	case 7:
		a.WriteString("STR ")
		a.WriteString(fmt.Sprintf("R%d ", (instruction>>9)&0x7))
		a.WriteString(fmt.Sprintf("R%d ", (instruction>>6)&0x7))
		a.WriteString(fmt.Sprintf("%#02x", instruction&0x3F))
	case 8:
		a.WriteString("RTI")
	case 9:
		a.WriteString("NOT ")
		a.WriteString(fmt.Sprintf("R%d ", (instruction>>9)&0x7))
		a.WriteString(fmt.Sprintf("R%d", (instruction>>6)&0x7))
	case 10:
		a.WriteString("LDI ")
		a.WriteString(fmt.Sprintf("R%d ", (instruction>>9)&0x7))
		a.WriteString(fmt.Sprintf("%#03x", instruction&0x1FF))
	case 11:
		a.WriteString("STI ")
		a.WriteString(fmt.Sprintf("R%d ", (instruction>>9)&0x7))
		a.WriteString(fmt.Sprintf("%#03x", instruction&0x1FF))
	case 12:
		a.WriteString("JMP ")
		a.WriteString(fmt.Sprintf("R%d", (instruction>>6)&0x7))
	case 13:
		a.WriteString(".FILL ")
		a.WriteString(fmt.Sprintf("%#04x", instruction))
	case 14:
		a.WriteString("LEA ")
		a.WriteString(fmt.Sprintf("R%d ", (instruction>>9)&0x7))
		a.WriteString(fmt.Sprintf("%#03x", instruction&0x1FF))
	case 15:
		a.WriteString("TRAP ")
		trapVector := instruction & 0xFF
		switch trapVector {
		case 0x20:
			a.WriteString("GETC")
		case 0x21:
			a.WriteString("OUT")
		case 0x22:
			a.WriteString("PUTS")
		case 0x23:
			a.WriteString("IN")
		case 0x24:
			a.WriteString("PUTSP")
		case 0x25:
			a.WriteString("HALT")
		default:
			a.WriteString(fmt.Sprintf("%#02x", trapVector))
		}
	default:
	}
	return a.String()
}
