package main

import (
	"log"
	"net/http"
)

func main() {
	startBroadcasts()

	http.HandleFunc("/ws", WsHandler)
	log.Fatal(http.ListenAndServe(":8080", nil))
}
