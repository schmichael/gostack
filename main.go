package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"strconv"
	"time"
)

type GoroutineState string

const (
	Running       GoroutineState = "running"
	ChanRecv      GoroutineState = "chan receive"
	ChanSend      GoroutineState = "chan send"
	FinalizerWait GoroutineState = "finalizer wait"
	Runnable      GoroutineState = "runnable"
	Select        GoroutineState = "select"
	Sleep         GoroutineState = "sleep"
	Syscall       GoroutineState = "syscall"
	IOWait        GoroutineState = "IO wait"
)

var (
	gStateEnd  = []byte{']', ':', '\n'} // end of goroutine state
	commaSpace = []byte{',', ' '}
)

type Profile struct {
	Created    time.Time
	Goroutines []*Goroutine
}

type Goroutine struct {
	ID      int
	State   GoroutineState
	Blocked int
	Stack   []*StackFrame
}

type StackFrame struct {
	Line1      string
	Line2      string
	Package    string
	Method     string   //TODO use a struct to differentiate between func & method?
	Parameters []string //TODO uniptr? try to look it up in the heap?
	SourceFile string
	LineNumber int
}

// scanGState implements bufio.SplitFunc to parse the Goroutine State.
func scanGState(data []byte, atEOF bool) (int, []byte, error) {
	if len(data) == 0 && atEOF {
		return 0, nil, fmt.Errorf("unexpected EOF")
	}
	if len(data) >= 1 && data[0] != '[' {
		return 0, nil, fmt.Errorf(`expected "[" but encountered %q`, data[0])
	}
	for i, b := range data[1:len(data)] {
		if b == ',' || b == ']' {
			// Found end of Goroutine state!
			return i + 1, data[1 : i+1], nil
		}
		if (b < 'A' || b > 'Z') && (b < 'a' || b > 'z') && b != ' ' {
			return 0, nil, fmt.Errorf(`invalid character in goroutine state: %q`, b)
		}
	}
	// Didn't find a Goroutine state terminator (',' or ']'), get more data
	if atEOF {
		return 0, nil, fmt.Errorf("unexpected EOF")
	}
	return 0, nil, nil
}

// scanBlocked implements bufio.SplitFunc to parse the Goroutine State.
func scanBlocked(data []byte, atEOF bool) (int, []byte, error) {
	if len(data) == 0 && atEOF {
		return 0, nil, fmt.Errorf("unexpected EOF")
	}
	if len(data) >= 1 && data[0] == ']' {
		// No blocked duration, gobble trailing "]:\n" before exiting
		if len(data) < 3 {
			return 0, nil, nil
		}
		// Sanity check
		if !bytes.Equal(data[0:3], gStateEnd) {
			return 0, nil, fmt.Errorf("expected %q but found %q", gStateEnd, data[0:3])
		}
		return len(gStateEnd), []byte{}, nil
	}

	// Looks like there's a blocked duration or not enough bytes
	if len(data) >= len(", N minutes]:\n") {
		if !bytes.Equal(data[0:2], commaSpace) {
			return 0, nil, fmt.Errorf(`expected %q but found %q`, commaSpace, data[0:2])
		}
		for i, b := range data[2:len(data)] {
			if b == ' ' {
				// End of number
				return 2 + i + len(" minutes]:\n"), data[2 : 2+i], nil // 2 == len(commaSpace)
			}
			if b < '0' || b > '9' {
				// Something went wrong
				return 0, nil, fmt.Errorf(`expected 0-9 but found %q`, b)
			}
		}
	}

	// If there arne't enough bytes for the duration but are at the EOF,
	// something went wrong.
	if atEOF {
		return 0, nil, fmt.Errorf("unexpected EOF")
	}
	return 0, nil, nil
}

func ReadProfile(r io.Reader) (*Profile, error) {
	p := &Profile{Created: time.Now()}
	scanner := bufio.NewScanner(r)

	for {
		// New goroutine
		scanner.Split(bufio.ScanWords)
		if !scanner.Scan() {
			return p, fmt.Errorf(`expected "goroutine" when error occurred: %v`, scanner.Err())
		}
		if scanner.Text() != `goroutine` {
			return p, fmt.Errorf(`expected "goroutine" but found %q`, scanner.Text())
		}
		g := &Goroutine{}
		p.Goroutines = append(p.Goroutines, g)
		fmt.Printf("- Added goroutine (total: %d)\n", len(p.Goroutines))

		// Goroutine ID
		if !scanner.Scan() {
			return p, fmt.Errorf(`expected goroutine ID when error occurred: %v`, scanner.Err())
		}
		var err error
		g.ID, err = strconv.Atoi(scanner.Text())
		if err != nil {
			return p, fmt.Errorf(`goroutine ID "%s" could not be parsed: %v`, scanner.Text(), err)
		}
		fmt.Printf("- Goroutine ID: %d\n", g.ID)

		// State
		scanner.Split(scanGState)
		if !scanner.Scan() {
			return p, fmt.Errorf(`expected goroutine state when error occurred: %v`, scanner.Err())
		}
		g.State = GoroutineState(scanner.Text())
		fmt.Printf("- Goroutine state: %s\n", g.State)

		// Blocked duration
		scanner.Split(scanBlocked)
		if !scanner.Scan() {
			return p, fmt.Errorf(`expected goroutine blocked when error occurred: %v`, scanner.Err())
		}
		if len(scanner.Bytes()) > 0 {
			g.Blocked, err = strconv.Atoi(scanner.Text())
			if err != nil {
				return p, fmt.Errorf(`blocked duration "%s" could not be parsed: %v`, scanner.Text(), scanner.Err())
			}
			fmt.Printf("- Goroutine blocked: %d\n", g.Blocked)
		}

		// Stack frames
		scanner.Split(bufio.ScanLines)
		for {
			if !scanner.Scan() {
				if scanner.Err() != nil {
					return p, fmt.Errorf(`expected first stack frame line, blank line, or EOF when error occurred: %v`, scanner.Err())
				}
				// EOF, exit cleanly
				return p, nil
			}
			if len(scanner.Bytes()) == 0 {
				// End of Goroutine
				fmt.Println("break")
				break
			}
			s := &StackFrame{}
			g.Stack = append(g.Stack, s)
			s.Line1 = scanner.Text()
			fmt.Printf("- Stack line 1/%d: %.10s\n", len(g.Stack), s.Line1)
			if !scanner.Scan() {
				return p, fmt.Errorf(`expected first stack frame line when error occurred: %v`, scanner.Err())
			}
			s.Line2 = scanner.Text()
			fmt.Printf("- Stack line 2/%d: %.10s\n", len(g.Stack), s.Line2)
		}
		fmt.Println(">>> start over! <<<")
	}

	return p, nil
}

func main() {
	p, err := ReadProfile(os.Stdin)
	fmt.Println(p)
	fmt.Println(err)
}
