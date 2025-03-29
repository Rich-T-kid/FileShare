package main

import (
	"bufio"
	"bytes"
	"fmt"
	"log"
	"net"
	"os"
	"sync"
	"time"

	"github.com/joho/godotenv"

	"FileShare/storage"
)

/*
Requirments
a collection of independent machines should be able to register and hold portions of files
this means if there are N machines registered under a server each files uploaded to the server should be split into n and each machine will hop 1/n'th  of the files data
The ordering of the data as well as what servers hold what is up to the server to manage .
when asking the server for the file it should reassemble the file from the collection of machines and return the file to the client

Steps/Plan as of now
(1) allow machines to register their ips with the server (Done)
	(1.1) this should be written to disk somehow for inbewtween server starts (Done)
	(1.2) mapping of Ip addresses and names would also be usful, for when we split the file data amoung servers
(2) server should be able to take in a file and split it up amoungst x machines. (this Will yeild a fileID that can be used to find the file later)
	(2.1) this split count doesnt need to rely on the number of machines we  have to store the file splits. at most it can be that but it does not need to be -> bewtween 1 , len(machines)
	(2.2) their should be  a mapping of what file resides on what machine. furthuremore there should be a mapping of which portions of files lie on which machines -> HashMap[fileID:HashMap[servers:portionPiece]]
	(2.3) the algorithm that we use to pick which servers store what files as well as how we will split the files isnt super important right now. When implementing keep loose and wrap in an interface
(3) Clients can retrive their files in perfect condition after passing in their fileID
	(3.1) the returned file should be exactly the same as it was when the client gave the server the file
	(3.2) grab all the piece of the file from the file->machineIP mapping in Order (Dont need to be in order actually just need to be assembled back in order)
	(3.3) return this back to the client
	(3.4) Not sure if we should remove the file splitings at this point? This can be worked on later not important now
(4) Server should be able to be turned on and off Gracefully
	(4.1) all important state should regulary be written to disk. No information should be lost if the server is randomly shut up
	(4.2)Includes file -> machine mappings , machine-IP's -> file split mapping, and any other usful data

*/

// TODO: Should just be in global scope but for now its fine
var (
	logChan          = make(chan logMsg, 2)
	connections_map  = sync.Map{}
	totalConnections = make([]string, 1)
	id               int
	txt              = "The code above uses the os' package Open function to open the file, defers its Close function with the defer keyword, defines an empty lines slice, and uses the bufio's NewScanner function to read the file line-by-line while appending each line to the lines array in a text format using the Text function. Finally, it uses the printLastNLines function to get the last N lines of the lines array. N is any number of the user's choosing. In this case, it is 3, and the code uses a for loop to print each line with an horizontal line between each one."
)

type logMsg struct {
	data []byte
	name string
}

func registerClient(c net.Conn) string {
	name := c.RemoteAddr().String()
	totalConnections = append(totalConnections, name)
	connections_map.Store(id, name)
	return name
}
func unregisterClient(c net.Conn) {
	name := c.RemoteAddr().String()
	connections_map.Delete(name)
}
func checkCancle(b []byte, shutdown chan<- string, id string) bool {
	var cancleMessage = os.Getenv("SERVER_SHUTDOWN_KEY")
	var size int = len(cancleMessage)
	if len(b) >= size && string(b[:size]) == cancleMessage {
		shutdown <- fmt.Sprintf("%s has instructed to shutdown the server", id)
		return true
	}
	return false
}

// Here we need to just register the conenction first
func handleConnection(conn net.Conn, shutdown chan<- string) {
	id := registerClient(conn)
	reader := bufio.NewReader(conn)
	rawBytes, err := reader.ReadBytes('\n')
	if err != nil {
		fmt.Println(err)
		return
	}
	if checkCancle(rawBytes, shutdown, id) {
		conn.Write([]byte("recieved shutdown message! shutting down server\n"))
		return
	}
	logmsg := fmt.Sprintf("client %s wrote: %s\n", id, string(rawBytes))
	logChan <- logMsg{
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
	unregisterClient(conn)
	time.Sleep(time.Second * 1)

}
func MiddleWare(c net.Conn, shutdown chan<- string) {
	// read from first X bytes of connection. if its the shutdwon and shut the server down
	start := time.Now()
	handleConnection(c, shutdown)
	fmt.Printf("connection took %v\n", time.Since(start))
	c.Close()

}

// This should also take in a flag. for termination reasons
func handleLogs(flag *bool) {
	f, err := os.OpenFile("fs_logs.txt", os.O_APPEND|os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		panic(err)
	}
	defer f.Close()
	for *flag {
		logMessage := <-logChan
		fmt.Printf("%s sent %d bytes to be written to disk\n", logMessage.name, len(logMessage.data))
		f.Write(logMessage.data)
	}
	fmt.Println("Finished writing logs to fs_logs.txt")
}

// This should manage loading things to disk consistantly
// Periodically save stuff to disk.
func (s *Server) CronJobs(wg *sync.WaitGroup) {
	defer wg.Done()
	go handleLogs(&s.running)
	fmt.Println("starting cron jobs")
	//var interval time.Duration = time.Second * 20
	for s.running {
		fmt.Println("<-->")
		time.Sleep(time.Second * 3)
	}
	fmt.Println("finished cron jobs")
}

// save all globals to memory
// Need to fix the error handling for this. mabey use an error array
func (s Server) cleanUp() []error {
	var res []error
	disk := storage.NewStorage("_diskStorage")
	toMap := storage.ConcurentMaptoMap[int, string](&connections_map)
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

type Server struct {
	listener net.Listener
	shutdown chan string
	running  bool
}

func newServer(connectionStr string) *Server {
	l, err := net.Listen("tcp", connectionStr)
	if err != nil {
		log.Fatal(err)
	}
	return &Server{
		listener: l,
		shutdown: make(chan string),
		running:  true,
	}
}
func init() {
	err := godotenv.Load()
	if err != nil {
		panic(err)
	}
	fmt.Println(os.Getenv("PORT"))
	fmt.Println(os.Getenv("SERVER_SHUTDOWN_KEY"))
}
func main() {
	// TODO: Move connection string as well as the shutdown string to an env varible
	//realistically this would be moved to an env variable
	var connectionStr = fmt.Sprintf("0.0.0.0:%s", os.Getenv("PORT"))
	fileServer := newServer(connectionStr)
	fmt.Printf("Started tcp Listener on %s\n", connectionStr)
	fileServer.Start()
	fmt.Println("Server has gracfully shut down")

}

// slight bug. This requires two messsage for a shut down to be succsefull (not gonna fix we can work with it)
func (s *Server) handleListener(wg *sync.WaitGroup) {
	defer wg.Done()
	start := time.Now()
	for s.running {
		conn, err := s.listener.Accept()
		if err != nil {
			fmt.Println(err)
			continue
		}
		id++
		go MiddleWare(conn, s.shutdown)

	}
	fmt.Printf("\nserver ran for %v \n", time.Since(start))
}
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
	//time.Sleep(time.Second * 1) // allow time for goroutines to finish cleaning up
	errs := s.cleanUp()
	if len(errs) == 0 {
		fmt.Println("No errors from storings data to disk")
		return
	}
	for i := 0; i < len(errs); i++ {
		fmt.Printf("Error %d from clean up %v\n", i, errs[i])
	}

}

// main -> go handleConnections -> Middleman -> HandleConnections
// middleMan will handle any panics that may occure from goroutines. This is where ill handle the graceful shutdonw of the TCP Server
// Do not use panics. just copy the first x bytes to a copy slice.it it maches the shutdown message send a msg on the channel to shutdown the server
