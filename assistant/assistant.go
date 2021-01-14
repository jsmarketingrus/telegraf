package assistant

import (
	"context"
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
	config *AssistantConfig // Configuration for Assitant's conn to server
	conn   *websocket.Conn  // Active websocket conn
	running bool
	agent  *agent.Agent     // Pointer to agent to issue commands
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

func NewAssistantConfig() *AssistantConfig {
	return &AssistantConfig{
		Host: "localhost:8080",
		Path: "/echo",
		RetryInterval: 15,
	}
}

// NewAssistant returns an Assistant for the given Config.
func NewAssistant(config *AssistantConfig, agent *agent.Agent) *Assistant {
	return &Assistant{
		config: config,
		agent:  agent,
		running: true,
	}
}

// Stop is used to clean up active conn and all channels
func (assistant *Assistant) Stop() {
	assistant.running = false
	assistant.conn.Close()
}

type pluginInfo struct {
	Name     string
	Type     string
	Config   map[string]interface{}
	UniqueId string
}

type requestType string

const (
	GET_PLUGIN          = requestType("GET_PLUGIN")
	GET_PLUGIN_SCHEMA   = requestType("GET_PLUGIN_SCHEMA")
	UPDATE_PLUGIN       = requestType("UPDATE_PLUGIN")
	START_PLUGIN        = requestType("START_PLUGIN")
	STOP_PLUGIN         = requestType("STOP_PLUGIN")
	GET_RUNNING_PLUGINS = requestType("GET_RUNNING_PLUGINS")
	GET_ALL_PLUGINS     = requestType("GET_ALL_PLUGINS")

	SUCCESS = "SUCCESS"
	FAILURE = "FAILURE"
)

type request struct {
	Operation requestType
	UUID      string
	Plugin    pluginInfo
}

type response struct {
	Status string
	UUID   string
	Data   interface{}
}

func (a *Assistant) init(ctx context.Context) error {
	token, exists := os.LookupEnv("INFLUX_TOKEN")
	if !exists {
		return fmt.Errorf("influx authorization token not found, please set in env")
	}

	header := http.Header{}
	header.Add("Authorization", "Token " + token)
	u := url.URL{Scheme: "ws", Host: a.config.Host, Path: a.config.Path}

	log.Printf("D! [assistant] Attempting conn to [%s]", a.config.Host)
	ws, _, err := websocket.DefaultDialer.Dial(u.String(), header)
	for err != nil { // on error, retry conn again
		log.Printf("E! [assistant] Failed to connect to [%s] due to: '%s'. Retrying in %ds... ",
			a.config.Host, err, a.config.RetryInterval)

		err = internal.SleepContext(ctx, time.Duration(a.config.RetryInterval)*time.Second)
		if err != nil {
			// Return because context was closed
			return err
		}

		ws, _, err = websocket.DefaultDialer.Dial(u.String(), header)
	}
	a.conn = ws

	log.Printf("D! [assistant] Successfully connected to %s", a.config.Host)
	return nil
}

// Run starts the assistant listening to the server and handles and interrupts or closed connections
func (a *Assistant) Run(ctx context.Context) error {
	err := a.init(ctx)
	if err != nil {
		log.Printf("E! [assistant] connection could not be established: %s", err.Error())
		return err
	}
	a.running = true

	go a.listen(ctx)

	return nil
}

// listenToServer takes requests from the server and puts it in Requests.
func (a *Assistant) listen(ctx context.Context) {
	defer a.conn.Close()

	go a.shutdownOnContext(ctx)

	for {
		var req *request
		if err := a.conn.ReadJSON(req); err != nil {
			if !a.running {
				log.Printf("I! [assistant] listener shutting down...")
				return
			}

			log.Printf("E! [assistant] error while reading from server: %s", err)
			// retrying a new websocket connection
			err := a.init(ctx)
			if err != nil {
				log.Printf("E! [assistant] connection could not be re-established: %s", err)
				return
			}
			err = a.conn.ReadJSON(&req)
			if err != nil {
				log.Printf("E! [assistant] re-established connection but could not read server request: %s", err)
				return
			}
		}
		res := a.handleRequest(ctx, req)

		if err := a.conn.WriteJSON(res); err != nil {
			log.Printf("E! [assistant] Error while writing to server: %s", err)
			a.conn.WriteJSON(response{FAILURE, req.UUID, "error marshalling config"})
		}
	}
}

func (a *Assistant) shutdownOnContext(ctx context.Context) {
	<-ctx.Done()
	a.running = false
	a.conn.Close()
}

func (assistant *Assistant) handleRequest(ctx context.Context, req *request) response {
	var res response
	switch req.Operation {
	case GET_PLUGIN:
		res = assistant.getPlugin(req)
	case GET_PLUGIN_SCHEMA:
		res = assistant.getSchema(req)
	case START_PLUGIN:
		res = assistant.startPlugin(ctx, req)
	case STOP_PLUGIN:
		res = assistant.stopPlugin(req)
	case UPDATE_PLUGIN:
		res = assistant.updatePlugin(req)
	case GET_RUNNING_PLUGINS:
		res = assistant.getRunningPlugins(req)
	case GET_ALL_PLUGINS:
		res = assistant.getAllPlugins(req)
	default:
		// return error response
		res = response{FAILURE, req.UUID, "invalid operation request"}
	}
	return res
}

// getPlugin returns the struct response containing config for a single plugin
func (assistant *Assistant) getPlugin(req *request) response {
	fmt.Print("D! [assistant] Received request: ", req.Operation, " for plugin ", req.Plugin.Name, "\n")

	var data interface{}
	var err error

	data, err = assistant.agent.GetRunningPlugin(req.Plugin.UniqueId)
	if err != nil {
		return response{FAILURE, req.UUID, err.Error()}
	}

	return response{SUCCESS, req.UUID, data}
}

type schema struct {
	Types    map[string]interface{}
	Defaults map[string]interface{}
}

// getSchema returns the struct response containing config schema for a single plugin
func (assistant *Assistant) getSchema(req *request) response {
	fmt.Print("D! [assistant] Received request: ", req.Operation, " for plugin ", req.Plugin.Name, "\n")

	var plugin interface{}
	var err error

	switch req.Plugin.Type {
	case "INPUT":
		plugin, err = assistant.agent.CreateInput(req.Plugin.Name)
	case "OUTPUT":
		plugin, err = assistant.agent.CreateOutput(req.Plugin.Name)
	default:
		err = fmt.Errorf("did not provide a valid plugin type")
	}
	if err != nil {
		return response{FAILURE, req.UUID, err.Error()}
	}

	types, typesErr := assistant.agent.GetPluginTypes(plugin)
	if typesErr != nil {
		return response{FAILURE, req.UUID, err.Error()}
	}

	defaultValues, dvErr := assistant.agent.GetPluginValues(plugin)
	if dvErr != nil {
		return response{FAILURE, req.UUID, err.Error()}
	}

	return response{SUCCESS, req.UUID, schema{types, defaultValues}}
}

// startPlugin starts a single plugin
func (assistant *Assistant) startPlugin(ctx context.Context, req *request) response {
	fmt.Print("D! [assistant] Received request: ", req.Operation, " for plugin ", req.Plugin.Name, "\n")

	var res response
	var uid string
	var err error

	switch req.Plugin.Type {
	case "INPUT":
		uid, err = assistant.agent.StartInput(ctx, req.Plugin.Name)
	case "OUTPUT":
		uid, err = assistant.agent.StartOutput(req.Plugin.Name)
	default:
		err = fmt.Errorf("did not provide a valid plugin type")
	}

	if err != nil {
		res = response{FAILURE, req.UUID, err.Error()}
	} else {
		res = response{SUCCESS, req.UUID, uid}
	}

	return res
}

// updatePlugin updates a plugin with the config specified in request
func (assistant *Assistant) updatePlugin(req *request) response {
	fmt.Print("D! [assistant] Received request: ", req.Operation, " for plugin ", req.Plugin.UniqueId, "\n")

	var res response
	var data interface{}
	var err error

	if req.Plugin.Config == nil {
		res = response{FAILURE, req.UUID, "no config specified!"}
		return res
	}

	switch req.Plugin.Type {
	case "INPUT":
		data, err = assistant.agent.UpdateInputPlugin(req.Plugin.UniqueId, req.Plugin.Config)
	case "OUTPUT":
		data, err = assistant.agent.UpdateOutputPlugin(req.Plugin.UniqueId, req.Plugin.Config)
	default:
		err = fmt.Errorf("did not provide a valid plugin type")
	}

	if err != nil {
		res = response{FAILURE, req.UUID, err.Error()}
	} else {
		res = response{SUCCESS, req.UUID, data}
	}

	return res
}

// stopPlugin stops a single plugin
func (assistant *Assistant) stopPlugin(req *request) response {
	fmt.Print("D! [assistant] Received request: ", req.Operation, " for plugin ", req.Plugin.Name, "\n")

	var res response
	var err error

	switch req.Plugin.Type {
	case "INPUT":
		assistant.agent.StopInputPlugin(req.Plugin.UniqueId, true)
	case "OUTPUT":
		assistant.agent.StopOutputPlugin(req.Plugin.UniqueId, true)
	default:
		err = fmt.Errorf("did not provide a valid plugin type")
	}

	if err != nil {
		res = response{FAILURE, req.UUID, err.Error()}
	} else {
		res = response{SUCCESS, req.UUID, fmt.Sprintf("%s plugin deleted.", req.Plugin.Name)}
	}

	return res
}

type pluginsWithIdList struct {
	Inputs  []map[string]string
	Outputs []map[string]string
}

// getRunningPlugins returns a JSON response obj with all running plugins
func (assistant *Assistant) getRunningPlugins(req *request) response {
	inputs := assistant.agent.GetRunningInputPlugins()
	outputs := assistant.agent.GetRunningOutputPlugins()
	data := pluginsWithIdList{inputs, outputs}

	return response{SUCCESS, req.UUID, data}
}

type pluginsList struct {
	Inputs  []string
	Outputs []string
}

// getAllPlugins returns a JSON response obj with names of all possible plugins
func (assistant *Assistant) getAllPlugins(req *request) response {
	inputs := agent.GetAllInputPlugins()
	outputs := agent.GetAllOutputPlugins()
	data := pluginsList{inputs, outputs}
	res := response{SUCCESS, req.UUID, data}
	return res
}
