package main

import (
	"flag"
	"log"

	"crosine.com/cyprus"
)

func main() {
	isSecure := flag.Bool("secure", true, "Use Secure Server")
	flag.Parse()
	serv, servErr := cyprus.NewServerModule(0, *isSecure)
	if servErr != nil {
		log.Fatalf("Server erorr: %v", servErr)
	}
	serv.Run()
}
