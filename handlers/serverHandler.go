package handlers

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"strings"
	"sync"
	"time"

	"FileShare/storage"
)

var (
	id               int
	connections_map  = &sync.Map{}       // string:int -> Ip:ID.json
	totalConnections = make([]string, 1) // -> Array_connections.json
	// Maps the generated file ID (that gets returned to external client) to usefull metadata about file
	fileMetaInfoMap = make(map[string]*FileMeta) // FileID:filemeta.json
)

const (
	megaByte = 1024 * 1024 // 32 MB
	kilobyte = 1024        // 1k
)

// Context key to avoid collision errors
type connReader struct{}

// load data structures from disk
func init() {
	s := storage.NewStorage(string(storage.Dir))
	conMap := make(map[string]int)
	conSlice := make([]string, 1)
	err := s.LoadFromDisk(storage.ConnectionsPairs, &conMap)
	if err != nil {
		panic(err)
	}
	err = s.LoadFromDisk(storage.FileMetaData, &fileMetaInfoMap)
	if err != nil {
		panic(err)
	}
	err = s.LoadFromDisk(storage.TotalConnections, &conSlice)
	if err != nil {
		panic(err)
	}
	connections_map = storage.MaptoConcurentMap(conMap)
	totalConnections = conSlice

}

type logMsg struct {
	data []byte
	name string
}

type Server struct {
	listener        net.Listener
	shutdown        chan string
	logChan         chan logMsg
	_shutDownString string
	running         bool
}

// Ignores the Shut down string field so we never expose the shutdown key
func (s *Server) String() string {
	return fmt.Sprintf("Server{listener: %v, running: %v}", s.listener.Addr(), s.running)
}

// slight bug. This requires two messsage for a shut down to be succsefull (not gonna fix we can work with it)
func (s *Server) Start() {
	wg := &sync.WaitGroup{}
	go s.handleListener(wg)
	go s.CronJobs(wg)
	wg.Add(2)
	for {
		msg := <-s.shutdown
		fmt.Printf("%s\nRecieved shutddown signal at %v\n", msg, time.Now().Format("Monday, 02-Jan-06 15:04:05 MST"))
		s.running = false
		break
	}
	wg.Wait()
	errs := s.UpdateDisk()
	if len(errs) == 0 {
		fmt.Println("No errors from storings data to disk")
		return
	}
	for i := 0; i < len(errs); i++ {
		fmt.Printf("Error %d from clean up %v\n", i, errs[i])
	}

}

func (s *Server) handleListener(wg *sync.WaitGroup) {
	defer wg.Done()
	for s.running {
		conn, err := s.listener.Accept()
		if err != nil {
			fmt.Println(err)
			continue
		}
		id++
		go s.MiddleWare(conn, s.shutdown)

	}
}

// Implementations should not close the connection. That what the servers middleware will do
func (s *Server) MiddleWare(c net.Conn, shutdown chan<- string) {
	defer logDuration(time.Now(), "HandleConnection")

	defer c.Close()
	reader := bufio.NewReader(c)
	// Peek the first line
	rawBytes, err := reader.ReadBytes('\n')
	fmt.Println("first byes -> ", string(rawBytes))
	if err != nil {
		fmt.Println("error reading from conn:", err)
		return
	}
	defer fmt.Println("Finished handling connection")
	// trims \n  | store:\n -> store:
	prefix := strings.TrimSpace(string(rawBytes))
	handler, exit := s.HandleConn(prefix)
	if exit {
		s.shutdown <- "Server has recieved shutdown message"
		c.Write([]byte("Acknowledged\n"))
		return
	}

	ctx := context.Background()
	ctx = context.WithValue(ctx, connReader{}, reader)
	// If it's a custom handler (Client/User), use it
	if handler != s {
		handler.HandleConnection(ctx, c)
		return
	}
	// Otherwise, the server handles it directly
	s.HandleConnection(ctx, c)
}
func (s *Server) HandleConnection(ctx context.Context, conn net.Conn) {
	reader, ok := ctx.Value(connReader{}).(*bufio.Reader)
	if !ok {
		conn.Write([]byte(ERRinternalServer("Missing Buffer reader from middleware").Error()))
		return
	}
	// for now this is just an echo
	rawBytes, err := reader.ReadBytes('\n')
	if err != nil {
		fmt.Println(err)
		return
	}
	conn.Write(rawBytes)

}

