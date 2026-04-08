package main

import (
	"log"

	logging "github.com/Miguel-Pezzini/GoMessenger/services/logging/internal"
)

func main() {
	if err := logging.Run(); err != nil {
		log.Fatal(err)
	}
}
