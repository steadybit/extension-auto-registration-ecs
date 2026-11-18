// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2026 Steadybit GmbH

package autoregistration

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-resty/resty/v2"
	"github.com/steadybit/extension-auto-registration-ecs/config"
	"github.com/stretchr/testify/assert"
)

func Test_registrationCallsSendConfiguredCredentials(t *testing.T) {
	original := config.Config
	t.Cleanup(func() { config.Config = original })
	config.Config = config.Specification{AgentApiUsername: "registrar", AgentApiPassword: "api-secret"}

	type call struct{ method, username, password string }
	var calls []call
	agent := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		username, password, _ := r.BasicAuth()
		calls = append(calls, call{r.Method, username, password})
	}))
	defer agent.Close()
	httpClient := resty.New().SetBaseURL(agent.URL)

	addNewRegistrations(httpClient, &[]extensionConfigAO{}, &[]extensionConfigAO{{Url: "http://new:8080"}})
	removeMissingRegistrations(httpClient, &[]extensionConfigAO{{Url: "http://gone:8080"}}, &[]extensionConfigAO{})

	assert.Equal(t, []call{
		{http.MethodPost, "registrar", "api-secret"},
		{http.MethodDelete, "registrar", "api-secret"},
	}, calls)
}
