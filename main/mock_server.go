package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/google/uuid"
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
	reader := bufio.NewReader(os.Stdin)
	defer c.Close()
	for {
		plugin, _ := reader.ReadString('\n')
		pluginType, _ := reader.ReadString('\n')
		plugin = strings.Replace(plugin, "\n", "", -1)
		pluginType = strings.Replace(pluginType, "\n", "", -1)

		uid, _ := uuid.NewRandom()
		var m = map[string]interface{}{
			"Operation": "GET_PLUGIN",
			"Uuid":      uid.String(),
			"Plugin": map[string]string{
				"Name": plugin,
				"Type": pluginType,
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

func convertStringToJSON(s string) ([]byte, error) {
	req := &Request{}
	json.Unmarshal([]byte(s), req)
	return json.Marshal(req)
}

type Request struct {
	Operation string
	Uuid      string
	Plugin    map[string]string
}

func main() {
	flag.Parse()
	log.SetFlags(0)
	http.HandleFunc("/echo", echo)
	log.Println("listening on localhost:8080/echo...")
	log.Fatal(http.ListenAndServe(*addr, nil))
}
