package main

import (
	"log"
	"net/url"
	"os"

	"github.com/gorilla/websocket"
)

func main() {
	if len(os.Args) < 2 {
		log.Fatal("Usage: go run client.go <txn_id>")
	}

	txnID := os.Args[1]

	u := url.URL{Scheme: "ws", Host: "localhost:8080", Path: "/ws", RawQuery: "txn_id=" + txnID}
	log.Printf("Connecting to %s", u.String())

	// Connect to the WebSocket server
	c, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		log.Fatal("Dial error:", err)
	}
	defer c.Close()

	// Listen for messages
	for {
		_, message, err := c.ReadMessage()
		if err != nil {
			log.Println("Read error:", err)
			return
		}
		log.Printf("Received: %s", message)
	}
}
