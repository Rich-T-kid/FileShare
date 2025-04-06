package handlers

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"strings"
	"sync"
	"time"

	"FileShare/storage"
)

var (
	id                int
	connections_map   = &sync.Map{} // string:int
	totalConnections  = make([]string, 1)
	fileLocalLocation = make(map[string]string) // fileName : Path
	// Maps the generated file ID (that gets returned to external client) to usefull metadata about file
	fileMetaInfoMap = make(map[string]*FileMeta)
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
	metaDataMap := make(map[string]*FileMeta)
	conSlice := make([]string, 1)
	err := s.LoadFromDisk(storage.ConnectionsPairs, &conMap)
	if err != nil {
		panic(err)
	}
	err = s.LoadFromDisk(storage.FileMetaData, &metaDataMap)
	if err != nil {
		panic(err)
	}
	err = s.LoadFromDisk(storage.TotalConnections, &conSlice)
	if err != nil {
		panic(err)
	}
	err = s.LoadFromDisk(storage.FileLocations, &fileLocalLocation)
	if err != nil {
		fmt.Println(err)
		panic(err)
	}
	connections_map = storage.MaptoConcurentMap(conMap)
	totalConnections = conSlice
	fmt.Println(connections_map, totalConnections, fileLocalLocation)
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
	err = disk.SaveToDisk(storage.FileLocations, &fileLocalLocation)
	if err != nil {
		res = append(res, err)
	}
	err = disk.SaveToDisk(storage.FileLocations, &fileMetaInfoMap)
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
type IPADDR = string

func splitToStorage(ctx context.Context, machines []string, data []byte) (map[IPADDR]int, []error) {
	// somehow validate the machines
	// choose n machines to split data amoungst. split the data into n evenly sized pieces
	// write data/n to n machines, if any errors occure, continue and return a slice of errors to caller
	// return the map of dataSplit:MachineIP
	var e []error
	m := make(map[IPADDR]int)
	m["129.0.0.1"] = 210
	return m, e
}

func recieveSplitStorage(ctx context.Context, fileInfo FileMeta, dest []byte) error { return nil }

// split source by n Times, each split is as even as possible and is atmost off by 1
func splitContent(src []byte, n uint8) [][]byte {
	return nil
}
