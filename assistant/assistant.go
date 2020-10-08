package assistant

import (
	"context"
	"fmt"
	"log"
	"golang.org/x/net/websocket"
	"github.com/influxdata/telegraf/config"
)

// 1. Should we make assistant in agent.Run? Where's the best place to access necessary data/resources
// 2. If we shouldn't where should we instantiate the assistant
// 3. If something goes wrong with the assistant, should we terminate Telegraf with log.Fatal

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
	FromAgent chan []byte
	ToAgent chan []byte
	Ctx context.Context
}

func (cli *client) agentListener() {
	// Relays Agent Channel messages to the Server

	for {
		select {
		case <-cli.Ctx.Done():
			return;
		case msg := <-cli.FromAgent:
			fmt.Printf("Server Received: \"%s\" from agent.\n", msg)
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
		fmt.Printf("Agent Received: \"%s\" from server.\n", msg[:n])
		select {
		case <-cli.Ctx.Done():
			return;
		case cli.ToAgent <- msg[:n]:
			continue
		}
	}
}

func (a *Assistant) Run(ctx context.Context, fromAgent chan []byte, toAgent chan []byte) error {
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
	cli := client{ServerConn: ws, FromAgent: fromAgent, ToAgent: toAgent, Ctx: ctx}

	go cli.agentListener()
	go cli.serverListener()

	for {
		select {
		case <-cli.Ctx.Done():
			cli.ServerConn.Close()
			close(cli.ToAgent)
			close(cli.FromAgent)
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
