package main

import (
	"log"

	notification "github.com/Miguel-Pezzini/GoMessenger/services/notification/internal"
)

func main() {
	if err := notification.Run(); err != nil {
		log.Fatal(err)
	}
}
