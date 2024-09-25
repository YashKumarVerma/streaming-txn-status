package main

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"

	"github.com/gorilla/websocket"
	"github.com/lib/pq"
	_ "github.com/lib/pq"
)

var (
	clients      = make(map[*Client]bool)
	clientsMutex sync.Mutex
	register     = make(chan *Client)
	unregister   = make(chan *Client)
	notify       = make(chan Notification)
)

type Client struct {
	conn  *websocket.Conn
	txnID int
	send  chan []byte
}

type Notification struct {
	TxnID  int
	Status string
}

func main() {
	// Start the hub
	go hub()

	// Start listening to Postgres notifications
	go listenToPostgres()

	// Set up WebSocket endpoint
	http.HandleFunc("/ws", wsHandler)

	// Start the server
	log.Println("Server started on :8080")
	err := http.ListenAndServe(":8080", nil)
	if err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
}

func wsHandler(w http.ResponseWriter, r *http.Request) {
	upgrader := websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return true // Allow all origins
		},
	}

	// Upgrade initial GET request to a WebSocket
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("Upgrade error:", err)
		return
	}

	// Read the transaction ID from query params
	txnIDStr := r.URL.Query().Get("txn_id")
	if txnIDStr == "" {
		conn.WriteMessage(websocket.TextMessage, []byte("Missing txn_id parameter"))
		conn.Close()
		return
	}

	txnID, err := strconv.Atoi(txnIDStr)
	if err != nil {
		conn.WriteMessage(websocket.TextMessage, []byte("Invalid txn_id parameter"))
		conn.Close()
		return
	}

	client := &Client{
		conn:  conn,
		txnID: txnID,
		send:  make(chan []byte),
	}

	// Register the client
	register <- client

	// Start goroutines for reading and writing
	go client.readPump()
	go client.writePump()
}

func hub() {
	for {
		select {
		case client := <-register:
			clientsMutex.Lock()
			clients[client] = true
			clientsMutex.Unlock()
			log.Printf("Client registered for txn_id %d", client.txnID)
		case client := <-unregister:
			clientsMutex.Lock()
			if _, ok := clients[client]; ok {
				delete(clients, client)
				close(client.send)
				log.Printf("Client unregistered for txn_id %d", client.txnID)
			}
			clientsMutex.Unlock()
		case notification := <-notify:
			// Send the notification to clients interested in this txn_id
			clientsMutex.Lock()
			for client := range clients {
				if client.txnID == notification.TxnID {
					message := fmt.Sprintf("Transaction %d status changed to %s", notification.TxnID, notification.Status)
					client.send <- []byte(message)
				}
			}
			clientsMutex.Unlock()
		}
	}
}

func (c *Client) readPump() {
	defer func() {
		unregister <- c
		c.conn.Close()
	}()
	for {
		_, _, err := c.conn.ReadMessage()
		if err != nil {
			log.Println("Read error:", err)
			break
		}
	}
}

func (c *Client) writePump() {
	defer c.conn.Close()
	for {
		select {
		case message, ok := <-c.send:
			if !ok {
				// Channel closed
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			err := c.conn.WriteMessage(websocket.TextMessage, message)
			if err != nil {
				log.Println("Write error:", err)
				return
			}
		}
	}
}

func listenToPostgres() {
	connStr := "user=postgres dbname=test_bench sslmode=disable"
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		log.Fatal("Error connecting to database:", err)
	}
	defer db.Close()

	// Establish a connection for LISTEN/NOTIFY
	listener := pq.NewListener(connStr, 10, 60, reportProblem)
	err = listener.Listen("transaction_status_changed")
	if err != nil {
		log.Fatal("Error listening to channel:", err)
	}
	log.Println("Listening for notifications on channel 'transaction_status_changed'")

	for {
		select {
		case n := <-listener.Notify:
			if n == nil {
				continue
			}
			// Parse the payload
			parts := strings.SplitN(n.Extra, ",", 2)
			if len(parts) != 2 {
				log.Println("Invalid notification payload:", n.Extra)
				continue
			}
			txnID, err := strconv.Atoi(parts[0])
			if err != nil {
				log.Println("Invalid txn_id in payload:", parts[0])
				continue
			}
			status := parts[1]
			notification := Notification{
				TxnID:  txnID,
				Status: status,
			}
			// Send the notification to the hub
			notify <- notification
		}
	}
}

func reportProblem(ev pq.ListenerEventType, err error) {
	if err != nil {
		log.Println(err.Error())
	}
}
