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
	_ "github.com/influxdata/telegraf/logger"
)

/*
Assistant is a client to facilitate communications between Agent and Cloud.
*/
type Assistant struct {
	config     *AssistantConfig // Configuration for Assitant's connection to server
	connection *websocket.Conn  // Active websocket connection
	done       chan struct{}    // Channel used to stop server listener
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

	a := &Assistant{
		config: config,
		done:   make(chan struct{}),
		agent:  agent,
	}

	return a, nil
}

// Stop is used to clean up active connection and all channels
func (assistant *Assistant) Stop() {
	assistant.done <- struct{}{}
}

type plugin struct {
	Name   string
	Type   string
	Config map[string]interface{}
}

type RequestType string

const (
	GET_PLUGIN      = RequestType("GET_PLUGIN")
	ADD_PLUGIN      = RequestType("ADD_PLUGIN")
	UPDATE_PLUGIN   = RequestType("UPDATE_PLUGIN")
	DELETE_PLUGIN   = RequestType("DELETE_PLUGIN")
	GET_ALL_PLUGINS = RequestType("GET_ALL_PLUGINS")
)

type Request struct {
	Operation RequestType
	Uuid      string
	Plugin    plugin
}

type Response struct {
	Status string
	Uuid   string
	Data   interface{}
}

const (
	SUCCESS = "SUCCESS"
	FAILURE = "FAILURE"
)

// Run starts the assistant listening to the server and handles and interrupts or closed connections
func (assistant *Assistant) Run(ctx context.Context) error {
	var config = assistant.config
	var addr = flag.String("addr", config.Host, "http service address")
	u := url.URL{Scheme: "ws", Host: *addr, Path: config.Path}

	header := http.Header{}

	if v, exists := os.LookupEnv("INFLUX_TOKEN"); exists {
		header.Add("Authorization", "Token "+v)
	} else {
		return fmt.Errorf("influx authorization token not found, please set in env")
	}

	// creates a new websocket connection
	log.Printf("D! [assistant] Attempting connection to [%s]", config.Host)
	ws, _, err := websocket.DefaultDialer.Dial(u.String(), header)
	for err != nil { // on error, retry connection again
		log.Printf("E! [assistant] Failed to connect to [%s], retrying in %ds, "+
			"error was '%s'", config.Host, config.RetryInterval, err)

		sleepErr := internal.SleepContext(ctx, time.Duration(config.RetryInterval)*time.Second)
		if sleepErr != nil {
			return sleepErr
		}

		ws, _, err = websocket.DefaultDialer.Dial(u.String(), header)
	}
	log.Printf("D! [assistant] Successfully connected to %s", config.Host)
	assistant.connection = ws

	defer assistant.connection.Close()
	go assistant.listenToServer(ctx)
	for {
		select {
		case <-assistant.done:
			return nil
		case <-ctx.Done():
			log.Printf("I! [assistant] Hang on, closing connection to server before shutdown")
			return nil
		}
	}
}

// writeToServer is used as a helper function to write status responses to server.
func (assistant *Assistant) writeToServer(payload Response) error {
	err := assistant.connection.WriteJSON(payload)
	return err
}

// listenToServer takes requests from the server and puts it in Requests.
func (assistant *Assistant) listenToServer(ctx context.Context) {
	defer close(assistant.done)
	for {
		var req Request
		err := assistant.connection.ReadJSON(&req)
		if err != nil {
			// TODO add error handling for different types of errors
			// common error that we see now is trying to read from a closed connection
			log.Printf("E! [assistant] error while reading from server: %s", err)
			return
		}
		var res Response
		switch req.Operation {
		case GET_PLUGIN:
			fmt.Print("D! [assistant] Received request: ", req.Operation, " for plugin ", req.Plugin.Name, "\n")
			var data interface{}
			var err error
			switch req.Plugin.Type {
			case "INPUT":
				data, err = assistant.agent.GetInputPlugin(req.Plugin.Name)
			case "OUTPUT":
				data, err = assistant.agent.GetOutputPlugin(req.Plugin.Name)
			case "AGGREGATOR":
				data, err = assistant.agent.GetAggregatorPlugin(req.Plugin.Name)
			case "PROCESSOR":
				data, err = assistant.agent.GetProcessorPlugin(req.Plugin.Name)
			default:
				err = fmt.Errorf("did not provide a valid plugin type")
			}

			if err == nil && data != nil {
				res = Response{"SUCCESS", req.Uuid, data}
			} else {
				res = Response{"FAILURE", req.Uuid, err.Error()}
			}

		case ADD_PLUGIN:
			// epic 2
			res = Response{"SUCCESS", req.Uuid, fmt.Sprintf("%s plugin added.", req.Plugin.Name)}
		case UPDATE_PLUGIN:
			data := "TODO fetch plugin config"
			res = Response{"SUCCESS", req.Uuid, data}
		case DELETE_PLUGIN:
			// epic 2
			res = Response{"SUCCESS", req.Uuid, fmt.Sprintf("%s plugin deleted.", req.Plugin.Name)}
		case GET_ALL_PLUGINS:
			// epic 2
			data := "TODO fetch all available plugins"
			res = Response{"SUCCESS", req.Uuid, data}
		default:
			// return error response
			res = Response{"ERROR", req.Uuid, "invalid operation request"}
		}
		err = assistant.writeToServer(res)
		if err != nil {
			// log error and keep connection open
			log.Printf("E! [assistant] Error while writing to server: %s", err)
		}

	}
}
