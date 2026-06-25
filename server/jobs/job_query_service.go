// Copyright 2026 The Atlantis Authors
// SPDX-License-Identifier: Apache-2.0

package jobs

import (
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	StaleReasonSupersededByNewerJob = "superseded_by_newer_job"
)

type JobQueryService struct {
	OutputHandler ProjectCommandOutputHandler
}

type JobFilters struct {
	RepoFullName string
	PullNum      int
	Command      string
	ProjectName  string
	Path         string
	Workspace    string
	JobID        string
	LatestOnly   bool
}

type Job struct {
	JobID             string    `json:"job_id"`
	RepoFullName      string    `json:"repo_full_name"`
	PullNum           int       `json:"pull_num"`
	Command           string    `json:"command"`
	ProjectName       string    `json:"project_name"`
	Path              string    `json:"path"`
	Workspace         string    `json:"workspace"`
	HeadCommit        string    `json:"head_commit,omitempty"`
	CreatedAt         time.Time `json:"created_at"`
	OperationComplete bool      `json:"operation_complete"`
	IsLatest          bool      `json:"is_latest"`
	StaleReasons      []string  `json:"stale_reasons"`
}

type JobLog struct {
	Job
	Lines []string `json:"lines"`
	Log   string   `json:"log"`
}

func (s JobQueryService) ListPRJobs(filters JobFilters) []Job {
	jobs := s.collectJobs(filters)
	markLatest(jobs)
	// Staleness is computed by markLatest over the full set of sibling jobs, so
	// the JobID filter must be applied here rather than in collectJobs. Filtering
	// it earlier would leave a single job in the slice and make markLatest always
	// mark it latest, even when it has been superseded.
	if filters.JobID != "" {
		jobs = filterByJobID(jobs, filters.JobID)
	}
	if filters.LatestOnly {
		jobs = filterLatest(jobs)
	}
	sortJobs(jobs)
	return jobs
}

func (s JobQueryService) GetPRLogs(filters JobFilters) []JobLog {
	jobs := s.ListPRJobs(filters)
	logs := make([]JobLog, 0, len(jobs))
	for _, job := range jobs {
		buffer, ok := s.OutputHandler.GetProjectOutputBuffer(job.JobID)
		if !ok {
			continue
		}
		job.OperationComplete = buffer.OperationComplete
		logs = append(logs, JobLog{
			Job:   job,
			Lines: buffer.Buffer,
			Log:   strings.Join(buffer.Buffer, "\n"),
		})
	}
	return logs
}

func (s JobQueryService) collectJobs(filters JobFilters) []Job {
	if s.OutputHandler == nil {
		return nil
	}
	var results []Job
	for _, mapping := range s.OutputHandler.GetPullToJobMapping() {
		pull := mapping.Pull
		if filters.RepoFullName != "" && pull.RepoFullName != filters.RepoFullName {
			continue
		}
		if filters.PullNum != 0 && pull.PullNum != filters.PullNum {
			continue
		}
		if filters.ProjectName != "" && pull.ProjectName != filters.ProjectName {
			continue
		}
		if filters.Path != "" && pull.Path != filters.Path {
			continue
		}
		if filters.Workspace != "" && pull.Workspace != filters.Workspace {
			continue
		}

		for _, info := range mapping.JobIDInfos {
			if filters.Command != "" && info.JobStep != filters.Command {
				continue
			}
			buffer, ok := s.OutputHandler.GetProjectOutputBuffer(info.JobID)
			results = append(results, Job{
				JobID:             info.JobID,
				RepoFullName:      pull.RepoFullName,
				PullNum:           pull.PullNum,
				Command:           info.JobStep,
				ProjectName:       pull.ProjectName,
				Path:              pull.Path,
				Workspace:         pull.Workspace,
				HeadCommit:        info.HeadCommit,
				CreatedAt:         info.Time,
				OperationComplete: ok && buffer.OperationComplete,
			})
		}
	}
	return results
}

func markLatest(jobs []Job) {
	latestByKey := map[string]int{}
	for i, job := range jobs {
		key := jobIdentity(job)
		latestIdx, ok := latestByKey[key]
		if !ok || job.CreatedAt.After(jobs[latestIdx].CreatedAt) {
			latestByKey[key] = i
		}
	}
	for i := range jobs {
		if latestByKey[jobIdentity(jobs[i])] == i {
			jobs[i].IsLatest = true
			jobs[i].StaleReasons = []string{}
			continue
		}
		jobs[i].IsLatest = false
		jobs[i].StaleReasons = []string{StaleReasonSupersededByNewerJob}
	}
}

func filterLatest(jobs []Job) []Job {
	latest := make([]Job, 0, len(jobs))
	for _, job := range jobs {
		if job.IsLatest {
			latest = append(latest, job)
		}
	}
	return latest
}

func filterByJobID(jobs []Job, jobID string) []Job {
	matching := make([]Job, 0, 1)
	for _, job := range jobs {
		if job.JobID == jobID {
			matching = append(matching, job)
		}
	}
	return matching
}

func sortJobs(jobs []Job) {
	sort.SliceStable(jobs, func(i, j int) bool {
		if jobs[i].RepoFullName != jobs[j].RepoFullName {
			return jobs[i].RepoFullName < jobs[j].RepoFullName
		}
		if jobs[i].PullNum != jobs[j].PullNum {
			return jobs[i].PullNum < jobs[j].PullNum
		}
		if jobs[i].ProjectName != jobs[j].ProjectName {
			return jobs[i].ProjectName < jobs[j].ProjectName
		}
		if jobs[i].Path != jobs[j].Path {
			return jobs[i].Path < jobs[j].Path
		}
		if jobs[i].Workspace != jobs[j].Workspace {
			return jobs[i].Workspace < jobs[j].Workspace
		}
		if jobs[i].Command != jobs[j].Command {
			return jobs[i].Command < jobs[j].Command
		}
		return jobs[i].CreatedAt.After(jobs[j].CreatedAt)
	})
}

func jobIdentity(job Job) string {
	return strings.Join([]string{
		job.RepoFullName,
		strconv.Itoa(job.PullNum),
		job.Command,
		job.ProjectName,
		job.Path,
		job.Workspace,
	}, "\x00")
}
