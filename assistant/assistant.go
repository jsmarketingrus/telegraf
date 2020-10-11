package assistant

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/gorilla/websocket"
	"github.com/influxdata/telegraf/agent"
	"github.com/influxdata/telegraf/internal"
)

/*
Assistant is a client to facilitate communications between Agent and Cloud.
*/
type Assistant struct {
	config     *AssistantConfig // stores plugins
	connection *websocket.Conn  // Active websocket connection
	ctx        context.Context  // go's context
	done       chan bool        // Channel used to stop server listener
	agent      *agent.Agent     // Pointer to agent to issue commands
}

/*
AssistantConfig allows us to configure where to connect to and other params
for the agent.
*/
type AssistantConfig struct {
	Host          string
	Path          string
	RetryInterval int
}

func (astConfig *AssistantConfig) fillDefaults() {
	if astConfig.Host == "" {
		astConfig.Host = "localhost:8080"
	}
	if astConfig.Path == "" {
		astConfig.Path = "/echo"
	}
	if astConfig.RetryInterval == 0 {
		astConfig.RetryInterval = 15
	}
}

// NewAssistant returns an Assistant for the given Config.
func NewAssistant(ctx context.Context, config *AssistantConfig, agent *agent.Agent) (*Assistant, error) {
	config.fillDefaults()
	var addr = flag.String("addr", config.Host, "http service address")
	u := url.URL{Scheme: "ws", Host: *addr, Path: config.Path}

	header := http.Header{}

	if v, exists := os.LookupEnv("INFLUX_TOKEN"); exists {
		header.Add("Authorization", "Token "+v)
	} else {
		return nil, fmt.Errorf("Influx authorization token not found, please set in env")
	}

	// creates a new websocket connection
	log.Printf("D! [assistant] Attempting connection to [%s]", config.Host)
	ws, _, err := websocket.DefaultDialer.Dial(u.String(), header)
	for err != nil { // on error, retry connection again
		log.Printf("E! [assistant] Failed to connect to [%s], retrying in %ds, "+
			"error was '%s'", config.Host, config.RetryInterval, err)

		err := internal.SleepContext(ctx, time.Duration(config.RetryInterval)*time.Second)
		if err != nil {
			return nil, err
		}

		ws, _, err = websocket.DefaultDialer.Dial(u.String(), header)
	}
	log.Printf("D! [assistant] Successfully connected to %s", config.Host)

	a := &Assistant{
		config:     config,
		connection: ws,
		ctx:        ctx,
		done:       make(chan bool),
		agent:      agent,
	}

	go a.listenToServer()

	return a, nil
}

// Stop is used to clean up active connection and all channels
func (assistant *Assistant) Stop() {
	assistant.connection.Close()
	assistant.done <- true
	close(assistant.done)
}

type plugin struct {
	Name   string
	Type   string
	Config map[string]interface{}
}

type Request struct {
	Operation string
	Uuid      string
	Plugin    plugin
}

type Response struct {
	Status string
	Uuid   string
	Data   interface{}
}

// WriteToServer is used as a helper function to write status responses to server.
func (assistant *Assistant) WriteToServer(payload Response) error {
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

func (assistant *Assistant) checkForTermination() {
	for {
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

// listenToServer takes requests from the server and puts it in Requests.
func (assistant *Assistant) listenToServer() {
	go assistant.checkForTermination()

	for {
		var req Request
		err := assistant.connection.ReadJSON(&req)
		if err != nil {
			fmt.Println(err)
		}

		var data string
		var res Response
		switch req.Operation {
		case "GET_PLUGIN":
			data = "TODO fetch plugin config"
			res = Response{"SUCCESS", req.Uuid, data}
			fmt.Print("Received request")
			fmt.Println(req)
		case "ADD_PLUGIN":
			// epic 2
			res = Response{"SUCCESS", req.Uuid, fmt.Sprintf("%s plugin added.", req.Plugin.Name)}
		case "UPDATE_PLUGIN":
			data = "TODO fetch plugin config"
			res = Response{"SUCCESS", req.Uuid, data}
		case "DELETE_PLUGIN":
			// epic 2
			res = Response{"SUCCESS", req.Uuid, fmt.Sprintf("%s plugin deleted.", req.Plugin.Name)}
		case "GET_ALL_PLUGINS":
			// epic 2
			data = "TODO fetch all available plugins"
			res = Response{"SUCCESS", req.Uuid, data}
		default:
			// return error response
			res = Response{"ERROR", req.Uuid, "invalid operation request"}
		}
		assistant.WriteToServer(res)

	}
}
