package main

import (
	"flag"
	"fmt"
	"os"
	"sort"

	"github.com/schmichael/gostack"
)

type StackCount struct {
	Caller string
	Count  int
}

type StackCountList struct {
	List []*StackCount
}

func NewStackCountList(n int) *StackCountList {
	s := &StackCountList{List: make([]*StackCount, n)}
	for i := range s.List {
		s.List[i] = &StackCount{}
	}
	return s
}

func (s *StackCountList) Len() int           { return len(s.List) }
func (s *StackCountList) Less(i, j int) bool { return s.List[i].Count < s.List[j].Count }
func (s *StackCountList) Swap(i, j int)      { s.List[i], s.List[j] = s.List[j], s.List[i] }

func (s *StackCountList) Add(caller string, count int) {
	if count > s.List[0].Count {
		s.List[0].Caller = caller
		s.List[0].Count = count
		sort.Sort(s)
	}
}

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
		fmt.Println("\nUnknown states")
		fmt.Printf("%-15s %d\n", state, v)
	}

	topN := 10
	if len(p.StackCount) < topN {
		topN = len(p.StackCount)
	}
	fmt.Printf("\nTop %d Goroutines\n", topN)
	top := NewStackCountList(topN)
	for caller, count := range p.StackCount {
		top.Add(caller, count)
	}
	for i := top.Len() - 1; i > 0; i-- {
		fmt.Printf("%-6d ... %s\n", top.List[i].Count, top.List[i].Caller)
	}
}
