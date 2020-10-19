package assistant

import (
	"encoding/json"
	"testing"

	"github.com/influxdata/telegraf/agent"
	"github.com/influxdata/telegraf/config"
	_ "github.com/influxdata/telegraf/plugins/inputs/all"
	_ "github.com/influxdata/telegraf/plugins/outputs/all"
	"github.com/stretchr/testify/assert"
)

func TestAssistant_GetPluginAsJSON(t *testing.T) {
	c := config.NewConfig()
	err := c.LoadConfig("../config/testdata/telegraf-agent.toml")
	assert.NoError(t, err)
	ag, _ := agent.NewAgent(c)
	ast, _ := NewAssistant(&AssistantConfig{Host: "localhost:8080", Path: "/echo", RetryInterval: 15}, ag)

	for _, p := range ag.Config.Inputs {
		name := p.Config.Name
		req := request{GET_PLUGIN, "123", plugin{name, "INPUT", nil}}
		res := ast.getPlugin(req)
		assert.True(t, res.Status == "SUCCESS")
		_, err := json.Marshal(res)
		assert.NoError(t, err)
	}

	for _, p := range ag.Config.Outputs {
		name := p.Config.Name
		req := request{GET_PLUGIN, "123", plugin{name, "OUTPUT", nil}}
		res := ast.getPlugin(req)
		assert.True(t, res.Status == "SUCCESS")
		_, err := json.Marshal(res)
		assert.NoError(t, err)
	}

}
