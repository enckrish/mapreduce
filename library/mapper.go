package library

import (
	"context"
	"fmt"
	"log"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type Mapper struct {
	obj   *Object
	store []map[string][]string
}

func NewMapper(obj *Object) *Mapper {
	return &Mapper{
		obj:   obj,
		store: make([]map[string][]string, obj.params.NumPartitions),
	}
}

func (m *Mapper) Run() error {
	log.SetPrefix(fmt.Sprintf("%-11s: ", fmt.Sprintf("Mapper	%3d", m.obj.params.MapperId)))

	params := m.obj.Params()
	it := m.obj.Parser()(params.InputPath, params.NumMappers, params.MapperId)

	for k, v := range it {
		m.obj.Mapper()(m, k, v)
	}

	log.Println("Map completed. Committing store to disk.")
	err := m.CommitStore()
	if err != nil {
		return err
	}

	conn, err := grpc.NewClient(params.CoordinatorAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		panic(err)
	}

	defer conn.Close()

	client := NewCoordinatorServiceClient(conn)
	log.Println("Sending NotifyMapCompleted message.")
	_, err = client.NotifyMapCompleted(context.Background(), &MapCompleted{
		Id: params.MapperId,
	})
	if err != nil {
		panic(err)
	}

	return nil
}

func (m *Mapper) Emit(key, value string) {
	partition := m.obj.Params().partitionFn(key)
	if m.store[partition] == nil {
		m.store[partition] = make(map[string][]string)
	}
	m.store[partition][key] = append(m.store[partition][key], value)
}

// CommitStore saves the mapper's intermediate key-value pairs to the appropriate storage
func (m *Mapper) CommitStore() error {
	for partition, kvMap := range m.store {
		filePath := GetMapOutputPath(m.obj.Params().TaskID, m.obj.Params().MapperId, int32(partition))

		// Convert map to string, with each key-value pair on a new line, separated by StringSeparator
		var lines []string
		for k, vList := range kvMap {
			line := k + StringSeparator + strings.Join(vList, StringSeparator)
			lines = append(lines, line)
		}

		data := strings.Join(lines, "\n")

		SaveToGFS(filePath, []byte(data))
	}

	return nil
}
