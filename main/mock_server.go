package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

var addr = flag.String("addr", "localhost:8080", "http service address")

var upgrader = websocket.Upgrader{}    // use default options
var webUpgrader = websocket.Upgrader{} // use default options

var connectAssistantWg sync.WaitGroup // waitgroup to open interface when connection established

var (
	requestWebChan       = make(chan map[string]interface{})
	responseWebChan      = make(chan map[string]interface{})
	requestTerminalChan  = make(chan map[string]interface{})
	responseTerminalChan = make(chan map[string]interface{})
)

func webInterface(w http.ResponseWriter, r *http.Request) {
	// init websocket conn with web server
	c, err := webUpgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Print("upgrade error:", err)
		return
	}
	defer c.Close()

	for {
		_, message, err := c.ReadMessage()
		if err != nil {
			log.Println("read:", err)
			break
		}

		var request map[string]interface{}
		err = json.Unmarshal([]byte(message), &request)
		if err != nil {
			log.Println("unmarshal failed with error:", err)
			break
		}

		operation, ok := request["operation"]
		if !ok {
			log.Println("please specify an operation")
			continue
		}
		plugin, _ := request["name"]
		pluginType, _ := request["type"]
		config, _ := request["config"]
		uniqueId, _ := request["uuid"]
		fmt.Printf("%s\n", operation)
		fmt.Printf("Plugin name / unique ID: %v\n", plugin)
		fmt.Printf("Plugin type: %v\n", plugin)
		fmt.Printf("Plugin config: %v", config)
		fmt.Printf("Plugin id: %v", uniqueId)

		uid, _ := uuid.NewRandom()

		// var config map[string]interface{}
		// _ = json.Unmarshal([]byte(pluginConfig), &config)
		var m = map[string]interface{}{
			"Operation": operation,
			"Uuid":      uid.String(),
			"Plugin": map[string]interface{}{
				"Name":     plugin,
				"Type":     pluginType,
				"Config":   config,
				"UniqueId": uniqueId,
			},
		}

		// send request
		requestWebChan <- m

		// write response back to web interface
		res := <-responseWebChan
		responseString, err := json.Marshal(res)
		if err != nil {
			log.Println("error responding to web interface:", err)
			break
		}
		err = c.WriteMessage(websocket.TextMessage, responseString)
		if err != nil {
			log.Println("error writing to web interface:", err)
			break
		}
	}
}

func terminalInterface() {

	reader := bufio.NewReader(os.Stdin)

	for { // gather terminal input
		fmt.Println("\n(0) GET_PLUGIN")
		fmt.Println("(1) START_PLUGIN")
		fmt.Println("(2) STOP_PLUGIN")
		fmt.Println("(3) UPDATE_PLUGIN")
		fmt.Println("(4) GET_RUNNING_PLUGINS")
		fmt.Println("(5) GET_ALL_PLUGINS")
		fmt.Println("(6) GET_PLUGIN_SCHEMA")
		fmt.Print("\nOperation: ")
		operation, _ := reader.ReadString('\n')
		fmt.Print("Plugin name: ")
		plugin, _ := reader.ReadString('\n')
		fmt.Print("Plugin type: ")
		pluginType, _ := reader.ReadString('\n')
		fmt.Print("Plugin config: ")
		pluginConfig, _ := reader.ReadString('\n')
		fmt.Print("Plugin uid: ")
		uniqueId, _ := reader.ReadString('\n')

		// clean strings
		plugin = strings.Replace(plugin, "\n", "", -1)
		pluginType = strings.Replace(pluginType, "\n", "", -1)
		operation = strings.Replace(operation, "\n", "", -1)
		pluginConfig = strings.Replace(pluginConfig, "\n", "", -1)
		uniqueId = strings.Replace(uniqueId, "\n", "", -1)

		switch operation {
		case "0":
			operation = "GET_PLUGIN"
		case "1":
			operation = "START_PLUGIN"
		case "2":
			operation = "STOP_PLUGIN"
		case "3":
			operation = "UPDATE_PLUGIN"
		case "4":
			operation = "GET_RUNNING_PLUGINS"
		case "5":
			operation = "GET_ALL_PLUGINS"
		case "6":
			operation = "GET_PLUGIN_SCHEMA"
		default:
			operation = ""
		}

		uid, _ := uuid.NewRandom()

		var config map[string]interface{}
		_ = json.Unmarshal([]byte(pluginConfig), &config)
		var m = map[string]interface{}{
			"Operation": operation,
			"Uuid":      uid.String(),
			"Plugin": map[string]interface{}{
				"Name":     plugin,
				"Type":     pluginType,
				"Config":   config,
				"UniqueId": uniqueId,
			},
		}

		// write m to channel
		requestTerminalChan <- m

		// wait for response
		<-responseTerminalChan
	}
}

