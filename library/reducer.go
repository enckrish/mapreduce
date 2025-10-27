package library

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type Reducer struct {
	obj          *Object
	mapTasks     chan int32
	store        map[string]string
	mapTasksDone int32
}

func NewReducer(obj *Object) *Reducer {
	return &Reducer{
		obj:      obj,
		mapTasks: make(chan int32, obj.params.NumMappers),
		store:    make(map[string]string),
	}
}

func (r *Reducer) Run() error {
	log.SetPrefix(fmt.Sprintf("Reducer\t%4d: ", r.obj.params.Partition))
	conn, err := grpc.NewClient(r.obj.params.CoordinatorAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		panic(err)
	}

	defer conn.Close()

	client := NewCoordinatorServiceClient(conn)
	log.Println("Sending RegisterReducer request")
	mapsDone, err := client.RegisterReducer(context.Background(), &Empty{})
	if err != nil {
		panic(err)
	}
	log.Println("Registered Reducer")

	go r.ProcessMapCompletions()

	for {
		task, err := mapsDone.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			panic(err)
		}

		r.mapTasks <- task.Id
		r.mapTasksDone += 1
		if r.mapTasksDone == r.obj.params.NumMappers {
			break
		}
	}
	close(r.mapTasks)

	log.Println("Reducer done. Committing store to disk.")
	err = r.CommitStore()
	if err != nil {
		panic(err)
	}

	log.Println("Commit completed. Sending NotifyReduceCompleted message.")
	_, err = client.NotifyReduceCompleted(context.Background(), &ReduceCompleted{
		Partition: r.obj.params.Partition,
	})
	if err != nil {
		panic(err)
	}
	return nil
}

func (r *Reducer) ProcessMapCompletions() {
	// read from mapTasks channel
	for mId := range r.mapTasks {
		outp := GetMapOutputPath(r.obj.params.TaskID, mId, r.obj.params.Partition)

		file, err := DownloadGFSFile(outp, "")
		if err != nil {
			panic("failed to download map output")
		}

		// Read the downloaded file and print it line by line (without using Parser)
		scanner := bufio.NewScanner(file)

		for scanner.Scan() {
			line := scanner.Text()
			kvals := strings.Split(line, StringSeparator)
			if len(kvals) < 2 {
				panic("invalid line")
			}
			r.obj.reducer(r, kvals[0], kvals[1:])
		}
	}
}

func (r *Reducer) Emit(key string, value string) {
	r.store[key] = value
}

func (r *Reducer) CommitStore() error {
	store := r.store
	filePath := GetReduceOutputPath(r.obj.params.TaskID, r.obj.params.Partition)

	// Convert map to string, with each key-value pair on a new line, separated by StringSeparator
	var lines []string
	for k, v := range store {
		lines = append(lines, k+StringSeparator+v)
	}
	data := strings.Join(lines, "\n")

	// TODO Write data to GFS at filePath
	SaveToGFS(filePath, []byte(data))
	return nil
}
