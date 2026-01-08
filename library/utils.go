package library

import (
	"fmt"
	"log"
	"net"
	"os"
	"path/filepath"
	"strconv"
)

const GFSRoot = "/tmp/gfs"

// StringSeparator All values in a map output file in single line are separated by this string
const StringSeparator = "%"

// GetMapOutputPath TODO placeholder
func GetMapOutputPath(taskId string, mapId, partition int32) string {
	return GFSRoot + "/mapreduce/" + taskId + "/map_" + strconv.Itoa(int(mapId)) + "_part_" + strconv.Itoa(int(partition)) + ".out"
}

func GetReduceOutputPath(taskId string, partition int32) string {
	return GFSRoot + "/mapreduce/" + taskId + "/reduce_" + strconv.Itoa(int(partition)) + ".out"
}

func DownloadGFSFile(path, localPath string) (*os.File, error) {
	file, err := os.Open(path)
	if err != nil {
		panic(err)
	}
	return file, err
}

func SaveToGFS(path string, data []byte) {
	// Implement by writing data to file at path
	// create new file at path
	// if folders in path do not exist, create them first
	// Extract the directory part of the file path
	dir := filepath.Dir(path)

	// Create all directories if they don't exist
	if err := os.MkdirAll(dir, 0755); err != nil {
		fmt.Println("Error creating directories:", err)
		return
	}

	file, err := os.Create(path)
	if err != nil {
		panic(err)
	}
	defer file.Close()

	_, err = file.Write(data)
	if err != nil {
		panic(err)
	}
}

func GetOutboundIP() net.IP {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()

	localAddr := conn.LocalAddr().(*net.UDPAddr)

	return localAddr.IP
}
