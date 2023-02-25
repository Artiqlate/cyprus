package main

import (
	"log"

	"crosine.com/cyprus"
)

func main() {
	serv, servErr := cyprus.NewServerModule()

	if servErr != nil {
		log.Fatalf("Server erorr: %v", servErr)
	}

	serv.Run()
}
