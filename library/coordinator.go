package library

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"google.golang.org/grpc"
)

const ExecutorsListPath = "/.mapreduce_executors"
const CoordinatorRPCPort = ":9001"

type Coordinator struct {
	obj           *Object
	avblExecutors chan string
	executors     map[string]*Executor

	redStreams   []grpc.ServerStreamingServer[MapCompleted]
	redTasksDone int32
	mappersDone  int32

	workerLock  sync.Mutex
	mapsDoneWg  sync.WaitGroup
	redsAwakeWg sync.WaitGroup
	finished    sync.Mutex

	conn *grpc.Server
}

func NewCoordinator(obj *Object) *Coordinator {
	log.SetPrefix(fmt.Sprintf("%-11s: ", "Coordinator"))
	c := &Coordinator{
		obj:        obj,
		redStreams: make([]grpc.ServerStreamingServer[MapCompleted], 0),
		// channel should be large enough to hold all executors
		avblExecutors: make(chan string, obj.params.NumMappers+obj.params.NumPartitions),
		executors:     make(map[string]*Executor),
	}
	c.obj.params.CoordinatorAddr = GetOutboundIP().String() + CoordinatorRPCPort
	log.Println("Discovered coordinator address:", c.obj.params.CoordinatorAddr)
	if err := c.setupExecutors(); err != nil {
		panic(err)
	}

	return c
}

func (c *Coordinator) Run() error {
	c.finished.Lock()
	c.startRPCServer()
	time.Sleep(0 * time.Second)

	c.redsAwakeWg.Add(int(c.obj.params.NumPartitions))
	c.mapsDoneWg.Add(int(c.obj.params.NumMappers))

	// We setup reducers first, so that they are always ready to receive
	c.setupReducers()
	c.redsAwakeWg.Wait()
	//// Mappers may be reassigned to a different MapperId once the prior is finished
	c.runMappers()

	c.finished.Lock()
	return nil
}

func (c *Coordinator) setupExecutors() error {
	log.Println("Setting up executors...")
	data, err := os.ReadFile(ExecutorsListPath)
	if err != nil {
		panic(err)
	}

	totalExecs := 0
	// Read executor addresses from file, where they are in the format:
	// ip_address:port max_jobs
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) == 0 {
			continue
		}
		addr := parts[0]
		maxJobs, _ := strconv.Atoi(parts[1])
		totalExecs += maxJobs
		c.executors[addr] = &Executor{addr: addr, maxJobs: maxJobs}
		c.avblExecutors <- addr
	}

	log.Println("Setting up executors completed, total executors available:", totalExecs)

	return nil
}

func (c *Coordinator) setupReducers() {
	// Assign reducers
	for i := range c.obj.params.NumPartitions {
		exec := <-c.avblExecutors

		p := c.obj.params.Copy()
		p.Mode = ModeReduce
		p.Partition = i
		go c.executors[exec].AssignJob(p, c.avblExecutors)
		log.Println("Assigned reducer for Partition", i)
	}
	log.Println("All reducers assigned")
}

func (c *Coordinator) runMappers() {
	// Assign mappers
	for i := range c.obj.params.NumMappers {
		exec := <-c.avblExecutors

		p := c.obj.params.Copy()
		p.Mode = ModeMap
		p.MapperId = i
		go c.executors[exec].AssignJob(p, c.avblExecutors)
		log.Println("Assigned mapper for Partition", i)
	}
	log.Println("All mappers assigned")
}

func (c *Coordinator) startRPCServer() {
	log.Println("Starting gRPC server...")
	lis, err := net.Listen("tcp", CoordinatorRPCPort)
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
		os.Exit(7)
	}
	c.conn = grpc.NewServer()

	RegisterCoordinatorServiceServer(c.conn, c)
	go func() {
		if err := c.conn.Serve(lis); err != nil {
			log.Fatalf("failed to serve: %v", err)
			os.Exit(8)

		}
	}()
}

// RPC Methods

func (c *Coordinator) RegisterReducer(_ *Empty, g grpc.ServerStreamingServer[MapCompleted]) error {
	c.workerLock.Lock()
	c.redStreams = append(c.redStreams, g)
	log.Printf("Registered reducer %d/%d\n", len(c.redStreams), c.obj.params.NumPartitions)
	c.workerLock.Unlock()

	c.redsAwakeWg.Done()

	// wait here until all mappers are done, and their output sent to the reducers
	c.mapsDoneWg.Wait()

	return nil
}

func (c *Coordinator) NotifyMapCompleted(_ context.Context, completed *MapCompleted) (*Empty, error) {
	go c.onMapperComplete(completed.Id)
	return &Empty{}, nil
}

func (c *Coordinator) NotifyReduceCompleted(_ context.Context, completed *ReduceCompleted) (*Empty, error) {
	c.onReducerComplete(completed.Partition)
	return &Empty{}, nil
}

func (c *Coordinator) mustEmbedUnimplementedCoordinatorServiceServer() {
	panic("unexpected call")
}

func (c *Coordinator) onMapperComplete(mapperID int32) {
	log.Printf("mapper %d completed\n", mapperID)
	c.workerLock.Lock()
	c.mappersDone++
	log.Printf("%d/%d mappers done\n", c.mappersDone, c.obj.params.NumMappers)
	c.workerLock.Unlock()

	if c.mappersDone == c.obj.params.NumMappers {
		log.Println("Map phase completed")
	}

	for _, r := range c.redStreams {
		err := r.Send(&MapCompleted{Id: mapperID})
		if err != nil {
			return
		}
	}

	log.Printf("All reducers notified of map id: %d completion\n", mapperID)

	c.mapsDoneWg.Done()
}

func (c *Coordinator) onReducerComplete(p int32) {
	log.Printf("reducer %d completed\n", p)
	c.workerLock.Lock()
	c.redTasksDone++
	if c.redTasksDone > c.obj.params.NumPartitions {
		panic("too many reducers completed")
	}

	if c.redTasksDone == c.obj.params.NumPartitions {
		log.Println("Reducer phase completed")
		log.Println("Shutting down gRPC server...")
		c.conn.Stop()
		log.Println("gRPC server stopped")
		log.Println("Exiting coordinator...")
		c.finished.Unlock()
		//os.Exit(0)
	}
	c.workerLock.Unlock()
}
