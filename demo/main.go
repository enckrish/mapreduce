package main

import (
	"io"
	"iter"
	"os"
	"strconv"
	"strings"

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

func parse(filePath string, totalMappers int32, mapId int32) iter.Seq2[string, string] {
	// Read the file
	file, err := os.Open(filePath)
	if err != nil {
		panic(err)
	}

	fileInfo, err := file.Stat()
	if err != nil {
		panic(err)
	}

	sz := int32(fileInfo.Size())
	start := int64((sz / totalMappers) * mapId)
	end := int64((sz / totalMappers) * (mapId + 1))

	start = backToWordStart(file, start)
	end = backToWordStart(file, end)

	buf := make([]byte, end-start)
	_, err = file.ReadAt(buf, start)
	if err != nil {
		panic(err)
	}

	st := string(buf)
	st = strings.ToLower(st)
	words := strings.Fields(strings.ReplaceAll(st, ".", " "))
	return func(yield func(string, string) bool) {
		for _, w := range words {
			w = strings.TrimSpace(w)
			//if w == "optio" {
			//	fmt.Println("optio found parser")
			//}
			if !yield(filePath, w) {
				break
			}

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
