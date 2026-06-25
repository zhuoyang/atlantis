// Copyright 2026 The Atlantis Authors
// SPDX-License-Identifier: Apache-2.0

package mcp

import (
	"context"
	"fmt"
	"net/http"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/runatlantis/atlantis/server/jobs"
	"github.com/runatlantis/atlantis/server/logging"
)

const atlantisTokenHeader = "X-Atlantis-Token"

type Controller struct {
	APISecret []byte
	Logger    logging.SimpleLogging
	handler   http.Handler
}

type ListPRJobsInput struct {
	RepoFullName string `json:"repo_full_name" jsonschema:"Atlantis repository full name, for example owner/repo"`
	PullNum      int    `json:"pull_num" jsonschema:"pull request number"`
	Command      string `json:"command,omitempty" jsonschema:"optional Atlantis command filter, usually plan or apply"`
	ProjectName  string `json:"project_name,omitempty" jsonschema:"optional Atlantis project name filter"`
	Path         string `json:"path,omitempty" jsonschema:"optional project path relative to the repository root"`
	Workspace    string `json:"workspace,omitempty" jsonschema:"optional Terraform workspace filter"`
	LatestOnly   bool   `json:"latest_only,omitempty" jsonschema:"return only the latest job per project/path/workspace/command"`
}

type ListPRJobsOutput struct {
	Jobs []jobs.Job `json:"jobs" jsonschema:"matching Atlantis plan/apply jobs"`
}

type GetPRLogsInput struct {
	RepoFullName string `json:"repo_full_name,omitempty" jsonschema:"Atlantis repository full name, for example owner/repo"`
	PullNum      int    `json:"pull_num,omitempty" jsonschema:"pull request number"`
	JobID        string `json:"job_id,omitempty" jsonschema:"optional Atlantis job id to fetch directly"`
	Command      string `json:"command,omitempty" jsonschema:"optional Atlantis command filter, usually plan or apply"`
	ProjectName  string `json:"project_name,omitempty" jsonschema:"optional Atlantis project name filter"`
	Path         string `json:"path,omitempty" jsonschema:"optional project path relative to the repository root"`
	Workspace    string `json:"workspace,omitempty" jsonschema:"optional Terraform workspace filter"`
	LatestOnly   bool   `json:"latest_only,omitempty" jsonschema:"return only the latest job per project/path/workspace/command"`
}

type GetPRLogsOutput struct {
	Logs []jobs.JobLog `json:"logs" jsonschema:"matching Atlantis plan/apply logs grouped by job"`
}

func NewController(apiSecret []byte, logger logging.SimpleLogging, service jobs.JobQueryService, version string) *Controller {
	server := mcpsdk.NewServer(&mcpsdk.Implementation{
		Name:    "atlantis",
		Version: version,
	}, nil)

	mcpsdk.AddTool(server, &mcpsdk.Tool{
		Name:        "list_pr_jobs",
		Title:       "List PR Jobs",
		Description: "List Atlantis plan/apply log jobs for a pull request, including stale/latest metadata.",
	}, func(_ context.Context, _ *mcpsdk.CallToolRequest, input ListPRJobsInput) (*mcpsdk.CallToolResult, ListPRJobsOutput, error) {
		if input.RepoFullName == "" || input.PullNum == 0 {
			return nil, ListPRJobsOutput{}, fmt.Errorf("repo_full_name and pull_num are required")
		}
		return nil, ListPRJobsOutput{Jobs: service.ListPRJobs(jobs.JobFilters{
			RepoFullName: input.RepoFullName,
			PullNum:      input.PullNum,
			Command:      input.Command,
			ProjectName:  input.ProjectName,
			Path:         input.Path,
			Workspace:    input.Workspace,
			LatestOnly:   input.LatestOnly,
		})}, nil
	})

	mcpsdk.AddTool(server, &mcpsdk.Tool{
		Name:        "get_pr_logs",
		Title:       "Get PR Logs",
		Description: "Get buffered Terraform plan/apply logs for Atlantis jobs matching a pull request or job id.",
	}, func(_ context.Context, _ *mcpsdk.CallToolRequest, input GetPRLogsInput) (*mcpsdk.CallToolResult, GetPRLogsOutput, error) {
		if input.JobID == "" && (input.RepoFullName == "" || input.PullNum == 0) {
			return nil, GetPRLogsOutput{}, fmt.Errorf("job_id or both repo_full_name and pull_num are required")
		}
		return nil, GetPRLogsOutput{Logs: service.GetPRLogs(jobs.JobFilters{
			RepoFullName: input.RepoFullName,
			PullNum:      input.PullNum,
			Command:      input.Command,
			ProjectName:  input.ProjectName,
			Path:         input.Path,
			Workspace:    input.Workspace,
			JobID:        input.JobID,
			LatestOnly:   input.LatestOnly,
		})}, nil
	})

	return &Controller{
		APISecret: apiSecret,
		Logger:    logger,
		handler: mcpsdk.NewStreamableHTTPHandler(func(*http.Request) *mcpsdk.Server {
			return server
		}, &mcpsdk.StreamableHTTPOptions{JSONResponse: true}),
	}
}

func (c *Controller) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if len(c.APISecret) > 0 && r.Header.Get(atlantisTokenHeader) != string(c.APISecret) {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	c.handler.ServeHTTP(w, r)
}
