package library

import (
	"fmt"
	"os"
)

// TaskParams NOTE: everytime you change this struct, update ArgSerialize and ParseCLIArgs accordingly
type TaskParams struct {
	CoordinatorAddr string `name:"coordinator_addr"` // address of the coordinator grpc server, optional in coordinator Mode
	TaskID          string `name:"task_id"`          // unique task identifier, needed when we store output of multiple tasks in the same output dir
	Mode            string `name:"mode"`             // "coordinator", "map" or "reduce"
	InputPath       string `name:"input_path"`       // optional in reduce Mode
	//outputPath      string `name:"output_path"`

	partitionFn func(key string) int32 `name:""` // ignore in serialization

	// Coordinator settings
	NumMappers    int32 `name:"num_mappers"`
	NumPartitions int32 `name:"num_partitions"` // same as number of reducers

	// Reducer settings
	Partition int32 `name:"partition"` // optional, only for reduce Mode

	// Mapper settings
	MapperId int32 `name:"mapper_id"` // optional, only for map Mode
	//fileOffset int32 `name:"file_offset"`
	//len        int32 `name:"len"`
}

// ArgSerialize serialize task params to CLI args for passing to remote executor
func (t *TaskParams) ArgSerialize() []string {
	args := []string{}

	if t.CoordinatorAddr != "" {
		args = append(args, fmt.Sprintf("--coordinator_addr=%s", t.CoordinatorAddr))
	}
	args = append(args, fmt.Sprintf("--task_id=%s", t.TaskID))
	args = append(args, fmt.Sprintf("--mode=%s", t.Mode))
	if t.InputPath != "" {
		args = append(args, fmt.Sprintf("--input_path=%s", t.InputPath))
	}
	args = append(args, fmt.Sprintf("--num_mappers=%d", t.NumMappers))
	args = append(args, fmt.Sprintf("--num_partitions=%d", t.NumPartitions))
	if t.Mode == ModeReduce {
		args = append(args, fmt.Sprintf("--partition=%d", t.Partition))
	}
	if t.Mode == ModeMap {
		args = append(args, fmt.Sprintf("--mapper_id=%d", t.MapperId))
	}

	return args
}

func (t *TaskParams) Copy() *TaskParams {
	tv := *t
	return &tv
}

// ReadCLIArgs reads all args in the form of "--key=value" and returns a map of key-value pairs
func ReadCLIArgs(args []string) map[string]string {
	argMap := make(map[string]string)
	for _, arg := range args {
		if len(arg) > 2 && arg[:2] == "--" {
			parts := splitAtFirstEqual(arg[2:])
			if len(parts) == 2 {
				argMap[parts[0]] = parts[1]
			} else {
				argMap[parts[0]] = ""
			}
		}
	}
	return argMap
}

// splitAtFirstEqual splits a string at the first '=' character
func splitAtFirstEqual(s string) []string {
	for i := 0; i < len(s); i++ {
		if s[i] == '=' {
			return []string{s[:i], s[i+1:]}
		}
	}
	return []string{s}
}

// ParseCLIArgs parses command line arguments into TaskParams
// Example args: ["--Mode=map", "--input=input.txt", "--output=output.txt", "--partitions=4"]
func ParseCLIArgs(args []string) *TaskParams {
	t := &TaskParams{}

	if _, ok := ReadCLIArgs(args)["help"]; ok {
		printHelp()
		os.Exit(0)
	}

	argMap := ReadCLIArgs(args)

	if tId, ok := argMap["task_id"]; ok {
		t.TaskID = tId
	} else {
		panic("task_id arg is required")
	}

	if mode, ok := argMap["mode"]; ok {
		t.Mode = mode
	} else {
		panic("Mode arg is required")
	}

	if cAddr, ok := argMap["coordinator_addr"]; ok {
		t.CoordinatorAddr = cAddr
	} else if t.Mode != ModeCoordinator {
		panic("in absence of coordinator_addr arg, Mode must be " + ModeCoordinator)
	}

	if input, ok := argMap["input_path"]; ok {
		t.InputPath = input
	} else if input != ModeReduce {
		panic("input_path arg is required in map and coordinator modes")
	}
	//
	//if output, ok := argMap["output"]; ok {
	//	t.outputPath = output
	//}

	if mappers, ok := argMap["num_mappers"]; ok {
		// Convert mappers to int
		var n int32
		_, err := fmt.Sscanf(mappers, "%d", &n)
		if err != nil {
			panic(fmt.Errorf("parsing mappers error: %s", err))
		}
		t.NumMappers = n
	} else {
		panic("num_mappers arg is required")
	}

	if partitions, ok := argMap["num_partitions"]; ok {
		// Convert partitions to int
		var n int32
		_, err := fmt.Sscanf(partitions, "%d", &n)
		if err != nil {
			panic(fmt.Errorf("parsing partitions error: %s", err))
		}
		t.NumPartitions = n
	} else {
		panic("num_partitions arg is required")
	}

	if partition, ok := argMap["partition"]; ok {
		// Convert Partition to int
		var n int32
		_, err := fmt.Sscanf(partition, "%d", &n)
		if err != nil {
			panic(fmt.Errorf("parsing Partition error: %s", err))
		}
		t.Partition = n
	} else if t.Mode == ModeReduce {
		panic("reduce Mode requires Partition arg")
	}

	if mapperId, ok := argMap["mapper_id"]; ok {
		// Convert MapperId to int
		var n int32
		_, err := fmt.Sscanf(mapperId, "%d", &n)
		if err != nil {
			panic(fmt.Errorf("parsing mapper_id error: %s", err))
		}
		t.MapperId = n
	} else if t.Mode == ModeMap {
		panic("map Mode requires mapper_id arg")
	}

	return t
}

func BuildSumHashFn(k int32) func(string) int32 {
	return func(key string) int32 {
		sum := int32(0)
		for i := 0; i < len(key); i++ {
			sum += int32(key[i])
		}
		return sum % k
	}
}

func printHelp() {
	// `default in` column tells which modes the flag is optional in
	fmt.Println("flag\t\t\toptional in\tusage")
	fmt.Println("----\t\t\t----------\t-----")
	fmt.Println("--coordinator_addr\tcoordinator\taddress of the coordinator grpc server")
	fmt.Println("--task_id\t\tnone\t\tunique task identifier")
	fmt.Println("--Mode\t\t\tnone\t\tMode of operation: coordinator, map, reduce")
	fmt.Println("--input_path\t\treduce\t\tpath of input data")
	fmt.Println("--num_mappers\t\tnone\t\tnumber of mappers to spawn")
	fmt.Println("--num_partitions\tnone\t\tnumber of partitions/reducers")
	fmt.Println("--Partition\t\tmap\t\tPartition id for reduce Mode")
	fmt.Println("--mapper_id\t\treduce\t\tmapper id for map Mode")
}
