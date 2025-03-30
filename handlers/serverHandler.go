package handlers

import (
	"bufio"
	"bytes"
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
	id               int
	connections_map  = &sync.Map{} // string:int
	totalConnections = make([]string, 1)
)

// Context key to avoid collision errors
type connReader struct{}

// load data structures from disk
func init() {
	s := storage.NewStorage(string(storage.Dir))
	conMap := make(map[string]int)
	conSlice := make([]string, 1)
	err := s.LoadFromDisk(storage.Connections, &conMap)
	fmt.Println(conMap)
	if err != nil {
		fmt.Println(err)
		panic(err)
	}
	err = s.LoadFromDisk(storage.TotalConnections, &conSlice)
	if err != nil {
		panic(err)
	}
	connections_map = storage.MaptoConcurentMap(conMap)
	totalConnections = conSlice
	fmt.Println(connections_map, totalConnections)
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

	reader := bufio.NewReader(c)
	// Peek the first line
	rawBytes, err := reader.ReadBytes('\n')
	if err != nil {
		fmt.Println("error reading from conn:", err)
		c.Close()
		return
	}
	defer c.Close()
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
	id := registerClient(conn)
	reader, ok := ctx.Value(connReader{}).(*bufio.Reader)
	if !ok {
		conn.Write([]byte("Server has made a fatal error, Missing Buffer reader from middleware"))
		return
	}
	rawBytes, err := reader.ReadBytes('\n')
	if err != nil {
		fmt.Println(err)
		return
	}
	logmsg := fmt.Sprintf("client %s wrote: %s\n", id, string(rawBytes))
	s.logChan <- logMsg{
		data: []byte(logmsg),
		name: id,
	}
	msg := fmt.Sprintf("Hello %v, you are now register in the file system \n", id)
	var b bytes.Buffer
	b.WriteString(msg)
	_, err = b.WriteTo(conn)
	if err != nil {
		fmt.Printf("err: %v\n", err)
	}
	//unregisterClient(conn)
	time.Sleep(time.Second * 1)

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
		time.Sleep(time.Second * 30)
	}
	fmt.Println("completed cron job ✔️")
}

// save all globals to memory
// Need to fix the error handling for this. mabey use an error array
func (s Server) UpdateDisk() []error {
	var res []error
	disk := storage.NewStorage("_diskStorage")
	toMap := storage.ConcurentMaptoMap[int, string](connections_map)
	err := disk.SaveToDisk(storage.Connections, &toMap)
	if err != nil {
		res = append(res, err)
	}
	err = disk.SaveToDisk(storage.TotalConnections, &totalConnections)
	if err != nil {
		res = append(res, err)
	}
	return res
}

func (s *Server) handleLogs() {
	f, err := os.OpenFile("fs_logs.txt", os.O_APPEND|os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		panic(err)
	}
	defer f.Close()
	for s.running {
		logMessage := <-s.logChan
		fmt.Printf("%s sent %d bytes to be written to disk\n", logMessage.name, len(logMessage.data))
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
func registerClient(c net.Conn) string {
	name := c.RemoteAddr().String()
	totalConnections = append(totalConnections, name)
	connections_map.Store(name, id)
	return name
}
func unregisterClient(c net.Conn) {
	name := c.RemoteAddr().String()
	connections_map.Delete(name)
}
