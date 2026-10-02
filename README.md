## ToyVM 

VM for the [Little Computer 3](https://en.wikipedia.org/wiki/Little_Computer_3). This is created for the purpose of learning about VMs, disassemblers and property based testing. 

To test out the LC3 instruction implementation, I am using 2 kind of tests: 

1. Property based tests which makes use of [Hegel](https://hegel.dev/)'s library for Golang, [hegel-go](https://github.com/hegeldev/hegel-go), found in [instructions_property_test.go](./instructions_property_test.go).  
2. Traditional example based tests, found in [instructions_test.go](./instructions_test.go) 

## Usage of LLMs

The VM is written entirely by hand. 

The tests (both table based tests and property based tests) are written with the aid of LLMs. 

