package main

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/exec"
	"sync"
	"time"
)

const Port = 4000 // global constant port number
const Timeout = 60 * time.Second

var createLock sync.Mutex

func main() {
	addr := fmt.Sprintf(":%d", Port)
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatalf("listen: %v", err)
	}
	log.Printf("listening on %s", addr)
	for {
		conn, err := ln.Accept()
		if err != nil {
			log.Printf("accept: %v", err)
			continue
		}
		go handleConn(conn)
	}
}

func handleConn(conn net.Conn) {
	// explicitly ignore Close() errors to satisfy linters
	defer func() { _ = conn.Close() }()

	// Read uint32 exe length (big-endian)
	var lbuf [4]byte
	if _, err := io.ReadFull(conn, lbuf[:]); err != nil {
		log.Printf("read exe length: %v", err)
		return
	}
	exeLen := int(binary.BigEndian.Uint32(lbuf[:]))
	if exeLen <= 0 {
		log.Printf("invalid exe length: %d", exeLen)
		return
	}

	// Read exe bytes
	exeBytes := make([]byte, exeLen)
	if _, err := io.ReadFull(conn, exeBytes); err != nil {
		log.Printf("read exe bytes: %v", err)
		return
	}

	// Read uint32 params length
	if _, err := io.ReadFull(conn, lbuf[:]); err != nil {
		log.Printf("read params length: %v", err)
		return
	}
	paramsLen := int(binary.BigEndian.Uint32(lbuf[:]))
	if paramsLen < 0 {
		log.Printf("invalid params length: %d", paramsLen)
		return
	}

	// Read params JSON bytes and unmarshal into []string
	paramsBytes := make([]byte, paramsLen)
	if paramsLen > 0 {
		if _, err := io.ReadFull(conn, paramsBytes); err != nil {
			log.Printf("read params bytes: %v", err)
			return
		}
	}
	var params []string
	if len(paramsBytes) > 0 {
		if err := json.Unmarshal(paramsBytes, &params); err != nil {
			log.Printf("unmarshal params: %v", err)
			return
		}
	}

	createLock.Lock()
	// Write exe to temp file
	tmpFile, err := os.CreateTemp("", "remexec-*")
	if err != nil {
		log.Printf("create temp file: %v", err)
		return
	}
	path := tmpFile.Name()
	if _, err := tmpFile.Write(exeBytes); err != nil {
		// ensure tmpFile and path are cleaned up, explicitly ignore errors
		_ = tmpFile.Close()
		_ = os.Remove(path)
		log.Printf("write exe: %v", err)
		return
	}
	// explicitly ignore Close error
	_ = tmpFile.Close()
	if err := os.Chmod(path, 0755); err != nil {
		_ = os.Remove(path)
		log.Printf("chmod: %v", err)
		return
	}
	defer func() { _ = os.Remove(path) }()

	// Run the executable with a timeout and stream its output
	// TODO this may need to be more configurable
	ctx, cancel := context.WithTimeout(context.Background(), Timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, path, params...)

	// Add temporary executable path and caller address to environment
	cmd.Env = append(
		os.Environ(),
		fmt.Sprintf("REME_EXEC_PATH=%s", path),
		fmt.Sprintf("REME_CALLER_ADDR=%s", conn.RemoteAddr().String()),
	)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		log.Printf("stdout pipe: %v", err)
		return
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		log.Printf("stderr pipe: %v", err)
		return
	}

	if err := cmd.Start(); err != nil {
		log.Printf("start cmd: %v", err)
		return
	}

	createLock.Unlock()

	type frame struct {
		typ  byte   // 0=stdout,1=stderr,2=exit
		data []byte // payload
	}

	frameCh := make(chan frame, 16)
	var wg sync.WaitGroup

	copyPipe := func(typ byte, r io.Reader) {
		defer wg.Done()
		buf := make([]byte, 32*1024)
		for {
			n, err := r.Read(buf)
			if n > 0 {
				// copy the bytes because buf will be reused
				b := make([]byte, n)
				copy(b, buf[:n])
				select {
				case frameCh <- frame{typ: typ, data: b}:
				default:
					// fallback: block if channel full to apply backpressure
					frameCh <- frame{typ: typ, data: b}
				}
			}
			if err != nil {
				if err == io.EOF {
					return
				}
				log.Printf("read pipe (type %d): %v", typ, err)
				return
			}
		}
	}

	wg.Add(2)
	go copyPipe(0, stdout)
	go copyPipe(1, stderr)

	// writer goroutine: serializes frames to the connection
	writeErrCh := make(chan error, 1)
	go func() {
		for f := range frameCh {
			// header: 1 byte type + 4 bytes length (big-endian)
			hdr := make([]byte, 5)
			hdr[0] = f.typ
			binary.BigEndian.PutUint32(hdr[1:], uint32(len(f.data)))
			if _, err := conn.Write(hdr); err != nil {
				writeErrCh <- fmt.Errorf("write header: %w", err)
				return
			}
			if len(f.data) > 0 {
				if _, err := conn.Write(f.data); err != nil {
					writeErrCh <- fmt.Errorf("write data: %w", err)
					return
				}
			}
		}
		writeErrCh <- nil
	}()

	// wait for pipes to be fully read
	wg.Wait()
	// close frameCh to signal writer that no more data frames will arrive
	// but we still need to send the exit frame

	// Wait for command to exit and determine code
	var exitCode int32 = 0
	if err := cmd.Wait(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			if status, ok := exitErr.Sys().(interface{ ExitStatus() int }); ok {
				exitCode = int32(status.ExitStatus())
			} else {
				exitCode = -1
			}
		} else {
			// could be context deadline or other error
			exitCode = -1
		}
	}

	// send exit frame: type=2, payload = 4-byte big-endian exit code
	exitPayload := make([]byte, 4)
	binary.BigEndian.PutUint32(exitPayload, uint32(exitCode))
	// try to send via frameCh; if writer already exited due to write error, skip
	select {
	case frameCh <- frame{typ: 2, data: exitPayload}:
		// sent
	default:
		// if channel is full, send blocking to ensure exit frame is delivered
		frameCh <- frame{typ: 2, data: exitPayload}
	}

	// close channel to let writer finish
	close(frameCh)

	// wait for writer result
	if werr := <-writeErrCh; werr != nil {
		log.Printf("write error: %v", werr)
		return
	}
}
