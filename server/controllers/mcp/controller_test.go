// Copyright 2026 The Atlantis Authors
// SPDX-License-Identifier: Apache-2.0

package mcp

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/runatlantis/atlantis/server/jobs"
	"github.com/runatlantis/atlantis/server/logging"
	"github.com/stretchr/testify/assert"
)

func TestControllerRequiresAPISecretWhenConfigured(t *testing.T) {
	controller := NewController([]byte("secret"), logging.NewNoopLogger(t), jobs.JobQueryService{OutputHandler: &jobs.NoopProjectOutputHandler{}}, "test")
	req := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`))
	rec := httptest.NewRecorder()

	controller.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestControllerAllowsMCPInitializeWithoutAPISecret(t *testing.T) {
	controller := NewController(nil, logging.NewNoopLogger(t), jobs.JobQueryService{OutputHandler: &jobs.NoopProjectOutputHandler{}}, "test")
	req := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`))
	req.Header.Set("Accept", "application/json, text/event-stream")
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	controller.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), `"name":"atlantis"`)
}
