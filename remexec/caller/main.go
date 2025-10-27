package main

import (
	"encoding/binary"
	"fmt"
	"log"
	"os"

	"github.com/enckrish/remexec/client"
)

func main() {
	addr := os.Args[1]
	exe := os.Args[2]

	it, err := client.DialAndSend(addr, exe, os.Args[2:])
	if err != nil {
		log.Fatalf("DialAndSend: %v", err)
	}
	defer func() { _ = it.Close() }()

	for {
		f, ok := it.Next()
		if !ok {
			if err := it.Err(); err != nil {
				log.Fatalf("iterator error: %v", err)
			}
			break
		}
		switch f.Type {
		case client.TypeStdout:
			_, _ = os.Stdout.Write(f.Data)
		case client.TypeStderr:
			_, _ = os.Stderr.Write(f.Data)
		case client.TypeExit:
			if len(f.Data) >= 4 {
				code := int(binary.BigEndian.Uint32(f.Data[:4]))
				_, _ = fmt.Fprintf(os.Stderr, "\nremote exit code: %d\n", code)
				os.Exit(code)
			} else {
				_, _ = fmt.Fprintln(os.Stderr, "invalid exit payload")
				os.Exit(1)
			}
		default:
			_, _ = fmt.Fprintf(os.Stderr, "unknown frame type: %d\n", f.Type)
		}
	}
}