func closeAssistantConn(c *websocket.Conn) {
	c.Close()
}

func config(w http.ResponseWriter, r *http.Request) {

	// blocks until connection established
	c, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Print("upgrade error: ", err)
		return
	}
	defer closeAssistantConn(c)
	// TODO fix thing where negative waitgroup counter
	connectAssistantWg.Done()

	// when either terminal or
	for {
		var m map[string]interface{}
		select {
		case m = <-requestWebChan:
			err = c.WriteJSON(m)
			if err != nil {
				log.Println("write:", err)
				break
			}
			_, res, err := c.ReadMessage()
			if err != nil {
				log.Println("read:", err)
				break
			}

			fmt.Print("Web Interface received request: \n %s\n", string(res))
			log.Println("\nOperation: ")

			var resMap map[string]interface{}
			_ = json.Unmarshal(res, &resMap)

			responseWebChan <- resMap

		case m = <-requestTerminalChan:
			err = c.WriteJSON(m)
			if err != nil {
				log.Println("write:", err)
				break
			}
			_, res, err := c.ReadMessage()
			if err != nil {
				log.Println("read:", err)
				break
			}
			log.Printf("recv: %s", res)

			// write m to channel
			responseTerminalChan <- nil
		}
	}
}

func home(w http.ResponseWriter, r *http.Request) {
	homeTemplate.Execute(w, "ws://"+r.Host+"/web")
}

func enableTerminal() {
	connectAssistantWg.Wait()

	// init terminal interface
	go terminalInterface()
}

func main() {
	connectAssistantWg.Add(1)

	flag.Parse()
	log.SetFlags(0)
	// web server interface
	http.HandleFunc("/", home)
	http.HandleFunc("/web", webInterface)
	// listens for websocket connection to Assistant
	http.HandleFunc("/assistant", config)
	log.Println("web server running on localhost:8080")
	log.Println("listening on localhost:8080/...\n\n")

	go enableTerminal()

	log.Fatal(http.ListenAndServe(*addr, nil))
}

