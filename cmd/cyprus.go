package main

import (
	"log"

	"crosine.com/cyprus"
)

func main() {
	serv, servErr := cyprus.NewServerModule(0)
	if servErr != nil {
		log.Fatalf("Server erorr: %v", servErr)
	}
	serv.Run()
}
