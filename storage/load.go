package storage

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
)

type fileName string

const (
	Dir              fileName = "_diskStorage"
	ConnectionsPairs fileName = "IP:ID.json"
	TotalConnections fileName = "connections.json"
	FileLocations    fileName = "file:Location.json"
)

type connStroage struct {
	directory string
}

// This functions assumes that you want to overwrite the current contents of the file
func (c connStroage) SaveToDisk(fileName fileName, data interface{}) error {
	path := fmt.Sprintf("%s/%s", c.directory, fileName)
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return fmt.Errorf("cannot open file %w", err)
	}
	defer f.Close()
	jsonBytes, _ := json.Marshal(data)
	_, err = f.Write(jsonBytes)
	return err
}

// The input data type must be a pointer
func (c connStroage) LoadFromDisk(fileName fileName, dest interface{}) error {
	path := fmt.Sprintf("%s/%s", c.directory, fileName)
	f, err := os.OpenFile(path, os.O_RDONLY, 0644)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			fmt.Println("This should never happend. the caller should always pass in a valid file")
			panic(err)
		}
		return fmt.Errorf("file system err %w", err)
	}
	defer f.Close()
	jbytes, _ := io.ReadAll(f)
	err = json.Unmarshal(jbytes, dest)
	if err == nil {
		return err
	}
	return fmt.Errorf("json encoding error %w", err)

}
func NewStorage(dir string) *connStroage {
	if exist, _ := directoryExists(dir); !exist {

		if err := os.Mkdir(dir, 0755); err != nil {
			panic(err)
		}
	}
	return &connStroage{
		directory: dir,
	}
}

func directoryExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		// Path exists
		if _, err := os.Stat(path); err == nil && os.IsNotExist(err) {
			return false, nil
		}
		return true, nil
	} else if os.IsNotExist(err) {
		// Path does not exist
		return false, nil
	} else {
		// Some other error occurred
		return false, err
	}
}
