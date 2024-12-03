// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2023 Steadybit GmbH

package config

type Specification struct {
	EcsClusterName    string   `json:"ecsClusterName" split_words:"true" required:"true"`
	AgentKey          string   `json:"agentKey" split_words:"true" required:"true"`
	DiscoveryInterval int      `json:"discoveryInterval" split_words:"true" required:"false" default:"30"`
	TaskFamilies      []string `json:"taskFamilies" split_words:"true" required:"false" default:"steadybit-extension-host,steadybit-extension-container,steadybit-extension-http,steadybit-extension-aws"`
}
