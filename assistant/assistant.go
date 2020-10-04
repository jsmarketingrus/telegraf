package assistant

import (
	"context"
	"fmt"
	"log"

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

type client struct {
	ServerConn *websocket.Conn
	AgentChan  chan []byte
	Ctx        context.Context
}

func (cli *client) agentListener() {
	// Relays Agent Channel messages to the Server

	for {
		select {
		case <-cli.Ctx.Done():
			return
		case msg := <-cli.AgentChan:
			fmt.Printf("Received: \"%s\" from agent.\n", msg)
			if _, err := cli.ServerConn.Write(msg); err != nil {
				cli.ServerConn.Close()
				log.Fatal(err)
			}
		}

	}
}

func (cli *client) serverListener() {
	// RelaysServer messages to the Agent Channel

	var msg = make([]byte, 512)
	var n int
	var err error

	for {
		if n, err = cli.ServerConn.Read(msg); err != nil {
			log.Fatal(err)
			break
		}
		fmt.Printf("Received: \"%s\" from server.\n", msg[:n])
		select {
		case <-cli.Ctx.Done():
			cli.ServerConn.Close()
			close(cli.AgentChan)
		case cli.AgentChan <- msg[:n]:
			continue
		}
	}
}

func (a *Assistant) Run(ctx context.Context, c chan []byte) error {
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
	cli := client{ServerConn: ws, AgentChan: c, Ctx: ctx}

	go cli.agentListener()
	go cli.serverListener()

	for {
		select {
		case <-cli.Ctx.Done():
			cli.ServerConn.Close()
			close(cli.AgentChan)
			return nil
		}
	}

	// var msg = make([]byte, 512)
	// var n int
	// if n, err = ws.Read(msg); err != nil {
	// 	// ws.Close()
	// 	// We should close the websocket if there is an error
	// 	log.Fatal(err)
	// }
	// fmt.Printf("Received: %s.\n", msg[:n])
}
