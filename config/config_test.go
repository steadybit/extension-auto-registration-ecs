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
