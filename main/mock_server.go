package main

import (
	"fmt"
	"log"
	"net/http"
	"time"

	"golang.org/x/net/websocket"
)

func handleMessage(conn *websocket.Conn) {
	var msg = make([]byte, 512)
	var n int
	var err error

	go func() {
		for {
			time.Sleep(2 * time.Second)
			conn.Write([]byte("Sending from server!"))
		}
	}()

	for {
		if n, err = conn.Read(msg); err != nil {
			log.Fatal(err)
		}
		fmt.Printf("Received: %s.\n", msg[:n])
	}

}

func main() {
	log.Printf("Listening on port 3001\n")
	http.Handle("/ws", websocket.Handler(handleMessage))
	err := http.ListenAndServe(":3001", nil)
	if err != nil {
		panic("ListenAndServe: " + err.Error())
	}
}
