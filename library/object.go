package library

import (
	"iter"
	"os"
)

const ModeCoordinator = "coordinator"
const ModeMap = "map"
const ModeReduce = "reduce"

var RemoteCallerAddress string = os.Getenv("REME_CALLER_ADDR")
var ExecutablePath string = os.Getenv("REME_EXEC_PATH")

// ParseIter returns a Seq2 iterator from raw data
type ParseIter = func(filePath string, totalMappers, mapperId int32) iter.Seq2[string, string]
type MapFn = func(mapper *Mapper, key, value string)
type ReduceFn = func(reducer *Reducer, key string, values []string)

type Object struct {
	params  *TaskParams
	parser  ParseIter
	mapper  MapFn
	reducer ReduceFn
}

func NewObjectWithParams(params *TaskParams, parser ParseIter, mapper MapFn, reducer ReduceFn) *Object {
	return &Object{
		params:  params,
		parser:  parser,
		mapper:  mapper,
		reducer: reducer,
	}
}

func NewObject(parser ParseIter, mapper MapFn, reducer ReduceFn) *Object {
	o := &Object{
		parser:  parser,
		mapper:  mapper,
		reducer: reducer,
	}
	o.params = ParseCLIArgs(os.Args[1:])
	o.params.partitionFn = BuildSumHashFn(o.params.NumPartitions)

	return o
}

func (o *Object) Params() *TaskParams { return o.params }
func (o *Object) Parser() ParseIter   { return o.parser }
func (o *Object) Mapper() MapFn       { return o.mapper }
func (o *Object) Reducer() ReduceFn   { return o.reducer }

func (o *Object) Run() error {
	if ExecutablePath == "" {
		panic("executable path is empty")
	}

	mode := o.params.Mode
	switch mode {
	case ModeCoordinator:
		err := NewCoordinator(o).Run()
		if err != nil {
			panic(err)
		}
	case ModeMap:
		err := NewMapper(o).Run()
		if err != nil {
			panic(err)
		}
	case ModeReduce:
		err := NewReducer(o).Run()
		if err != nil {
			panic(err)
		}
	default:
		panic("unknown Mode: " + mode)
	}
	return nil
}
