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
	ErrContentAlreadyExist = errors.New("Content already exist in this distributed file Server. No addition data was stored\n")
	contentSet             = make(map[string]bool)
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
	case "store":
		fmt.Println("line: ->", string(line))
		fName, size, err := parseStore(line)
		if err != nil {
			conn.Write([]byte(invalidRequest(string(line))))
			writelog([]byte(err.Error()), "client store command failed")
			return
		}
		// in the future client should have to worry about this
		exist := fileExist(fName)
		if exist {
			conn.Write([]byte(fmt.Sprintf("Cannot use file of name %s, its already taken \n", fName)))
			return
		}
		// at most only read in 32 MB's
		buffer := make([]byte, min(size, megaByte*32))
		n, err := reader.Read(buffer)
		if err != nil {
			writelog([]byte(err.Error()), "client store command failed")
			return
		}
		err = storeFile(ctx, conn, fName, buffer[:n])
		if err != nil {
			if errors.Is(err, ErrContentAlreadyExist) {
				fmt.Printf("Recieved a duplicate store request, %v", err)
				conn.Write([]byte("The content you are trying to store has already been stored on this distributed file server"))
				return
			}
			writelog([]byte(fmt.Sprintf("error attempting to store %s to disk, error:%v", fName, err.Error())), "SERVER ERROR")
			fmt.Println(err)
			return
		}
		conn.Write([]byte(fmt.Sprintf("\nSuccsucfully wrote %d bytes to distributed file sever\n", n)))

	case "recieve":
		name, err := grabFileName(line)
		if err != nil {
			conn.Write([]byte(err.Error()))
		}
		exist := fileExist(name)
		if !exist {
			conn.Write([]byte("Cannot retrive a file that doesnt already exist. Please recheck file name spelling\n"))
			return
		}
		content, err := retriveFile(ctx, conn, name)
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
type FileMeta struct {
	// Machine Ip address mapped to the file chunk number
	// EX: 127.0.0.1:2 -> ip of 127.0.0.1 holds the second chunk of the file info
	// len(machineMapping) = number of pieces, so this is how you can get an upper bound for the number of splits
	MachienMapping map[IPADDR]int
	// Orginal file name that was passed in by external client
	OriginalFileName string
	//Generated file uuid
	HashFileName string
	// Last time the file was stored or retrieved
	LastEdited time.Time
}

// this should also check if the file contents already exist
// IE doing a hash
func storeFile(ctx context.Context, conn net.Conn, fileName string, content []byte) error {
	path := fmt.Sprintf("%s/%s", downloadDir, fileName)
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return err
	}
	_, err = f.Write(content)
	fileLocalLocation[fileName] = path
	// New stuff below
	// This is where wed generate a fileID Hash here
	fileHashID := hashBytes(content)
	if contentSet[fileHashID] {
		fmt.Println("Returning early since hash ID already exist")
		// theres no need to continue of the file hash already be stored previously and exist within our distributed file server
		return ErrContentAlreadyExist

	}
	machineMap, errors := splitToStorage(ctx, totalConnections, content)
	if len(errors) != 0 {
		return errors[0]
	}
	// Need to also Store metaData like , fileName, fileSize, fileHashID,ect hold this in a struct and these structs should hold the associated  mapping table, but the file UUID should be mapped to this struct
	fInfo := &FileMeta{
		MachienMapping:   machineMap,
		OriginalFileName: fileName,
		HashFileName:     fileHashID,
		LastEdited:       time.Now().UTC(),
	}
	fileMetaInfoMap[fileHashID] = fInfo
	fmt.Println(fileHashID, fInfo, "Now Stored together on disk")
	contentSet[fileHashID] = true
	fileLocalLocation[fileName] = path
	return err
}
func retriveFile(ctx context.Context, conn net.Conn, fileName string) ([]byte, error) {
	path := fmt.Sprintf("%s/%s", downloadDir, fileName)
	f, err := os.OpenFile(path, os.O_RDONLY, 0644)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("%s does not exist in the %s directory", fileName, downloadDir)
		} else {
			return nil, err
		}
	}
	fileInfo, err := f.Stat()
	if err != nil {
		return nil, fmt.Errorf("failed to stat file: %w", err)
	}

	// ✅ File size in bytes
	fileSize := fileInfo.Size()
	fmt.Printf("File size: %d bytes\n", fileSize)
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

func fileExist(fname string) bool {
	_, ok := fileLocalLocation[fname]
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
