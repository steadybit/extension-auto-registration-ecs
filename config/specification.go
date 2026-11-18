// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2023 Steadybit GmbH

package config

import "errors"

type Specification struct {
	EcsClusterName    string   `json:"ecsClusterName" split_words:"true" required:"true"`
	AgentApiUsername  string   `json:"agentApiUsername" split_words:"true" default:"_"`
	AgentApiPassword  string   `json:"agentApiPassword" split_words:"true" required:"false"`
	DiscoveryInterval int      `json:"discoveryInterval" split_words:"true" required:"false" default:"30"`
	TaskFamilies      []string `json:"taskFamilies" split_words:"true" required:"false" default:"steadybit-extension-host,steadybit-extension-container,steadybit-extension-http,steadybit-extension-aws"`
	// Deprecated: AgentKey is only read as a fallback for AgentApiPassword.
	AgentKey string `json:"agentKey" split_words:"true" required:"false"`
}

// ResolveAgentApiPassword falls back to the deprecated AgentKey when no AgentApiPassword is set, and reports whether
// the deprecated AgentKey was configured at all.
func (s *Specification) ResolveAgentApiPassword() (usesDeprecatedAgentKey bool, err error) {
	if s.AgentApiPassword == "" {
		s.AgentApiPassword = s.AgentKey
	}
	if s.AgentApiPassword == "" {
		return false, errors.New("STEADYBIT_EXTENSION_AGENT_API_PASSWORD must be set")
	}
	return s.AgentKey != "", nil
}
