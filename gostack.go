package gostack

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"
)

// GoroutineState represents a recognized state of a goroutine.
type GoroutineState string

// Recognized goroutine states.
const (
	Running       GoroutineState = "running"
	Runnable      GoroutineState = "runnable"
	ChanRecv      GoroutineState = "chan receive"
	ChanSend      GoroutineState = "chan send"
	FinalizerWait GoroutineState = "finalizer wait"
	Select        GoroutineState = "select"
	Sleep         GoroutineState = "sleep"
	Syscall       GoroutineState = "syscall"
	IOWait        GoroutineState = "IO wait"
	SemAcquire    GoroutineState = "semacquire"
)

var (
	// GoroutineStates is a list of recognized goroutine states such as
	// "running" and "sleep".
	GoroutineStates = []GoroutineState{
		Running, Runnable, ChanRecv, ChanSend, Select, Sleep, Syscall, IOWait, FinalizerWait, SemAcquire,
	}

	gStateEnd  = []byte{']', ':', '\n'} // end of goroutine state
	commaSpace = []byte{',', ' '}
)

// Profile is a goroutine stack profile.
type Profile struct {
	Created    time.Time
	Goroutines []*Goroutine
	StackCount map[string]int // Count of bottom call in stack frames
}

// Goroutine is a goroutine's metadata and stack.
type Goroutine struct {
	ID      int
	State   GoroutineState
	Blocked int
	Stack   []*StackFrame
}

// StackFrame is a single frame in a goroutine's stack.
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
				adv, slc := 2+i+len(" minutes]:\n"), data[2:2+i] // 2 == len(commaSpace)
				if adv > len(data) {
					// buffer doesn't actually include entire blocking bit, so ask for more
					return 0, nil, nil
				}
				return adv, slc, nil
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

// ReadProfile parses a goroutine stack profile from an io.Reader and returns a
// Profile. A partial Profile is returned even when errors occur.
//
// Currently, this function only supports goroutine profiles generated with verbosity/debug level of
// 2. This means that the code that originally generated the goroutine profile should look like
// myProfile.WriteTo(w, 2) . The example contained in this package demonstrates this. Also see the
// docs for runtime/pprof/*Profile.WriteTo in the standard library.
//
// Call Debug(true) to see verbose output from profile parsing.
func ReadProfile(r io.Reader) (*Profile, error) {
	p := &Profile{
		Created:    time.Now(),
		StackCount: make(map[string]int),
	}
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
		debug("New goroutine (total: %d)\n", len(p.Goroutines))

		// Goroutine ID
		if !scanner.Scan() {
			return p, fmt.Errorf(`expected goroutine ID when error occurred: %v`, scanner.Err())
		}
		var err error
		g.ID, err = strconv.Atoi(scanner.Text())
		if err != nil {
			return p, fmt.Errorf(`goroutine ID "%s" could not be parsed: %v`, scanner.Text(), err)
		}
		debug("- Goroutine ID: %d\n", g.ID)

		// State
		scanner.Split(scanGState)
		if !scanner.Scan() {
			return p, fmt.Errorf(`expected goroutine state when error occurred: %v`, scanner.Err())
		}
		g.State = GoroutineState(scanner.Text())
		debug("- Goroutine state: %s", g.State)

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
			debug("- Goroutine blocked: %d", g.Blocked)
		}

		// Stack frames
		{
			scanner.Split(bufio.ScanLines)
			var s *StackFrame
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
					debug("End of Goroutine %d", g.ID)
					break
				}
				s = &StackFrame{}
				g.Stack = append(g.Stack, s)
				s.Line1 = scanner.Text()
				debug("- Stack line 1/%d: %.20s", len(g.Stack), s.Line1)

				if !scanner.Scan() {
					return p, fmt.Errorf(`expected first stack frame line when error occurred: %v`, scanner.Err())
				}
				s.Line2 = strings.TrimSpace(scanner.Text())
				debug("- Stack line 2/%d: %.20s", len(g.Stack), s.Line2)
			}
			if s != nil {
				p.StackCount[s.Line2]++
			}
		}
	}

	return p, nil
}
