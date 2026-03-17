package main

import "os"

// Exit codes matching jq conventions.
const (
	exitOK       = 0
	exitUsage    = 2 // bad flags, I/O errors
	exitCompile  = 3 // filter parse/compile error
	exitNoOutput = 4 // --exit-status with no output
	exitRuntime  = 5 // jq filter runtime error
)

var version = "dev"

func main() {
	os.Exit(run())
}
