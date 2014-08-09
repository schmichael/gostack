package gostack

import (
	"log"
	"sync"
)

var (
	debug     func(msg string, params ...interface{}) = noop
	debugLock sync.Mutex
)

// Debug puts gostack into debug mode which outputs verbose logging.
func Debug(on bool) {
	debugLock.Lock()
	if on {
		debug = dlog
	} else {
		debug = noop
	}
	debugLock.Unlock()
}

func noop(_ string, _ ...interface{}) {}

func dlog(msg string, params ...interface{}) {
	log.Printf(msg, params...)
}
