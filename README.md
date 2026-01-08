# MapReduce Paper Implementation

MapReduce is a distributed computation framework for parallely processing datasets across multiple machines. The main aspect of it is the ease of use for the end-programmer, who doesn't need to be aware of the the behind-the-scenes parallelization and fault-tolerance and only has to define `map` and `reduce` functions for the task.

This implementation is mostly based on the Google's [MapReduce paper](https://static.googleusercontent.com/media/research.google.com/en//archive/mapreduce-osdi04.pdf), with some omissions and changes.

## Components
The following implementation is divided into 2 components:
### remexec
`remexec/listener` is a general-purpose daemon that runs on the worker machines, continuously listening for job scheduling requests on a TCP port. It receives an executable binary and its execution params on the TCP connection, and executes it on the machine that it is running on. Whn the program is running, it streams back the `STDOUT` and `STDERR` outputs of the running program. And when it terminates, it sends back the error code back to the entity which scheduled the execution.
`remexec/client` provides a stub for calling the `listener` and listening to the streamed outputs and returned error code. This is used by the MapReduce coordinator to programmatically spawn the mapper and reducer worker processes. 
`remexec/caller` uses `remexec/client` to provide a CLI interface for calling the `listener`. This is used by the programmer to schedule the Mapreduce coordinator process on a remote machine.
### library
This is the library that the end-programmer includes in his program to make it runnable as a MapReduce program. The steps involved are extremely minimal:
```go
// Declare a `map` function
func mapFn(m *library.Mapper, key, value string) { ... }

// Declare a `reduce` function
func reduceFn(r *library.Reducer, key string, values []string) { ... }

// In `main`, make a MapReduce `object` with these and run it
func main() {
    obj := library.NewObject(parse, mapFn, reduceFn)
    err := obj.Run()
    if err != nil {
        panic(err)
    }
}
```
An example of using these to execute a word count task can be found [here](./demo).

# Running
```bash
./remexec/caller_bin \    # compiled from `remexec/caller`
    localhost:4000 \      # host on which the listener is running
    ./demo/demo           # the binary of the user's program  
     --mode=coordinator \
     --task_id=word_count 
     --input_path="/tmp/data.txt" # path to input data
     --num_mappers=4 --num_partitions=16 # MapReduce parameters
```