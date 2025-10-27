package main

import (
	"fmt"
	"iter"

	"github.com/enckrish/library"
)

func parse(string, int32, int32) iter.Seq2[string, string] {
	//fmt.Println("parsefn called")
	// Return an dummy iterator
	return func(yield func(string, string) bool) {
		for i := 0; i < 2; i++ {
			for j := 0; j < 2; j++ {
				if !yield(fmt.Sprintf("key%d", i), fmt.Sprintf("value%d", j)) {
					return
				}
			}
		}
	}
}

func mapfn(m *library.Mapper, k, v string) {
	//fmt.Println("mapfn called on key-value pair:", k, v)
	m.Emit(k, v)
}

func redfn(r *library.Reducer, k string, vs []string) {
	//fmt.Println("redfn called on key:", k, "with values:", vs)
	r.Emit(k, vs[0])
}

func main() {
	obj := library.NewObject(parse, mapfn, redfn)
	obj.Run()
}
