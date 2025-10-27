package main

import (
	"fmt"
	"os"
	"time"
)

func main() {
	fmt.Println("hello from testprog stdout")
	fmt.Fprintln(os.Stderr, "this is testprog stderr")
	fmt.Println("Environment variables:", os.Getenv("REME_EXEC_PATH"), os.Getenv("REME_CALLER_ADDR"))
	// This print will occur after 3 secs
	time.Sleep(3 * time.Second)
	fmt.Println("testprog completed after sleep")

	os.Exit(3)
}
