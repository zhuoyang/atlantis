// Copyright 2026 The Atlantis Authors
// SPDX-License-Identifier: Apache-2.0

package jobs_test

import (
	"testing"
	"time"

	"github.com/runatlantis/atlantis/server/events/command"
	"github.com/runatlantis/atlantis/server/events/models"
	"github.com/runatlantis/atlantis/server/jobs"
	"github.com/stretchr/testify/assert"
)

type queryOutputHandler struct {
	mappings []jobs.PullInfoWithJobIDs
	buffers  map[string]jobs.OutputBuffer
}

func (h queryOutputHandler) Send(command.ProjectContext, string, bool) {}

func (h queryOutputHandler) SendWorkflowHook(models.WorkflowHookCommandContext, string, bool) {}

func (h queryOutputHandler) Register(string, chan string) {}

func (h queryOutputHandler) Deregister(string, chan string) {}

func (h queryOutputHandler) IsKeyExists(string) bool { return false }

func (h queryOutputHandler) Handle() {}

func (h queryOutputHandler) CleanUp(jobs.PullInfo) {}

func (h queryOutputHandler) GetPullToJobMapping() []jobs.PullInfoWithJobIDs { return h.mappings }

func (h queryOutputHandler) GetProjectOutputBuffer(jobID string) (jobs.OutputBuffer, bool) {
	buffer, ok := h.buffers[jobID]
	return buffer, ok
}

func TestJobQueryServiceListPRJobsMarksLatest(t *testing.T) {
	baseTime := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	service := jobs.JobQueryService{OutputHandler: queryOutputHandler{
		mappings: []jobs.PullInfoWithJobIDs{
			{
				Pull: jobs.PullInfo{
					PullNum:      12,
					RepoFullName: "owner/repo",
					ProjectName:  "prod",
					Path:         "infra/prod",
					Workspace:    "default",
				},
				JobIDInfos: []jobs.JobIDInfo{
					{JobID: "old", JobStep: "plan", Time: baseTime, HeadCommit: "old-sha"},
					{JobID: "new", JobStep: "plan", Time: baseTime.Add(time.Minute), HeadCommit: "new-sha"},
					{JobID: "apply", JobStep: "apply", Time: baseTime.Add(2 * time.Minute), HeadCommit: "new-sha"},
				},
			},
		},
		buffers: map[string]jobs.OutputBuffer{
			"old":   {OperationComplete: true},
			"new":   {OperationComplete: false},
			"apply": {OperationComplete: true},
		},
	}}

	got := service.ListPRJobs(jobs.JobFilters{RepoFullName: "owner/repo", PullNum: 12, Command: "plan"})

	assert.Len(t, got, 2)
	assert.Equal(t, "new", got[0].JobID)
	assert.True(t, got[0].IsLatest)
	assert.False(t, got[0].OperationComplete)
	assert.Equal(t, "old", got[1].JobID)
	assert.False(t, got[1].IsLatest)
	assert.Equal(t, []string{jobs.StaleReasonSupersededByNewerJob}, got[1].StaleReasons)

	latest := service.ListPRJobs(jobs.JobFilters{RepoFullName: "owner/repo", PullNum: 12, Command: "plan", LatestOnly: true})
	assert.Len(t, latest, 1)
	assert.Equal(t, "new", latest[0].JobID)
}

func TestJobQueryServiceGetPRLogs(t *testing.T) {
	service := jobs.JobQueryService{OutputHandler: queryOutputHandler{
		mappings: []jobs.PullInfoWithJobIDs{
			{
				Pull: jobs.PullInfo{PullNum: 12, RepoFullName: "owner/repo", ProjectName: "prod", Path: "infra/prod", Workspace: "default"},
				JobIDInfos: []jobs.JobIDInfo{
					{JobID: "job", JobStep: "apply", Time: time.Now()},
				},
			},
		},
		buffers: map[string]jobs.OutputBuffer{
			"job": {OperationComplete: true, Buffer: []string{"line one", "line two"}},
		},
	}}

	got := service.GetPRLogs(jobs.JobFilters{JobID: "job"})

	assert.Len(t, got, 1)
	assert.Equal(t, []string{"line one", "line two"}, got[0].Lines)
	assert.Equal(t, "line one\nline two", got[0].Log)
	assert.True(t, got[0].OperationComplete)
}

// Fetching a superseded job by its JobID must still report it as stale, because
// staleness is computed against the job's siblings, not just the filtered result.
func TestJobQueryServiceStaleJobByJobID(t *testing.T) {
	baseTime := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	service := jobs.JobQueryService{OutputHandler: queryOutputHandler{
		mappings: []jobs.PullInfoWithJobIDs{
			{
				Pull: jobs.PullInfo{PullNum: 12, RepoFullName: "owner/repo", ProjectName: "prod", Path: "infra/prod", Workspace: "default"},
				JobIDInfos: []jobs.JobIDInfo{
					{JobID: "old", JobStep: "plan", Time: baseTime, HeadCommit: "old-sha"},
					{JobID: "new", JobStep: "plan", Time: baseTime.Add(time.Minute), HeadCommit: "new-sha"},
				},
			},
		},
		buffers: map[string]jobs.OutputBuffer{
			"old": {OperationComplete: true, Buffer: []string{"old output"}},
			"new": {OperationComplete: false, Buffer: []string{"new output"}},
		},
	}}

	list := service.ListPRJobs(jobs.JobFilters{JobID: "old"})
	assert.Len(t, list, 1)
	assert.Equal(t, "old", list[0].JobID)
	assert.False(t, list[0].IsLatest)
	assert.Equal(t, []string{jobs.StaleReasonSupersededByNewerJob}, list[0].StaleReasons)

	logs := service.GetPRLogs(jobs.JobFilters{JobID: "old"})
	assert.Len(t, logs, 1)
	assert.Equal(t, "old", logs[0].JobID)
	assert.False(t, logs[0].IsLatest)
	assert.Equal(t, []string{jobs.StaleReasonSupersededByNewerJob}, logs[0].StaleReasons)
}
