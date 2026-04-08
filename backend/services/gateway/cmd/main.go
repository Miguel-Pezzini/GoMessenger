package main

import (
	"log"

	gateway "github.com/Miguel-Pezzini/GoMessenger/services/gateway/internal"
)

func main() {
	if err := gateway.Run(); err != nil {
		log.Fatal(err)
	}
}
