package library

import (
	"log"
	"sync"
	"time"
)

const ExecutorTimeout = 60 // seconds
type Executor struct {
	addr      string // executor address, example: "102.168.0.1:8080"
	lastAlive int    // last alive timestamp
	maxJobs   int    // maximum number of concurrent jobs

	lock sync.Mutex
}

func (ee *Executor) IsAvailable() bool {
	return ee.maxJobs > 0
}

func (ee *Executor) IsActive() bool {
	return (time.Now().Second() - ee.lastAlive) < ExecutorTimeout
}

func (ee *Executor) AssignJob(params *TaskParams, execChan chan<- string) bool {
	if !ee.IsAvailable() {
		return false
	}
	ee.lock.Lock()
	ee.maxJobs--
	currCap := ee.maxJobs
	if currCap > 0 {
		// if the executor is still available, put it back to the available executors channel
		execChan <- ee.addr
	}
	ee.lock.Unlock()

	ec, err := SubmitTask(ee.addr, params)
	if err != nil {
		if params.Partition != -1 {
			log.Printf("Reducer for partition %d exited with errCode %d and error %s", params.Partition, ec, err)
		} else if params.MapperId != -1 {
			log.Printf("Mapper with id %d exited with errCode %d and error %s", params.MapperId, ec, err)
		}
	}

	ee.lock.Lock()
	ee.maxJobs++
	// if the executor just become available again, add it back to the available executors channel
	if currCap == 0 {
		execChan <- ee.addr
	}
	ee.lock.Unlock()

	return err == nil
}
