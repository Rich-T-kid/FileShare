package main

import (
	"fmt"
	"os"
	"time"

	"FileShare/handlers"
)

/*
Requirments
a collection of independent machines should be able to register and hold portions of files
this means if there are N machines registered under a server each files uploaded to the server should be split into n and each machine will hop 1/n'th  of the files data
The ordering of the data as well as what servers hold what is up to the server to manage .
when asking the server for the file it should reassemble the file from the collection of machines and return the file to the client

Steps/Plan as of now
(1) allow machines to register their ips with the server (DONE)
	(1.1) this should be written to disk somehow for inbewtween server starts (DONE)
	(1.2) mapping of Ip addresses and names would also be usful, for when we split the file data amoung servers (DONE)
(2) server should be able to take in a file and split it up amoungst x machines. (this Will yeild a fileID that can be used to find the file later)
	(2.1) this split count doesnt need to rely on the number of machines we  have to store the file splits. at most it can be that but it does not need to be -> bewtween 1 , len(machines)
	(2.2) their should be  a mapping of what file resides on what machine. furthuremore there should be a mapping of which portions of files lie on which machines -> HashMap[fileID:HashMap[servers:portionPiece]]
	(2.3) the algorithm that we use to pick which servers store what files as well as how we will split the files isnt super important right now. When implementing keep loose and wrap in an interface
(3) Clients can retrive their files in perfect condition after passing in their fileID
	(3.1) the returned file should be exactly the same as it was when the client gave the server the file
	(3.2) grab all the piece of the file from the file->machineIP mapping in Order (Dont need to be in order actually just need to be assembled back in order)
	(3.3) return this back to the client
	(3.4) Not sure if we should remove the file splitings at this point? This can be worked on later not important now
(4) Server should be able to be turned on and off Gracefully (DONE)
	(4.1) all important state should regulary be written to disk. No information should be lost if the server is randomly shut up (DONE)
	(4.2)Includes file -> machine mappings , machine-IP's -> file split mapping, and any other usful data (DONE)

(5)	There needs to be a limited Number of commands that can be ran between the server and client
	(5.1) this reduces the need for RegEx and string parsing if we can just match to X number of cases
	(5.2)off the top of my head this includes:
	All of this should be done using json for the easist encoding and decoding
	All non file saving requirments must be prefixed with C: -> C(lient):   this is to differentiant from a client request to save a large file
	(client,server) client -> server Response
	(request to become register for file sharing,Acknologment)
	(request polling to keep tcp connection,Acknologment)

	(server,client) server -> client response
	(prepare to recive portion of file, accept, then generate file to store this file|decline) -> ($FileID_portion $FileBytes,Acknolgment)
	(HeartBeat $count,Acknolgment $count+1)
	(Contains $FileID_Portion,True | False) -> if true server creates file and prepares to recieve bytes from client, then server Acknologies


	THIS IS UNIQUIE. ANYONE CAN SEND A SHUT DOWN MESSAGE SO THE SERVER HANDLERS THESE DIRECTLY
	($shutdown_string,Acknologment)

*/

func main() {
	defer logDuration(time.Now(), "TCP Server")
	var connectionStr = fmt.Sprintf("0.0.0.0:%s", os.Getenv("PORT"))
	fileServer := handlers.NewServer(connectionStr)
	fmt.Printf("Started tcp Listener on %s\n", connectionStr)
	fileServer.Start()

}

func logDuration(start time.Time, label string) {
	fmt.Printf("%s took %v -- Server has gracfully shut down\n ", label, time.Since(start))
}
