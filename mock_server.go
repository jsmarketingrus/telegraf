package main

import (
	"flag"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
)

var addr = flag.String("addr", "localhost:8080", "http service address")

var upgrader = websocket.Upgrader{} // use default options

func echo(w http.ResponseWriter, r *http.Request) {
	c, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Print("upgrade error: ", err)
		return
	}
	defer c.Close()
	for {
		time.Sleep(2 * time.Second)
		var m = map[string]interface{}{
			"Operation": "GET_PLUGIN",
			"Uuid":      "123",
			"Plugin": map[string]string{
				"Name": "example plugin",
			},
		}
		err = c.WriteJSON(m)
		if err != nil {
			log.Println("write:", err)
			break
		}
		_, message, err := c.ReadMessage()
		if err != nil {
			log.Println("read:", err)
			break
		}
		log.Printf("recv: %s", message)
	}
}

func main() {
	flag.Parse()
	log.SetFlags(0)
	http.HandleFunc("/echo", echo)
	log.Println("listening on localhost:8080/echo...")
	log.Fatal(http.ListenAndServe(*addr, nil))
}
