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

/*
Assistant is a client to facilitate communications between Agent and Cloud.
*/
type Assistant struct {
	Config     *config.Config              // stores plugins and agent config
	Requests   chan map[string]interface{} // fifo queue of requests from server
	connection *websocket.Conn             // Active websocket connection
	ctx        context.Context             // go's context
	done       chan bool                   // Channel used to stop server listener
}

// NewAssistant returns an Assistant for the given Config.
func NewAssistant(ctx context.Context, config *config.Config) (*Assistant, error) {
	// TODO: Make addr and url dynamic if necessary.
	var addr = flag.String("addr", "localhost:8080", "http service address")
	u := url.URL{Scheme: "ws", Host: *addr, Path: "/echo"}

	header := http.Header{}

	if v, exists := os.LookupEnv("INFLUX_TOKEN"); exists {
		header.Add("Authorization", "Token "+v)
	} else {
		// ? Should we terminate the program? Without a INFLUX_TOKEN we can't authenticate.
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

	go a.listenToServer()

	return a, nil
}

// Stop is ysed to clean up active connection and all channels
func (assistant *Assistant) Stop() {
	assistant.connection.Close()
	close(assistant.Requests)
	assistant.done <- true
	close(assistant.done)
}

// WriteToServer is used as a helper function to write status responses to server.
func (assistant *Assistant) WriteToServer(payload interface{}) error {
	/*
		TODO: Write a dedicated struct for the input of WriteToServer.
		Refer to the design doc for implementation.
		https://docs.google.com/document/d/1pnXrWgXCvlpe5tB3YAlOtO6rsvBxoV7DMPHL82KpCok/edit?usp=sharing
	*/

	err := assistant.connection.WriteJSON(payload)
	if err != nil {
		return err
	}
	return nil
}

// listenToServer takes requests from the server and puts it in Requests.
func (assistant *Assistant) listenToServer() {
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
		case <-assistant.done:
			return
		case <-assistant.ctx.Done():
			assistant.Stop()
		default:
			continue
		}
	}
}
