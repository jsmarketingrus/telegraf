package assistant

import (
	"context"
	"testing"
	"time"

	"github.com/influxdata/telegraf/agent"
	"github.com/influxdata/telegraf/config"
	_ "github.com/influxdata/telegraf/plugins/inputs/all"
	_ "github.com/influxdata/telegraf/plugins/outputs/all"
	"github.com/stretchr/testify/assert"
)

func initAgentAndAssistant(ctx context.Context) (*agent.Agent, *Assistant) {
	c := config.NewConfig()
	_ = c.LoadConfig("../config/testdata/single_plugin.toml")
	ag, _ := agent.NewAgent(c)
	ast, _ := NewAssistant(&AssistantConfig{Host: "localhost:8080", Path: "/echo", RetryInterval: 15}, ag)

	go func() {
		ag.Run(ctx)
	}()

	return ag, ast
}

func TestAssistant_GetPluginAsJSON(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	ag, ast := initAgentAndAssistant(ctx)
	time.Sleep(2 * time.Second)

	// ! BROKEN UNTIL MERGE!
	// for inputName := range inputs.Inputs {
	// 	ag.AddInput(ctx, inputName)
	// }

	// for outputName := range outputs.Outputs {
	// 	ag.AddOutput(ctx, outputName)
	// }

	for _, p := range ag.Config.Inputs {
		name := p.Config.Name
		req := request{GET_PLUGIN, "123", plugin{name, "INPUT", nil}}
		res := ast.getPlugin(req)
		assert.True(t, res.Status == SUCCESS)
		_, err := json.Marshal(res)
		assert.NoError(t, err)
	}

	for _, p := range ag.Config.Outputs {
		name := p.Config.Name
		req := request{GET_PLUGIN, "123", plugin{name, "OUTPUT", nil}}
		res := ast.getPlugin(req)
		assert.True(t, res.Status == SUCCESS)
		// _, err := json.Marshal(res)
		// assert.NoError(t, err)
	}

	cancel()
}

func TestAssistant_UpdatePlugin(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	_, ast := initAgentAndAssistant(ctx)

	time.Sleep(5 * time.Second)

	servers := true
	unixSockets := []string{"ubuntu"}
	testReq := request{UPDATE_PLUGIN, "69", plugin{"memcached", "INPUT", map[string]interface{}{
		"Servers":     servers,
		"UnixSockets": unixSockets,
	}}}

	response := ast.updatePlugin(testReq)
	t.Log(response)
	assert.True(t, response.Status == SUCCESS)

	// plugin := ast.getPlugin(testReq)
	// t.Log(plugin)

	// v, ok := plugin.(memcached.Memcached)

	// if ok {
	// 	t.Log(v.Servers)
	// 	assert.True(t, v.Servers[0] == "go")
	// }

	cancel()

}

// ? Unsure what Data will contain
// TODO Implement assertions on res.Data
func TestAssistant_ListActivePlugins(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	_, ast := initAgentAndAssistant(ctx)
	time.Sleep(2 * time.Second)

	res := ast.listActivePlugins()
	assert.True(t, res.Status == SUCCESS)

	cancel()
}

func TestAssistant_ListPlugins(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	_, ast := initAgentAndAssistant(ctx)
	time.Sleep(2 * time.Second)

	res := ast.listPlugins()
	assert.True(t, res.Status == SUCCESS)

	cancel()
}
