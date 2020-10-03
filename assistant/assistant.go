package assistant

import (
	"context"
	"log"
	"time"

	"github.com/influxdata/telegraf/config"
	"golang.org/x/net/websocket"
)

type Assistant struct {
	Config *config.Config
}

func NewAssistant(config *config.Config) (*Assistant, error) {
	a := &Assistant{
		Config: config,
	}
	return a, nil
}

func (a *Assistant) Run(ctx context.Context) error {
	log.Printf("Started assistant")
	// ! Don't use log.Fatal as that will terminate the whole process
	origin := "http://localhost/"
	url := "ws://localhost:3001/ws"
	ws, err := websocket.Dial(url, "", origin)
	if err != nil {
		// ws.Close()
		// We should close the websocket if there is an error
		log.Fatal(err)
	}
	if _, err := ws.Write([]byte("hello, world!\n")); err != nil {
		// ws.Close()
		// We should close the websocket if there is an error
		log.Fatal(err)
	}

	// TODO:

	for {
		time.Sleep(2 * time.Second)
		ws.Write([]byte("The websocket is still connected!\n"))
	}

	// var msg = make([]byte, 512)
	// var n int
	// if n, err = ws.Read(msg); err != nil {
	// 	// ws.Close()
	// 	// We should close the websocket if there is an error
	// 	log.Fatal(err)
	// }
	// fmt.Printf("Received: %s.\n", msg[:n])
	return nil
}
