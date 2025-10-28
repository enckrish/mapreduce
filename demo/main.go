package main

import (
	"bufio"
	"fmt"
	"io"
	"iter"
	"os"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/enckrish/library"
)

// backToWordStart moves backward from offset to the first byte of the word
// that contains or precedes offset. It returns the new offset (0-based).
func backToWordStart(file *os.File, offset int64) int64 {
	if offset <= 0 {
		return 0
	}

	// get file size and clamp position
	fi, err := file.Stat()
	if err != nil {
		return offset
	}
	size := fi.Size()
	pos := offset
	if pos >= size {
		pos = size - 1
		if pos < 0 {
			return 0
		}
	}

	var b [1]byte

	isWordChar := func(c byte) bool {
		return (c >= 'A' && c <= 'Z') ||
			(c >= 'a' && c <= 'z') ||
			(c >= '0' && c <= '9') ||
			c == '_'
	}

	// Helper to read one byte at position p. Returns false on read error.
	readByte := func(p int64) (byte, bool) {
		if _, err := file.ReadAt(b[:], p); err != nil && err != io.EOF {
			return 0, false
		}
		return b[0], true
	}

	// If the byte at pos is word char, move back until previous is not a word char.
	if ch, ok := readByte(pos); ok && isWordChar(ch) {
		for pos > 0 {
			if prev, ok := readByte(pos - 1); ok && isWordChar(prev) {
				pos--
			} else {
				break
			}
		}
		return pos
	}

	// Otherwise, skip backwards over non-word chars until we find a word char,
	// then move back to the start of that word.
	for pos > 0 {
		if prev, ok := readByte(pos - 1); ok {
			pos--
			if isWordChar(prev) {
				for pos > 0 {
					if pch, ok := readByte(pos - 1); ok && isWordChar(pch) {
						pos--
					} else {
						break
					}
				}
				return pos
			}
		} else {
			break
		}
	}

	return 0
}

func ScanWords2(data []byte, atEOF bool) (advance int, token []byte, err error) {
	// Skip leading spaces.
	start := 0
	for width := 0; start < len(data); start += width {
		var r rune
		r, width = utf8.DecodeRune(data[start:])
		if !(unicode.IsSpace(r) || r == '.') {
			break
		}
	}
	// Scan until space, marking end of word.
	for width, i := 0, start; i < len(data); i += width {
		var r rune
		r, width = utf8.DecodeRune(data[i:])
		if unicode.IsSpace(r) || r == '.' {
			return i + width, data[start:i], nil
		}
	}
	// If we're at EOF, we have a final, non-empty, non-terminated word. Return it.
	if atEOF && len(data) > start {
		return len(data), data[start:], nil
	}
	// Request more data.
	return start, nil, nil
}

func GetStartOffset(file *os.File, totalMappers, mapId int32) int64 {
	fileInfo, err := file.Stat()
	if err != nil {
		panic(err)
	}
	sz := fileInfo.Size()
	start := (sz / int64(totalMappers)) * int64(mapId)

	start = backToWordStart(file, start)
	return start
}
func parse(filePath string, totalMappers int32, mapId int32) iter.Seq2[string, string] {
	// Read the file
	file, err := os.Open(filePath)
	if err != nil {
		panic(err)
	}

	start := GetStartOffset(file, totalMappers, mapId)
	end := GetStartOffset(file, totalMappers, mapId+1)
	_, err = file.Seek(start, 0)
	if err != nil {
		panic(nil)
	}

	scanner := bufio.NewScanner(file)
	scanner.Split(ScanWords2)
	offset := start
	return func(yield func(string, string) bool) {
		for scanner.Scan() {
			w := scanner.Text()
			offset += int64(len(w))
			if offset > end {
				break
			}
			w = strings.ToLower(w)
			if !yield(filePath, w) {
				break
			}
		}
		if scanner.Err() != nil {
			fmt.Println(scanner.Err())
		}
	}
}

func mapfn(m *library.Mapper, k, v string) {
	//fmt.Println("mapfn called on key-value pair:", k, v)
	m.Emit(v, "1")
}

func redfn(r *library.Reducer, k string, vs []string) {
	r.Emit(k, strconv.Itoa(len(vs)))
}

func main() {
	obj := library.NewObject(parse, mapfn, redfn)
	err := obj.Run()
	if err != nil {
		panic(err)
	}
}
