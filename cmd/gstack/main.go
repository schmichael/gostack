package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/schmichael/gostack"
)

func main() {
	debug := flag.Bool("debug", false, "turn on verbose debug output")
	flag.Parse()
	gostack.Debug(*debug)
	p, err := gostack.ReadProfile(os.Stdin)
	if err != nil {
		fmt.Println("Error reading goroutine stack profile (was it created with debug=2?): %v", err)
		os.Exit(2)
	}

	// Compute states
	states := map[gostack.GoroutineState]int{}
	for _, g := range p.Goroutines {
		states[g.State]++
	}

	// Print
	for _, state := range gostack.GoroutineStates {
		fmt.Printf("%-15s %d\n", state, states[state])
		delete(states, state)
	}

	// Sanity check
	for state, v := range states {
		fmt.Printf("%-15s %d\n", state, v)
	}
}
