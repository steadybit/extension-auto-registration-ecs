// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2023 Steadybit GmbH

package config

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func Test_ParseConfiguration(t *testing.T) {
	t.Setenv("STEADYBIT_EXTENSION_ECS_CLUSTER_NAME", "my-cluster")
	t.Setenv("STEADYBIT_EXTENSION_AGENT_KEY", "my-agent-key")

	ParseConfiguration()

	assert.Equal(t, "my-cluster", Config.EcsClusterName)
	assert.Equal(t, "my-agent-key", Config.AgentKey)
	// Not set above, so these must come from the struct tag defaults.
	assert.Equal(t, 30, Config.DiscoveryInterval)
	assert.Equal(t, []string{
		"steadybit-extension-host",
		"steadybit-extension-container",
		"steadybit-extension-http",
		"steadybit-extension-aws",
	}, Config.TaskFamilies)
}

func Test_ParseConfiguration_readsOptionalValues(t *testing.T) {
	t.Setenv("STEADYBIT_EXTENSION_ECS_CLUSTER_NAME", "my-cluster")
	t.Setenv("STEADYBIT_EXTENSION_AGENT_KEY", "my-agent-key")
	t.Setenv("STEADYBIT_EXTENSION_DISCOVERY_INTERVAL", "5")
	t.Setenv("STEADYBIT_EXTENSION_TASK_FAMILIES", "family-a,family-b")

	ParseConfiguration()

	assert.Equal(t, 5, Config.DiscoveryInterval)
	assert.Equal(t, []string{"family-a", "family-b"}, Config.TaskFamilies)
}

func Test_ParseConfiguration_fallsBackToDeprecatedAgentKey(t *testing.T) {
	Config = Specification{}
	t.Setenv("STEADYBIT_EXTENSION_ECS_CLUSTER_NAME", "my-cluster")
	t.Setenv("STEADYBIT_EXTENSION_AGENT_KEY", "my-agent-key")

	ParseConfiguration()

	assert.Equal(t, "_", Config.AgentApiUsername)
	assert.Equal(t, "my-agent-key", Config.AgentApiPassword)
}

func Test_ParseConfiguration_readsApiCredentials(t *testing.T) {
	Config = Specification{}
	t.Setenv("STEADYBIT_EXTENSION_ECS_CLUSTER_NAME", "my-cluster")
	t.Setenv("STEADYBIT_EXTENSION_AGENT_API_USERNAME", "registrar")
	t.Setenv("STEADYBIT_EXTENSION_AGENT_API_PASSWORD", "api-secret")

	ParseConfiguration()

	assert.Equal(t, "registrar", Config.AgentApiUsername)
	assert.Equal(t, "api-secret", Config.AgentApiPassword)
}

func Test_ResolveAgentApiPassword(t *testing.T) {
	_, err := (&Specification{}).ResolveAgentApiPassword()
	assert.Error(t, err)

	spec := Specification{AgentKey: "my-agent-key", AgentApiPassword: "api-secret"}
	usesDeprecatedAgentKey, err := spec.ResolveAgentApiPassword()
	assert.NoError(t, err)
	assert.True(t, usesDeprecatedAgentKey)
	assert.Equal(t, "api-secret", spec.AgentApiPassword)

	spec = Specification{AgentApiPassword: "api-secret"}
	usesDeprecatedAgentKey, err = spec.ResolveAgentApiPassword()
	assert.NoError(t, err)
	assert.False(t, usesDeprecatedAgentKey)
}