// adapted from gorilla-websocket's example echo server
// https://github.com/gorilla/websocket/blob/master/examples/echo
var homeTemplate = template.Must(template.New("").Parse(`
<!DOCTYPE html>
<html>
<head>
<meta charset="utf-8">
<link rel="preconnect" href="https://fonts.gstatic.com">
<link href="https://fonts.googleapis.com/css2?family=Rubik:wght@300&display=swap" rel="stylesheet">
<style>
      body {
        font-family: 'Rubik', sans-serif;
	  }
	  .outer {
		height: 95vh;
		overflow: hidden;
		display: flex;
		flex-direction: row;
	  }
	  .inner {
		flex: 1;
		overflow-y: scroll;
		width:0;
		padding: 0% 3%;
	  }
	  div {
		border-radius: 4px;
	  }
	  pre {
		white-space: pre-wrap;
		word-wrap: break-word;
	  }
    </style>
<script>  
function setFocusOnDiv(d) {
	const scrollIntoViewOptions = { behavior: "smooth", block: "center" };
	d.scrollIntoView(scrollIntoViewOptions); 
};
window.addEventListener("load", function(evt) {
	var ws;
	ws = new WebSocket("{{.}}");
	ws.onmessage = function(evt) {
		data = JSON.parse(evt.data)
		printRes("RESPONSE: " + JSON.stringify(data, null, 2));
	}
	ws.onerror = function(evt) {
		data = JSON.parse(evt.data)
		printRes("RESPONSE: " + JSON.stringify(data, null, 2));
	}

    var printReq = function(message) {
		var d = document.createElement("pre");
		d.style.background = "linear-gradient(45deg,#066fc5,#00a3ff)";
		d.width = "100%";
		d.textContent = message;
		output.appendChild(d);
		setFocusOnDiv(d);
		};
		var printRes = function(message) {
			var d = document.createElement("pre");
			d.textContent = message;
			// d.style["white-space"]= "pre-wrap";
			output.appendChild(d);
		setFocusOnDiv(d);
    };
    
    document.getElementById("requestInput").onclick = function(evt) {
		evt.preventDefault();
        if (!ws) {
            return false;
		}
		var operation = document.querySelector("input[name=operation]:checked");
		var name = document.getElementById("plugin");
		var uuid = document.getElementById("uuid");
		var config = document.getElementById("config");
		req = JSON.stringify({
			"type": "INPUT",
			"operation": operation.value,
			"config": JSON.parse(config.value),
			"uuid": uuid.value,
			"name": name.value,
		}, null, 2)
		printReq("SEND: " + req);
        ws.send(req);
        return false;
    };
    document.getElementById("requestOutput").onclick = function(evt) {
		evt.preventDefault();
        if (!ws) {
            return false;
		}
		var operation = document.querySelector("input[name=operation]:checked");
		var name = document.getElementById("plugin");
		var uuid = document.getElementById("uuid");
		var config = document.getElementById("config");
		req = JSON.stringify({
			"type": "OUTPUT",
			"operation": operation.value,
			"config": JSON.parse(config.value),
			"uuid": uuid.value,
			"name": name.value,
		}, null, 2)
		printReq("SEND: " + req);
        ws.send(req);
        return false;
    };
});
</script>
</head>
<body>
<div class="outer">
<div class="inner">
<h1>Telegraf Assistant</h1>
<p>Configure Assistant to connect to "localhost:8080/assistant".
<p>
<form>
<p><input id="get" type="radio" name="operation" value="GET_PLUGIN">
<label for="get">Get Plugin</label><br>
<input id="start" type="radio" name="operation" value="START_PLUGIN">
<label for="start">Start Plugin</label><br>
<input id="stop" type="radio" name="operation" value="STOP_PLUGIN">
<label for="stop">Stop Plugin</label><br>
<input id="update" type="radio" name="operation" value="UPDATE_PLUGIN">
<label for="update">Update Plugin</label><br>
<input id="get_running" type="radio" name="operation" value="GET_RUNNING_PLUGINS">
<label for="get_running">Get Running Plugins</label><br>
<input id="get_all" type="radio" name="operation" value="GET_ALL_PLUGINS">
<label for="get_all">Get All Plugins</label><br>
<input id="get_schema" type="radio" name="operation" value="GET_PLUGIN_SCHEMA">
<label for="get_schema">Get Plugin Schema</label>

<p><label for="plugin">Plugin Name:</label>
<input id="plugin" type="text" value="">
<p><label for="uuid">Plugin Unique ID:</label>
<input id="uuid" type="text" value="">

<p><label for="config">Plugin Config:</label><br>
<textarea id="config" name="config" rows="10" cols="35">{}</textarea>
<p>

<button id="requestInput">Request for Input</button>
<button id="requestOutput">Request for Output</button>
</form>
</div><div class="inner" style="background:#1b1b29;color:white">
<div id="output" style="overflow-wrap:break-word;"></div>
</div>
</body>
</html>
`))
