// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2023 Steadybit GmbH

package config

import (
	"github.com/kelseyhightower/envconfig"
	"github.com/rs/zerolog/log"
)

var (
	Config Specification
)

func ParseConfiguration() {
	err := envconfig.Process("steadybit_extension", &Config)
	if err != nil {
		log.Fatal().Err(err).Msgf("Failed to parse configuration from environment.")
	}
	usesDeprecatedAgentKey, err := Config.ResolveAgentApiPassword()
	if err != nil {
		log.Fatal().Err(err).Msgf("Invalid configuration.")
	}
	if usesDeprecatedAgentKey {
		log.Warn().Msg("STEADYBIT_EXTENSION_AGENT_KEY is deprecated, use STEADYBIT_EXTENSION_AGENT_API_PASSWORD instead.")
	}
}
