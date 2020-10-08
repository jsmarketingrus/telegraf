package assistant

import (
	"context"
	"fmt"
	"log"

	"github.com/influxdata/telegraf/config"
	"golang.org/x/net/websocket"
)

// Assistant represents a client that will connect to the cloud server to allow
// for plugin management remotely.
type Assistant struct {
	Config *config.Config // stores plugins and agent config
}

// NewAssistant returns an Assistant for the given Config.
func NewAssistant(config *config.Config) (*Assistant, error) {
	a := &Assistant{
		Config: config,
	}
	return a, nil
}

// client stores the channels to communicate with the agent and the websocket
// connection to the cloud server.
// TODO determine if Ctx is necessary, and if so, what for
type client struct {
	ServerConn *websocket.Conn
	FromAgent  chan []byte
	ToAgent    chan []byte
	Ctx        context.Context
}

// agentListener relays messages received from the agent channel to the cloud
// server.
func (cli *client) agentListener() {
	for {
		select {
		case <-cli.Ctx.Done():
			return
		case msg := <-cli.FromAgent:
			fmt.Printf("Assistant received: \"%s\" from agent.\n", msg)
			if _, err := cli.ServerConn.Write(msg); err != nil {
				cli.ServerConn.Close()
				log.Fatal(err)
			}
		}

	}
}

// serverListener relays messages from the cloud server to the agent channel.
func (cli *client) serverListener() {
	var msg = make([]byte, 512)
	var n int
	var err error

	for {
		// Block on ServerConn.Read until a message is sent.
		// Cannot put into a `select` statement because it is not a channel.
		if n, err = cli.ServerConn.Read(msg); err != nil {
			log.Fatal(err)
			break
		}
		fmt.Printf("Assistant received: \"%s\" from server.\n", msg[:n])
		select {
		// TODO ensure that this message is handled and actually returns
		case <-cli.Ctx.Done():
			return
		case cli.ToAgent <- msg[:n]:
			continue
		}
	}
}

// Run is the main function that connects to the cloud server via websocket and
// starts the Telegraf assistant's listeners.
func (a *Assistant) Run(ctx context.Context, fromAgent chan []byte, toAgent chan []byte) error {
	log.Printf("Started assistant")

	origin := "http://localhost/"
	url := "ws://localhost:3001/ws"
	ws, err := websocket.Dial(url, "", origin)

	if err != nil {
		// TODO find a mechanism to retry connection or alert the agent
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
}
