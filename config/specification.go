// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2023 Steadybit GmbH

package config

import "errors"

type Specification struct {
	EcsClusterName    string   `json:"ecsClusterName" split_words:"true" required:"true"`
	AgentKey          string   `json:"agentKey" split_words:"true" required:"false"`
	AgentApiUsername  string   `json:"agentApiUsername" split_words:"true" default:"_"`
	AgentApiPassword  string   `json:"agentApiPassword" split_words:"true" required:"false"`
	DiscoveryInterval int      `json:"discoveryInterval" split_words:"true" required:"false" default:"30"`
	TaskFamilies      []string `json:"taskFamilies" split_words:"true" required:"false" default:"steadybit-extension-host,steadybit-extension-container,steadybit-extension-http,steadybit-extension-aws"`
}

// AgentApiCredentials returns the Basic-auth credentials for the agent API. Like the agent itself, the password
// falls back to the agent key when no dedicated API password is configured.
func (s *Specification) AgentApiCredentials() (string, string) {
	if s.AgentApiPassword != "" {
		return s.AgentApiUsername, s.AgentApiPassword
	}
	return s.AgentApiUsername, s.AgentKey
}

func (s *Specification) Validate() error {
	if s.AgentKey == "" && s.AgentApiPassword == "" {
		return errors.New("either STEADYBIT_EXTENSION_AGENT_KEY or STEADYBIT_EXTENSION_AGENT_API_PASSWORD must be set")
	}
	return nil
}
