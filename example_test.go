package gostack

import (
	"bytes"
	"fmt"
	"runtime/pprof"
)

func Example() {
	// In this example we'll just create a stack profile from this process in a slice in memory. You
	// could alternatively read a profile from a file or anywhere else.
	buf := bytes.NewBuffer(nil)
	if err := pprof.Lookup("goroutine").WriteTo(buf, 2); err != nil {
		fmt.Printf(err.Error())
		return
	}

	// fmt.Printf("profile is %s\n", buf.String())

	profile, err := ReadProfile(buf)
	if err != nil {
		fmt.Printf(err.Error())
		return
	}

	if len(profile.Goroutines) < 1 {
		fmt.Printf("Profile was parsed weirdly, how could there be no goroutines?")
		return
	}

	fmt.Printf("Success")
	// Output: Success
}