// This should manage loading things to disk consistantly
// Periodically save stuff to disk.
func (s *Server) CronJobs(wg *sync.WaitGroup) {
	defer wg.Done()
	go s.handleLogs()
	fmt.Println("started cron jobs ✔️")
	//var interval time.Duration = time.Second * 20
	for s.running {
		errs := s.UpdateDisk()
		if len(errs) > 0 {
			for i := 0; i < len(errs); i++ {
				fmt.Printf("errors updating persistance storage issues:%d   %v", i, errs[i])
			}
		}
		time.Sleep(time.Second * 10)
	}
	fmt.Println("completed cron job ✔️")
}

// save all globals to memory
// Need to fix the error handling for this. mabey use an error array
func (s Server) UpdateDisk() []error {
	var res []error
	disk := storage.NewStorage("_diskStorage")
	toMap := storage.ConcurentMaptoMap[string, int](connections_map)
	err := disk.SaveToDisk(storage.ConnectionsPairs, &toMap)
	if err != nil {
		res = append(res, err)
	}
	err = disk.SaveToDisk(storage.TotalConnections, &totalConnections)
	if err != nil {
		res = append(res, err)
	}
	err = disk.SaveToDisk(storage.FileMetaData, &fileMetaInfoMap)
	if err != nil {
		res = append(res, err)
	}
	return res
}

func (s *Server) handleLogs() {
	f, err := os.OpenFile("FileShare_logs.txt", os.O_APPEND|os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		panic(err)
	}
	defer f.Close()
	defer close(s.logChan)
	// allow client and storage handlers to use the same logger
	// this is only for simplicity. if needed they can use their own logger later
	go setexternalLog(s.logChan)
	for s.running {
		logMessage := <-s.logChan
		fmt.Printf("%s sent %d bytes to be written to disk\n", logMessage.name, len(logMessage.data))
		logMessage.data = append(logMessage.data, []byte("\n")...)
		f.Write(logMessage.data)
	}
	fmt.Println("Finished writing logs to fs_logs.txt")
}
func logDuration(start time.Time, label string) {
	fmt.Printf("%s took %v\n", label, time.Since(start))
}

func NewServer(connectionStr string) *Server {
	l, err := net.Listen("tcp", connectionStr)
	if err != nil {
		log.Fatal(err)
	}
	return &Server{
		listener:        l,
		shutdown:        make(chan string),
		logChan:         make(chan logMsg, 2),
		running:         true,
		_shutDownString: os.Getenv("SERVER_SHUTDOWN_KEY"),
	}
}

// machines: array of possible machines that can store the data, use the connections.json list
// data: byte slice that can be any data to be written over wire
type IPADDR string

// somehow validate the machines
// choose n machines to split data amoungst. split the data into n evenly sized pieces
// write data/n to n machines, if any errors occure, continue and return a slice of errors to caller
// return the map of dataSplit:MachineIP
func splitToStorage(ctx context.Context, machines []string, data []byte) (map[IPADDR]int, []error) {
	var e []error
	var machineSplitMap = make(map[IPADDR]int)
	machines = validMachines(machines)
	if len(machines) == 0 {
		e = append(e, fmt.Errorf("no valid machines to store data"))
		return machineSplitMap, e
	}
	dataPartition := splitContent(data, uint8(len(machines)))
	fileID := hashBytes(data)
	for i := 0; i < len(dataPartition); i++ {
		go transferFile(ctx, machines[i], dataPartition[i], fileID)
		machineSplitMap[IPADDR(machines[i])] = i
	}
	return machineSplitMap, e
}

// split source by n Times, each split is as even as possible and is atmost off by 1
func splitContent(src []byte, n uint8) [][]byte {
	// Calculate the size of each split
	splitSize := len(src) / int(n)
	// Calculate the remaining bytes to distribute
	remaining := len(src) % int(n)

	var result [][]byte
	start := 0

	// Split the content into n parts
	for i := 0; i < int(n); i++ {
		// Calculate the end index for each part
		end := start + splitSize
		if remaining > 0 {
			end++ // Distribute the remaining bytes
			remaining--
		}

		// Append the split part
		result = append(result, src[start:end])

		// Move the start index for the next split
		start = end
	}

	return result
}

