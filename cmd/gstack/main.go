package main

import (
	"fmt"
	"os"

	"github.com/schmichael/gostack"
)

func main() {
	p, err := gostack.ReadProfile(os.Stdin)
	if err != nil {
		fmt.Println("Error reading goroutine stack profile: %v", err)
		os.Exit(2)
	}
	fmt.Println(p)
}
