package assistant

import (
	"context"
	"encoding/json"
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
		_, err := json.Marshal(res)
		assert.NoError(t, err)
	}

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

// ? Unsure what Data will contain
// TODO Implement assertions on res.Data
func TestAssistant_ListPlugins(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	_, ast := initAgentAndAssistant(ctx)
	time.Sleep(2 * time.Second)

	res := ast.listPlugins()
	assert.True(t, res.Status == SUCCESS)

	cancel()
}
