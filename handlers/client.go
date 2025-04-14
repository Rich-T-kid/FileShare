package handlers

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"hash/fnv"
	"net"
	"os"
	"strconv"
	"strings"
	"time"
	//"github.com/google/uuid"
	//"github.com/google/uuid"
)

const (
	downloadDir = "FileDownloads"
)

var (
	// this is to remove the chance of storing duplicates
	// Maps hashID of file content to a  bool to show it alrady exist
	ErrContentAlreadyExist = func(fileID string) error {
		return errors.New(fmt.Sprintf("Content already exist in this distributed file Server.FileID:%s , No addition data was stored\n", fileID))
	} //errors.New("Content already exist in this distributed file Server. No addition data was stored\n")
	// This is used to detect if contents of file already exist in the distributed file server IDeally this would point to the hashID of where the contents lie
)

type User struct {
	operation string
}

func init() {
	err := os.MkdirAll(downloadDir, 0755)
	if err != nil {
		panic(fmt.Errorf("failed to create directory %s: %w", downloadDir, err))
	}
}

// Note to self. Unless you need to use bytes (images,videos) tranform into strings/asci before writing over wire
// Store case works -> :store \n FileName:string size:int \n file_in_bytes
// recieve case works -> :recieve \n Filename:string
// ALl logs here should go to the single log handler for program
func (u *User) HandleConnection(ctx context.Context, conn net.Conn) {
	// Two operations (Store , save file) (recieve, retrive file)
	reader, ok := ctx.Value(connReader{}).(*bufio.Reader)
	if !ok {
		conn.Write([]byte(ERRinternalServer("Missing Buffer reader from middleware").Error()))
		return
	}
	line, err := reader.ReadBytes('\n')
	if err != nil {
		fmt.Println(err)
		return
	}
	switch u.operation {
	// good to go
	// The the API that this proviedes and is given this works 100%
	case "store":
		fName, size, err := parseStore(line)
		if err != nil {
			conn.Write([]byte(invalidRequest(string(line))))
			writelog([]byte(err.Error()), "client store command failed")
			return
		}
		// TODO: Remove this in a second. The caller shouldnt have to worry about this and the hashID of the content should be enough to remove duplicates
		// at most only read in 32 MB's
		buffer := make([]byte, min(size, megaByte*32))
		n, err := reader.Read(buffer)
		if err != nil {
			writelog([]byte(err.Error()), "client store command failed")
			return
		}
		// Should be a way to not have to compute the hash twice
		err = checkDuplcateContent(buffer[:n])
		if err != nil {
			conn.Write([]byte(err.Error()))
		}
		fileID, err := storeFile(ctx, conn, fName, buffer[:n])
		if err != nil {
			conn.Write([]byte("error -> " + err.Error()))
			return
		}
		conn.Write([]byte(fmt.Sprintf("FileID:%s\nSuccsucfully wrote %d bytes to distributed file sever\n", fileID, n)))

	case "recieve":
		fileHashID, err := grabFileName(line)
		if err != nil {
			conn.Write([]byte(err.Error()))
		}
		// This needs to be changed to look for the fileID hash
		exist := fileExist(fileHashID)
		if !exist {
			conn.Write([]byte("Cannot retrive a file that doesnt already exist. Please recheck file name spelling\n"))
			return
		}
		content, err := retriveFile(ctx, conn, fileHashID)
		if err != nil {
			conn.Write([]byte(err.Error()))
			return
		}
		// pref
		conn.Write(content)
		conn.Write([]byte("Completed transfering file contents"))
	default:
		fmt.Println("This case should never be reached")
	}

}

// assume the file name passed in is unique
// for now just write to a file. later well add to its own directory
// later well split files into chunks and then store to directory
// then well handle sending it to other files later
// for now keep everything on one server
// FileMeta represents metadata about a file, including its chunks and other details.
type FileMeta struct {
	// Machine IP address mapped to the file chunk number
	// EX: 127.0.0.1:2 -> ip of 127.0.0.1 holds the second chunk of the file info
	MachienMapping map[IPADDR]int `json:"MachienMapping"`

	// Original file name that was passed in by external client
	OriginalFileName string `json:"OriginalFileName"`

	// Generated file UUID
	HashFileName string `json:"HashFileName"`

	// Last time the file was stored or retrieved
	LastEdited time.Time `json:"LastEdited"`

	// This is the size of the file in bytes
	TotSize int `json:"TotSize"`

	// Additional field to calculate the size of each piece

	// len(map)/TotSize + 1 for each piece at most
}

func checkDuplcateContent(content []byte) error {
	fileHashID := hashBytes(content)
	_, ok := fileMetaInfoMap[fileHashID]
	if ok {
		v := fileMetaInfoMap[fileHashID]
		// theres no need to continue of the file hash already be stored previously and exist within our distributed file server
		return ErrContentAlreadyExist(v.HashFileName)

	}
	return nil

}

