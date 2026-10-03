package main

import (
	"log/slog"
	"os"

	"github.com/containerd/console"
)

// setRawTerminal takes the terminal out of line mode for the whole run, a
// read then returns on the first key press instead of waiting for Enter.
func (vm *VM) setRawTerminal() error {
	current, err := console.ConsoleFromFile(os.Stdin)
	if err != nil {
		return err
	}

	if err := current.SetRaw(); err != nil {
		return err
	}

	vm.Terminal = current
	return nil
}

// restoreTerminal puts the terminal back in line mode.
func (vm *VM) restoreTerminal() {
	if vm.Terminal == nil {
		return
	}

	_ = vm.Terminal.Reset()
	vm.Terminal = nil
}

// readByte reads one byte from the terminal. It treats Ctrl+C as a request to stop.
func (vm *VM) readByte() byte {
	input := make([]byte, 1)
	if _, err := os.Stdin.Read(input); err != nil {
		return 0
	}

	if input[0] == 0x03 {
		slog.Info("halting")
		vm.Executing = Halted
	}

	return input[0]
}
