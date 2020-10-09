package assistant

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"net/url"
	"os"

	"github.com/gorilla/websocket"
	"github.com/influxdata/telegraf/config"
)

// Assistant represents a client that will connect to the cloud server to allow
// for plugin management remotely.
type Assistant struct {
	Config     *config.Config              // stores plugins and agent config
	Requests   chan map[string]interface{} // Queue of requests from server
	connection *websocket.Conn
	ctx        context.Context
	done       chan bool
}

// NewAssistant returns an Assistant for the given Config.
func NewAssistant(ctx context.Context, config *config.Config) (*Assistant, error) {
	var addr = flag.String("addr", "localhost:8080", "http service address")
	u := url.URL{Scheme: "ws", Host: *addr, Path: "/echo"}

	header := http.Header{}

	if v, exists := os.LookupEnv("INFLUX_TOKEN"); exists {
		header.Add("Authorization", "Token "+v)
	}

	ws, _, err := websocket.DefaultDialer.Dial(u.String(), header)

	if err != nil {
		return nil, err
	}

	a := &Assistant{
		Config:     config,
		Requests:   make(chan map[string]interface{}),
		connection: ws,
		ctx:        ctx,
		done:       make(chan bool),
	}

	go a.serverListener()

	return a, nil
}

func (assistant *Assistant) Stop() {
	assistant.connection.Close()
	close(assistant.Requests)
	assistant.done <- true
}

/*
	TODO: Write a dedicated struct for the input of WriteToServer.
	Refer to the design doc for implementation.
*/
func (assistant *Assistant) WriteToServer(payload interface{}) error {
	err := assistant.connection.WriteJSON(payload)
	if err != nil {
		return err
	}
	return nil
}

// serverListener relays messages from the cloud server to the agent channel.
func (assistant *Assistant) serverListener() {
	for {
		var res map[string]interface{}
		err := assistant.connection.ReadJSON(&res)
		if err != nil {
			fmt.Println(err)
		}
		if len(res) != 0 {
			assistant.Requests <- res
		}
		select {
		case <-assistant.ctx.Done():
			return
		case <-assistant.done:
			return
		default:
			continue
		}
	}
}