// In perfect world this check all the passed in machines for a ping to make sure thier running
func validMachines(machines []string) []string {
	var validMachines []string
	for _, machineIP := range machines {
		// Change this to a ping check
		if machineIP == "" {
			continue
		}
		if pingMachine(machineIP) {
			fmt.Printf("Machine %s is valid\n", machineIP)
			validMachines = append(validMachines, machineIP)

		}
		//validMachines = append(validMachines, machineIP)
	}

	/*
		var result []string
		for _, machine := range machines {
			if pingMachine(machine) {
				result = append(result, machine)
			}
		}
	*/
	return validMachines
}
func pingMachine(ipAddr string) bool {
	endpoint := ipAddr + ":9998"
	conn, err := net.Dial("tcp", endpoint)
	if err != nil {
		return false
	}
	defer conn.Close()
	conn.Write([]byte("ping"))
	buf := make([]byte, 1024)
	n, err := conn.Read(buf)
	if err != nil {
		return false
	}
	if n == 0 {
		return false
	}
	return strings.TrimSpace(string(buf[:n])) == "pong"
	// Simulate a ping to the machine
	// In a real implementation, you would use a network library to check the machine's availability
	// For example, you could use net.Dial to check if the machine is reachable
}

func transferFile(ctx context.Context, storageIPAddr string, fileBytes []byte, fileID string) {
	const port = ":9997"
	_, canclefunc := context.WithTimeout(ctx, time.Second*5)
	defer canclefunc()
	conn, err := net.Dial("tcp", net.JoinHostPort(storageIPAddr, "9997"))
	if err != nil {
		fmt.Printf("Error connecting to %s: %v\n", storageIPAddr, err)
		return
	}
	defer conn.Close()
	conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	// Send the file bytes to the storage machine
	conn.Write([]byte(fileID + "\n")) // Send file ID with newline
	_, err = conn.Write(fileBytes)    // Send raw bytes
	if err != nil {
		fmt.Printf("Error sending data to %s: %v\n", storageIPAddr, err)
		return
	}
	// Read the response from the storage machine
	response := make([]byte, 1024)
	_, err = conn.Read(response)
	if err == io.EOF {
		fmt.Printf("Finished Connection with %s\n", storageIPAddr)
		return
	}
	if err != nil {
		fmt.Printf("Error reading response from %s: %v\n", storageIPAddr, err)
		return
	}
}

// LOL this is going to be hell to implement

// should handle all of the assembling of the split data as well. Write directly into the provided buffer
// if for what ever reason if the assembled data isnt the same size as the original data, return an error
func recieveSplitStorage(ctx context.Context, fileInfo FileMeta, dest []byte) error {
	if len(fileInfo.MachienMapping) == 0 {
		return fmt.Errorf("no machine mapping found")
	}

	// Step 1: Invert the map to get chunkIndex -> IP
	indexToIP := make(map[int]string)
	for ip, index := range fileInfo.MachienMapping {
		indexToIP[index] = string(ip)
	}

	// Step 2: Prepare a slice to hold the chunks
	chunkBuffers := make([][]byte, len(indexToIP))

	// Step 3: Request each chunk by its index
	for idx := 0; idx < len(indexToIP); idx++ {
		ip, ok := indexToIP[idx]
		if !ok {
			return fmt.Errorf("missing chunk %d in metadata", idx)
		}

		address := net.JoinHostPort(ip, "9996")
		conn, err := net.DialTimeout("tcp", address, 5*time.Second)
		if err != nil {
			return fmt.Errorf("failed to connect to %s: %w", address, err)
		}

		// Send request in format: "<fileID>:<chunkIndex>\n"
		request := fmt.Sprintf("%s:%d\n", fileInfo.HashFileName, idx)
		_, err = conn.Write([]byte(request))
		if err != nil {
			conn.Close()
			return fmt.Errorf("failed to send chunk request: %w", err)
		}

		// Read chunk data
		data, err := io.ReadAll(conn)
		conn.Close()
		if err != nil {
			return fmt.Errorf("error reading chunk %d from %s: %w", idx, ip, err)
		}

		chunkBuffers[idx] = data
	}

	// Step 4: Reassemble the chunks into dest
	offset := 0
	for _, chunk := range chunkBuffers {
		copy(dest[offset:], chunk)
		offset += len(chunk)
	}

	if offset != fileInfo.TotSize {
		return fmt.Errorf("expected assembled size %d, but got %d", fileInfo.TotSize, offset)
	}

	return nil
}
