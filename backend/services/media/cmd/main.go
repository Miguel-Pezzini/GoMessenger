package main

import (
	"log"

	media "github.com/Miguel-Pezzini/GoMessenger/services/media/internal"
)

func main() {
	if err := media.Run(); err != nil {
		log.Fatal(err)
	}
}
