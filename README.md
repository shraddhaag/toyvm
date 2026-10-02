## Toy VM 

VM for the [Little Computer 3](https://en.wikipedia.org/wiki/Little_Computer_3). This is created for the purpose of learning about VMs, disassemblers and property based testing. 

To test out the LC3 instruction implementation, I am using 2 kind of tests: 

1. Property based tests which makes use of [Hegel](https://hegel.dev/)'s library for Golang, [hegel-go](https://github.com/hegeldev/hegel-go), found in [instructions_property_test.go](./instructions_property_test.go).  
2. Traditional example based tests, found in [instructions_test.go](./instructions_test.go) 

## Emulator

Before implementing the VM itself, I wanted to understand: 
1. What is the disassembled output for an LC3 compatible object file. 
2. What does the changing registers and memory look like? 

Initially I manually implemented a [disassembler](./disassembler/). Later, 
I converted it into a full blown emulator! You can try it out here: [lc3.shraddhaag.dev](https://lc3.shraddhaag.dev). 

## Usage of LLMs

The VM is written entirely by hand. 

The tests (both table based tests and property based tests) and the emulator are written with the aid of LLMs. 