// this should also check if the file contents already exist
// IE doing a hash
func storeFile(ctx context.Context, conn net.Conn, fileName string, content []byte) (hashID string, e error) {
	// TODO: For now keep writing it to local file storage, but once you have split logic
	// move to writing to the storage machine
	path := fmt.Sprintf("%s/%s", downloadDir, fileName)
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return "", err
	}
	_, err = f.Write(content)
	// New stuff below
	// This is where wed generate a fileID Hash here
	fileHashID := hashBytes(content)
	// This is where the machines would be choosen. not imporant right now
	machineMap, errors := splitToStorage(ctx, totalConnections, content)
	if len(errors) != 0 {
		return "", errors[0]
	}
	fInfo := &FileMeta{
		MachienMapping:   machineMap,
		OriginalFileName: fileName,
		HashFileName:     fileHashID,
		LastEdited:       time.Now().UTC(),
		TotSize:          len(content),
	}
	fileMetaInfoMap[fileHashID] = fInfo
	return fileHashID, err
}
func retriveFile(ctx context.Context, conn net.Conn, fileHashID string) ([]byte, error) {
	metaInfo := fileMetaInfoMap[fileHashID]
	if metaInfo == nil {
		return nil, fmt.Errorf("file with ID %s not found", fileHashID)
	}
	// For now since we havent actually sent data to over the wire
	fileName := metaInfo.OriginalFileName
	path := fmt.Sprintf("%s/%s", downloadDir, fileName)
	f, err := os.OpenFile(path, os.O_RDONLY, 0644)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("%s does not exist in the %s directory", fileName, downloadDir)
		} else {
			return nil, err
		}
	}
	defer f.Close()
	/* add after implementing the send and recieve logic
	//adding an extra 100 bytes in the rare case that for what ever reason the totsize is too small.
	buffer := make([]byte, metaInfo.TotSize+100)
	err = recieveSplitStorage(ctx, *metaInfo, buffer)
	if err != nil {
		return nil, fmt.Errorf("failed to gather and assemble file data: %w", err)
	}
	return buffer, nil
	*/
	fileInfo, err := f.Stat()
	if err != nil {
		return nil, fmt.Errorf("failed to stat file: %w", err)
	}

	// ✅ File size in bytes
	fileSize := fileInfo.Size()
	buffer := make([]byte, fileSize)
	n, _ := f.Read(buffer)
	return buffer[:n], nil
}

// takes in a line of input, parses and returns the file name, size
func parseStore(line []byte) (string, int, error) {
	str := string(line)

	// Look for the filename part
	fileNamePrefix := "FileName:"
	fileNameIndex := strings.Index(str, fileNamePrefix)
	if fileNameIndex == -1 {
		return "", 0, fmt.Errorf("FileName field not found")
	}

	// Look for the size part
	sizePrefix := "size:"
	sizeIndex := strings.Index(str, sizePrefix)
	if sizeIndex == -1 {
		return "", 0, fmt.Errorf("size field not found")
	}

	// Extract the filename (between "FileName:" and "size:")
	fileNameStart := fileNameIndex + len(fileNamePrefix)
	fileName := strings.TrimSpace(str[fileNameStart:sizeIndex])

	// Extract the size (everything after "size:")
	sizeStart := sizeIndex + len(sizePrefix)
	sizeStr := strings.TrimSpace(str[sizeStart:])

	// Convert size to integer
	size, err := strconv.Atoi(sizeStr)
	if err != nil {
		return "", 0, fmt.Errorf("invalid size format: %v", err)
	}

	return fileName, size, nil
}
func grabFileName(line []byte) (string, error) {
	str := string(line)
	str = strings.TrimSpace(str)
	prefix := "FileName:"

	if len(str) <= len(prefix) || !strings.HasPrefix(str, prefix) {
		return "", fmt.Errorf("format of input isn't correct or fileName is missing")
	}

	return str[len(prefix):], nil
}

func fileExist(fileHashID string) bool {
	fileHashID = strings.TrimSpace(fileHashID)
	//	fmt.Println("meta data", fileMetaInfoMap)
	//	fmt.Printf("fileMetaInfoMap[%s] -> %+v\n", fileHashID, fileMetaInfoMap[fileHashID])

	_, ok := fileMetaInfoMap[fileHashID]
	return ok
}
func hashBytes(data []byte) string {
	hasher := fnv.New64a()
	hasher.Write(data)
	return fmt.Sprintf("%x", hasher.Sum64())
}

// For simplicity for now just have one storageMachine hold one portion of a file
// this means SM1 wont hold x_file_1 and x_file_2 it will hold at most 1 portion of a unique file
// this is just because id be more complicated to handle multiple files

// client request to store file
// they must do so in a very specefic way
// if size of file is too big return error and explain why operation couldnt be complete
// on the next line should be the start of the file contents
// only read in the len number of bytes the client specified at first (this is to prevent security issues that arises from reading an unknown number of bytes from a 3rd party)
// create a directy, write this file under that directory and give it a name
// then another struct will handle spliting up the file into X peices -> choose  n servers to split this between -> Server will directly handle writing this to storage machines
// return Good status to the client as well as the FileID they can use to retrive it

// client request previously sent file
// Check if the file ID exist
// check mapping for storageMachines with files
// find the storagMachiens that contain the file contents
// pass this to the server -> the server will handle requesting the indiviual storageMachines for their split of the data
// reassemble the split file contents in a temp directory
// return to client
// delete directory and temp file
