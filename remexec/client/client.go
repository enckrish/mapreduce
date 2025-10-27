package client

import (
	"bufio"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
)

// Iterator provides iteration over values of type T.
// Next returns the next value and true while there are values. When iteration
// finishes Next returns the zero value and false. If an error occurs during
// iteration it is recorded and returned by Err().
type Iterator[T any] struct {
	next  func() (T, bool, error)
	close func() error
	err   error
}

// New creates an Iterator from a next function and an optional close function.
// next should return (value, ok, err). When next returns err the iterator will
// record it and Next will return ok=false.
func New[T any](next func() (T, bool, error), close func() error) *Iterator[T] {
	if next == nil {
		panic("iter: next function cannot be nil")
	}
	return &Iterator[T]{next: next, close: close}
}

// Next returns the next value and true while there are values. When the
// sequence is exhausted Next returns the zero value and false.
func (it *Iterator[T]) Next() (T, bool) {
	var zero T
	if it == nil {
		return zero, false
	}
	if it.err != nil {
		return zero, false
	}
	v, ok, err := it.next()
	if err != nil {
		it.err = err
		return zero, false
	}
	if !ok {
		return zero, false
	}
	return v, true
}

// Err returns the first error encountered by the iterator, if any.
func (it *Iterator[T]) Err() error { return it.err }

// Close closes the iterator, releasing any resources. It is safe to call
// multiple times.
func (it *Iterator[T]) Close() error {
	if it == nil {
		return nil
	}
	if it.close == nil {
		return nil
	}
	if err := it.close(); err != nil {
		return fmt.Errorf("iter close: %w", err)
	}
	return nil
}

// DefaultAddr is the address of the listener (the listener in this repo).
const DefaultAddr = "127.0.0.1:4000"

// FrameType represents the type of frame returned by the listener.
type FrameType byte

const (
	TypeStdout FrameType = 0
	TypeStderr FrameType = 1
	TypeExit   FrameType = 2
)

// Frame is a single unit returned by the iterator.
type Frame struct {
	Type FrameType
	Data []byte
}

// DialAndSend connects to the listener at addr, sends the executable found at exePath
// and the provided args, and returns an Iterator[Frame] to read frames produced by the listener.
// Example:
//
//	it, err := DialAndSend(client.DefaultAddr, "./myprog", []string{"arg1"})
//	for {
//	    f, err := it.Next()
//	    if err == io.EOF { break }
//	    if err != nil { log.Fatal(err) }
//	    // handle f
//	}
func DialAndSend(addr, exePath string, args []string) (*Iterator[Frame], error) {
	exeBytes, err := os.ReadFile(exePath)
	if err != nil {
		return nil, fmt.Errorf("read exe: %w", err)
	}

	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("dial %s: %w", addr, err)
	}

	// send exe length and bytes
	if err := writeBytesWithLen(conn, exeBytes); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("send exe: %w", err)
	}

	// send params (JSON array of strings)
	paramsBytes, err := json.Marshal(args)
	if err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("marshal params: %w", err)
	}
	if err := writeBytesWithLen(conn, paramsBytes); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("send params: %w", err)
	}

	r := bufio.NewReader(conn)

	// next function for the generic iterator
	next := func() (Frame, bool, error) {
		var zero Frame
		// read header
		hdr := make([]byte, 5)
		if _, err := io.ReadFull(r, hdr); err != nil {
			_ = conn.Close()
			if err == io.EOF {
				return zero, false, nil
			}
			return zero, false, fmt.Errorf("read header: %w", err)
		}
		typ := FrameType(hdr[0])
		length := int(binary.BigEndian.Uint32(hdr[1:5]))
		if length < 0 {
			_ = conn.Close()
			return zero, false, fmt.Errorf("negative length: %d", length)
		}
		var data []byte
		if length > 0 {
			data = make([]byte, length)
			if _, err := io.ReadFull(r, data); err != nil {
				_ = conn.Close()
				return zero, false, fmt.Errorf("read payload: %w", err)
			}
		}

		f := Frame{Type: typ, Data: data}
		if typ == TypeExit {
			// treat exit as last frame; return it once then end iteration
			// return the exit frame to caller; subsequent Next() should return ok=false
			// but we need to close connection after delivering this frame
			_ = conn.Close()
			return f, true, nil
		}
		return f, true, nil
	}

	closeFn := func() error { return conn.Close() }

	it := New(next, closeFn)
	return it, nil
}

func writeBytesWithLen(w io.Writer, data []byte) error {
	var lbuf [4]byte
	binary.BigEndian.PutUint32(lbuf[:], uint32(len(data)))
	if _, err := w.Write(lbuf[:]); err != nil {
		return err
	}
	if len(data) > 0 {
		_, err := w.Write(data)
		return err
	}
	return nil
}
