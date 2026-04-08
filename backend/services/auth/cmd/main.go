package main

import (
	"log"

	auth "github.com/Miguel-Pezzini/GoMessenger/services/auth/internal"
)

func main() {
	if err := auth.Run(); err != nil {
		log.Fatal(err)
	}
}
