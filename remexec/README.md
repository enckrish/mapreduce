remexec — quick start

This repository contains a small listener that accepts an executable over TCP, runs it, and streams back framed output (stdout/stderr/exit). A `client` package exposes a Go-friendly iterator-style API to call the listener and read the frames.

This README collects the crucial commands to build and run the listener, capture logs, run the caller, and use the client iterator from Go code.

Prerequisites
- Go 1.25 or newer
- Unix-like shell (examples use bash)

Repository layout (relevant files)
- `listener/main.go` — TCP server that accepts executables and streams framed output
- `client/client.go` — client package exposing an iterator (Seq2-style) and `DialAndSend`
- `caller/main.go` — example CLI that uses `client.DialAndSend` and prints frames to local stdio
- `testprog/main.go` — small test program used for the demo

Build

From the repo root:

```bash
# Build the test program used in the example
go build -o testprog_bin ./testprog

# Build the listener (listener has its own module)
cd listener
go build -o ../listener_bin .
cd ..

# Build the caller that demonstrates the client usage
go build -o caller_bin ./caller
```

Run the listener and store logs

Start the listener in the background and redirect its output to a log file:

```bash
nohup ./listener_bin > listener.log 2>&1 &
# prints background PID; you can also capture it with:
# pid=$(nohup ./listener_bin > listener.log 2>&1 & echo $!)
```

Check the listener logs:

```bash
tail -f listener.log
# or see recent contents
tail -n 200 listener.log
```

Confirm the listener process is running (replace listener_bin with the actual binary name):

```bash
pgrep -af listener_bin
```

Stopping the listener:

```bash
# If you printed/stored the PID earlier
kill <PID>
# or find & kill
pkill -f listener_bin
```

Run the caller (send an executable to the listener)

The `caller` demonstrates sending an executable to the running listener and streaming back frames (stdout/stderr/exit). Example using the test binary built above:

```bash
./caller_bin -exe ./testprog_bin
```

You should see the test program's stdout/stderr printed by the caller and a remote exit code. Example output (observed in the example run used during development):

```
hello from testprog stdout
this is testprog stderr

remote exit code: 3
```

(If your `testprog` sleeps before printing later output you may see additional lines after a pause.)

Using the client package from Go code

The `client` package contains a generic iterator implementation and a helper `DialAndSend`:

- `DialAndSend(addr, exePath string, args []string) (*Iterator[Frame], error)`
  - Sends the executable bytes and args to the listener and returns an iterator over `Frame` values.
- `Iterator[Frame]` supports:
  - `Next() (Frame, bool)` — returns (value, true) while frames are available, (zero, false) when done
  - `Err() error` — returns the first error encountered while iterating, if any
  - `Close() error` — close underlying resources (safe to call multiple times)

Frame format (as packaged by the `listener`)
- Each frame is serialized as:
  - 1 byte: frame type
    - 0 = stdout
    - 1 = stderr
    - 2 = exit
  - 4 bytes: big-endian payload length (uint32)
  - N bytes: payload bytes
- For exit frames the payload is a 4-byte big-endian exit code.

Example usage (copy into a Go program):

```go
package main

import (
    "encoding/binary"
    "fmt"
    "log"
    "os"

    "github.com/enckrish/remexec/client"
)

func main() {
    it, err := client.DialAndSend(client.DefaultAddr, "./myprogram", []string{"arg1"})
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
                fmt.Fprintf(os.Stderr, "remote exit code: %d\n", code)
                os.Exit(code)
            }
        }
    }
}
```

Notes and important details
- The `listener` runs the uploaded executable with a hard-coded 60s timeout (context in `listener/main.go`). If you need longer runs change the timeout there.
- `client.DialAndSend` currently reads the entire executable into memory (via `os.ReadFile`) before sending. For very large binaries you may want a streaming upload instead.
- The `listener` saves the uploaded executable to a temp file and sets execute permissions on it before running. The temp file is removed after execution.
- If you change the `Port` constant in `listener/main.go`, update `client.DefaultAddr` or pass the `-addr` flag to `caller`.

Troubleshooting
- If the caller can't connect: ensure the listener is running and reachable on the configured port; check `listener.log` for listening errors.
- Permission denied while running the saved binary: ensure the listener process has permission to create and execute files in the temp directory.
- To inspect a live connection: check `listener.log` and use `strace`/`lsof` if necessary.

Next steps / improvements you might want
- Stream the executable bytes from the client instead of reading whole file into memory.
- Make the listener timeout configurable via flag or a parameter.
- Add unit tests for the `client` iterator and integration tests that spin up the listener in-process.

If you want, I can add a short `Makefile` with build/run targets and a convenience script to start/stop the listener and rotate logs.
