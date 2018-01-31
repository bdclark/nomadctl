package deploy

import (
	"bytes"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/bdclark/nomadctl/logging"
	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/jobspec"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/pkg/errors"
)

const (
	// RedeployMetaKey ...
	RedeployMetaKey = "nomadctl_redeploy"
)

// Deployment is the internal representation of a Nomadctl deployment
type Deployment struct {
	client           *api.Client // the Nomad API client
	job              *api.Job    // the Nomad job spec
	enforceIndex     bool        // job will only be registered if jobModifyIndex matches the current job's index
	jobModifyIndex   uint64      //  index to enforce job state
	useTemplateCount bool        // whether the job will get its group counts from template rather than remote job
	autoPromote      bool        // whether a canary job should be automatically promoted
	deploymentID     string      // the nomad deployment id
	idLen            int         // how long to print ids
	needsPromotion   bool        // whether the running deployment requires a promotion to complete
	promoted         bool        // whether a job needing promotion has been promoted
	isRedeploy       bool        // whether this deployment is actually a re-deployment
}

// NewDeploymentInput represents the input for a new deployment
type NewDeploymentInput struct {
	Job              *api.Job // the Nomad Job to deploy
	Jobspec          *[]byte  // the nomad job spec to be converted to a Nomad Job
	EnforceIndex     bool     // job will only be registered if JobModifyIndex matches the current job's index
	JobModifyIndex   uint64   // index to enforce job state
	UseTemplateCount bool     // whether the job will get its group counts from template rather than remote job
	AutoPromote      bool     // whether a canary job should be automatically promoted
	Verbose          bool     // whether long UUIDs should be logged
}

// RedeploymentInput represents the input for a redeployment
type RedeploymentInput struct {
	JobName        string
	TaskGroupNames []string
	AutoPromote    bool
	Verbose        bool
}

// NewDeployment generates a new deployment
func NewDeployment(i *NewDeploymentInput) (d *Deployment, err error) {
	client, err := api.NewClient(api.DefaultConfig())
	if err != nil {
		return
	}

	d = &Deployment{
		client:           client,
		job:              i.Job,
		enforceIndex:     i.EnforceIndex,
		jobModifyIndex:   i.JobModifyIndex,
		useTemplateCount: i.UseTemplateCount,
		autoPromote:      i.AutoPromote,
	}

	d.setIDLength(i.Verbose)

	if i.Jobspec != nil && len(*i.Jobspec) > 0 {
		if i.Job != nil {
			return nil, fmt.Errorf("cannot specify Job and Jobspec")
		}

		r := bytes.NewReader(*i.Jobspec)
		if job, err := jobspec.Parse(r); err == nil {
			d.job = job
		} else {
			return nil, errors.Wrap(err, "jobspec parse")
		}
	}

	return
}

// ReDeploy redeploys an existing remote job
func ReDeploy(i *RedeploymentInput) (bool, error) {
	client, err := api.NewClient(api.DefaultConfig())
	if err != nil {
		return false, err
	}

	// ensure job exists remotely
	job, _, err := client.Jobs().Info(i.JobName, nil)
	if err != nil {
		if strings.Contains(err.Error(), "404") {
			return false, fmt.Errorf("job \"%s\" not found on server", i.JobName)
		}
		return false, err
	}

	// use current time for value in meta key
	t := time.Now().Format(time.RFC3339)

	for _, tg := range job.TaskGroups {
		if tg.Meta == nil {
			tg.Meta = make(map[string]string)
		}
		// if no group(s) specified, update meta key in each
		if len(i.TaskGroupNames) == 0 {
			tg.Meta[RedeployMetaKey] = t
		} else {
			// group(s) specified, only update if matching
			for _, g := range i.TaskGroupNames {
				if g == *tg.Name {
					tg.Meta[RedeployMetaKey] = t
				}
			}
		}
	}

	// create a deployment
	var d Deployment
	d.isRedeploy = true
	d.client = client
	d.job = job
	d.setIDLength(i.Verbose)
	d.autoPromote = i.AutoPromote

	// deploy it
	return d.Deploy()
}

// Deploy performs a deployment
func (d *Deployment) Deploy() (success bool, err error) {
	// validate the job first
	resp, _, err := d.client.Jobs().Validate(d.job, nil)
	if err != nil {
		return false, errors.Wrap(err, "validation failed")
	} else if resp.Error != "" {
		return false, fmt.Errorf("validation failed: %s", resp.Error)
	}

	// optionally update task group counts to reflect what's currently deployed
	if !d.useTemplateCount {
		if err = d.updateGroupCounts(); err != nil {
			return false, err
		}
	}

	// update the redeployment meta to match remote job
	if !d.isRedeploy {
		if err = d.updateRedeployMeta(); err != nil {
			return false, err
		}
	}

	// check we have some task group counts to actually deploy
	// (as long as it's not a system job - they don't define count)
	if d.job.Type == nil || *d.job.Type != structs.JobTypeSystem {
		count := 0
		for _, g := range d.job.TaskGroups {
			count += *g.Count
		}
		if count == 0 {
			return false, fmt.Errorf("all TaskGroups have a count of 0, nothing to do")
		}
	}

	// register the job with Nomad
	logging.Info("registering job \"%s\"", *d.job.Name)
	opts := &api.RegisterOptions{}
	if d.enforceIndex {
		opts.EnforceIndex = true
		opts.ModifyIndex = d.jobModifyIndex
	}
	registerResp, _, err := d.client.Jobs().RegisterOpts(d.job, opts, nil)
	if err != nil {
		return false, errors.Wrap(err, "job register failed")
	}
	evalID := registerResp.EvalID

	// check the evaluation for failures on jobs that have eval IDs
	if evalID != "" {
		if ok, err := d.monitorEvalStatus(evalID); err != nil {
			return false, err
		} else if !ok {
			return false, fmt.Errorf("abandoning deployment due to failed/blocked evaluation(s), manual intervention required")
		}
	}

	switch *d.job.Type {
	case structs.JobTypeService:
		if eval, _, err := d.client.Evaluations().Info(evalID, nil); err == nil {
			d.deploymentID = eval.DeploymentID
		} else {
			return false, err
		}

		if d.deploymentID == "" {
			logging.Info("no deployment ID found, monitoring for running status")
			return d.waitJobRunning()
		}

		logging.Info("monitoring deployment \"%s\"", limit(d.deploymentID, d.idLen))
		success, err = d.monitorDeployment()

		if !success {
			err = fmt.Errorf("abandoning unsuccessful deployment, manual intervention required")
		}

	case structs.JobTypeBatch:
		// batch jobs don't have eval IDs so just check if running
		success, err = d.waitJobRunning()

	default:
		success = true
	}

	return
}

// updateGroupCounts updates the job's task group counts with those
// found in a remote job with the same name
func (d *Deployment) updateGroupCounts() error {
	remoteJob, _, err := d.client.Jobs().Info(*d.job.Name, nil)

	if err != nil {
		if strings.Contains(err.Error(), "404") {
			return nil // job doesn't exist
		}
		return err
	}

	logging.Debug("attempting to update group counts for job \"%s\" from remote job", *d.job.Name)

	for _, rtg := range remoteJob.TaskGroups {
		for _, tg := range d.job.TaskGroups {
			if rtg.Name == tg.Name {
				if tg.Count != rtg.Count {
					logging.Info("updating count to match running job \"%s\", group \"%s\" from %v to %v",
						d.job.Name, tg.Name, tg.Count, rtg.Count)
					tg.Count = rtg.Count
				}
				break
			}
		}
	}
	return nil
}

// updateRedeployMeta updates the job's task group redeployment related
// meta with the meta found in a remote job with the same name
func (d *Deployment) updateRedeployMeta() error {
	remoteJob, _, err := d.client.Jobs().Info(*d.job.Name, nil)

	if err != nil {
		if strings.Contains(err.Error(), "404") {
			return nil // job doesn't exist
		}
		return err
	}

	logging.Debug("attempting to update redeploy meta for job \"%s\" from remote job", *d.job.Name)

	for _, rtg := range remoteJob.TaskGroups {
		for _, tg := range d.job.TaskGroups {
			if *rtg.Name == *tg.Name {

				if val, ok := rtg.Meta[RedeployMetaKey]; ok {
					logging.Debug("updating `%s` meta key to match remote job \"%s\", group \"%s\"",
						RedeployMetaKey, *d.job.Name, *tg.Name)
					if tg.Meta == nil {
						tg.Meta = make(map[string]string)
					}
					tg.Meta[RedeployMetaKey] = val
				}
				break
			}
		}
	}

	return nil
}

// monitorEvalStatus waits for an evaluation to complete, and returns
// true if all allocations were placed, false if not.
func (d *Deployment) monitorEvalStatus(id string) (bool, error) {

	q := &api.QueryOptions{
		WaitIndex: 0,
		WaitTime:  time.Duration(10 * time.Second),
	}

	for {
		eval, meta, err := d.client.Evaluations().Info(id, q)
		if err != nil {
			return false, errors.Wrapf(err, "failed to get eval info")
		}

		if meta.LastIndex <= q.WaitIndex {
			continue
		}

		switch eval.Status {
		case structs.EvalStatusComplete, structs.EvalStatusFailed, structs.EvalStatusCancelled:

			if len(eval.FailedTGAllocs) == 0 {
				logging.Info("evaluation \"%s\" finished with status \"%s\"", limit(id, d.idLen), eval.Status)
				return true, nil
			}

			logMsg := []string{fmt.Sprintf("evaluation \"%s\" finished with status \"%s\" but failed to place all allocations", limit(eval.ID, d.idLen), eval.Status)}
			for tg, metrics := range eval.FailedTGAllocs {
				logMsg = append(logMsg, fmt.Sprintf("  task group %q failed to place %d allocation(s):", tg, metrics.CoalescedFailures+1))
				logMsg = append(logMsg, formatAllocMetrics(metrics, false, strings.Repeat(" ", 4))...)
			}
			logging.Error(strings.Join(logMsg, "\n"))

			if eval.BlockedEval != "" {
				logging.Error("blocked evaluation %q waiting for additional capacity to place remainder", limit(eval.BlockedEval, d.idLen))
			}
			return false, nil

		default:
			logging.Info("evaluation \"%s\" has status \"%s\"", limit(id, d.idLen), eval.Status)
			q.WaitIndex = meta.LastIndex
			continue
		}
	}
}

// monitorDeployment waits for the Nomad deployment to complete,
// and returns true if it completed successfully, false if not.
func (d *Deployment) monitorDeployment() (bool, error) {
	t := time.Now()
	q := &api.QueryOptions{
		WaitIndex: 0,
		WaitTime:  time.Duration(10 * time.Second),
	}

	for {
		dep, meta, err := d.client.Deployments().Info(d.deploymentID, q)
		if err != nil {
			return false, errors.Wrap(err, "failed to get deployment")
		}

		if meta.LastIndex <= q.WaitIndex {
			continue
		}
		q.WaitIndex = meta.LastIndex

		switch dep.Status {
		case structs.DeploymentStatusSuccessful:
			logging.Info("deployment \"%s\" completed with status \"%s\"", limit(dep.ID, d.idLen), dep.Status)
			return true, nil

		case structs.DeploymentStatusRunning:
			logging.Debug("deployment \"%s\" has been running for %.1fs", limit(d.deploymentID, d.idLen), time.Since(t).Seconds())

			// we already auto-promoted, and are waiting on the deployment to complete
			if d.promoted {
				continue
			}

			var healthy int

			for name, state := range dep.TaskGroups {
				logging.Debug("group %s: %d desired canaries, %d healthy allocs, %d desired total", name, state.DesiredCanaries, state.HealthyAllocs, state.DesiredTotal)

				switch {
				case state.DesiredCanaries == 0 && state.HealthyAllocs == state.DesiredTotal:
					healthy++

				case state.DesiredCanaries > 0 && state.HealthyAllocs == state.DesiredCanaries:
					healthy++
					d.needsPromotion = true

				case state.UnhealthyAllocs > 0:
					logging.Error("group \"%s\" has %d unhealthy allocations", name, state.UnhealthyAllocs)
				}
			}

			// all desired allocs are healthy, requires promotion to complete
			if healthy == len(dep.TaskGroups) && d.needsPromotion {
				if d.autoPromote {
					logging.Info("deployment \"%s\" has healthy canaries - attempting auto-promotion", limit(d.deploymentID, d.idLen))
					if _, _, err := d.client.Deployments().PromoteAll(d.deploymentID, nil); err != nil {
						return false, errors.Wrap(err, "promotion failed")
					}
					d.promoted = true
				} else {
					logging.Info("deployment \"%s\" has healthy canaries but must be manually promoted", limit(d.deploymentID, d.idLen))
					return true, nil
				}
			}

			continue

		default:
			logging.Error("deployment \"%s\" has status \"%s\"", limit(dep.ID, d.idLen), dep.Status)
			d.logFailedDeployment()
			return false, nil
		}
	}
}

func (d *Deployment) logFailedDeployment() {
	allocs, _, err := d.client.Deployments().Allocations(d.deploymentID, nil)
	if err != nil {
		logging.Error("failed to get allocations for deployment \"%s\": %v", limit(d.deploymentID, d.idLen), err)
		return
	}

	var ids []string

	for _, alloc := range allocs {
		for _, ts := range alloc.TaskStates {
			if ts.State != structs.TaskStateRunning {
				ids = append(ids, alloc.ID)
				break
			}
		}
	}

	var wg sync.WaitGroup
	wg.Add(+len(ids))

	for _, id := range ids {
		go d.logFailedAllocEvents(id, &wg)
	}

	wg.Wait()
}

func (d *Deployment) logFailedAllocEvents(allocID string, wg *sync.WaitGroup) {
	defer wg.Done()

	alloc, _, err := d.client.Allocations().Info(allocID, nil)
	if err != nil {
		logging.Error("failed to get allocation \"%s\": %v", limit(allocID, d.idLen), err)
		return
	}

	var logMsg []string
	logMsg = append(logMsg, fmt.Sprintf("allocation \"%s\" failed:", limit(allocID, d.idLen)))
	for name, task := range alloc.TaskStates {
		logMsg = append(logMsg, fmt.Sprintf("  task \"%s\" is %s with the following events:", name, task.State))
		for _, event := range task.Events {
			if desc := buildTaskEventMessage(event); desc != "" {
				logMsg = append(logMsg, fmt.Sprintf("    * %s - %s",
					event.Type, strings.TrimSpace(desc)))
			}
		}
	}
	logging.Error(strings.Join(logMsg, "\n"))
}

// buildTaskEventMessage returns a message based
// on the details of a Nomad allocation TaskEvent
// (see https://github.com/hashicorp/nomad/blob/master/command/alloc_status.go)
func buildTaskEventMessage(event *api.TaskEvent) (desc string) {
	switch event.Type {
	case api.TaskSetup:
		desc = event.Message
	case api.TaskStarted:
		desc = "Task started by client"
	case api.TaskReceived:
		desc = "Task received by client"
	case api.TaskFailedValidation:
		if event.ValidationError != "" {
			desc = event.ValidationError
		} else {
			desc = "Validation of task failed"
		}
	case api.TaskSetupFailure:
		if event.SetupError != "" {
			desc = event.SetupError
		} else {
			desc = "Task setup failed"
		}
	case api.TaskDriverFailure:
		if event.DriverError != "" {
			desc = event.DriverError
		} else {
			desc = "Failed to start task"
		}
	case api.TaskDownloadingArtifacts:
		desc = "Client is downloading artifacts"
	case api.TaskArtifactDownloadFailed:
		if event.DownloadError != "" {
			desc = event.DownloadError
		} else {
			desc = "Failed to download artifacts"
		}
	case api.TaskKilling:
		if event.KillReason != "" {
			desc = fmt.Sprintf("Killing task: %v", event.KillReason)
		} else if event.KillTimeout != 0 {
			desc = fmt.Sprintf("Sent interrupt. Waiting %v before force killing", event.KillTimeout)
		} else {
			desc = "Sent interrupt"
		}
	case api.TaskKilled:
		if event.KillError != "" {
			desc = event.KillError
		} else {
			desc = "Task successfully killed"
		}
	case api.TaskTerminated:
		var parts []string
		parts = append(parts, fmt.Sprintf("Exit Code: %d", event.ExitCode))

		if event.Signal != 0 {
			parts = append(parts, fmt.Sprintf("Signal: %d", event.Signal))
		}

		if event.Message != "" {
			parts = append(parts, fmt.Sprintf("Exit Message: %q", event.Message))
		}
		desc = strings.Join(parts, ", ")
	case api.TaskRestarting:
		in := fmt.Sprintf("Task restarting in %v", time.Duration(event.StartDelay))
		if event.RestartReason != "" {
			desc = fmt.Sprintf("%s - %s", event.RestartReason, in)
		} else {
			desc = in
		}
	case api.TaskNotRestarting:
		if event.RestartReason != "" {
			desc = event.RestartReason
		} else {
			desc = "Task exceeded restart policy"
		}
	case api.TaskSiblingFailed:
		if event.FailedSibling != "" {
			desc = fmt.Sprintf("Task's sibling %q failed", event.FailedSibling)
		} else {
			desc = "Task's sibling failed"
		}
	case api.TaskSignaling:
		sig := event.TaskSignal
		reason := event.TaskSignalReason

		if sig == "" && reason == "" {
			desc = "Task being sent a signal"
		} else if sig == "" {
			desc = reason
		} else if reason == "" {
			desc = fmt.Sprintf("Task being sent signal %v", sig)
		} else {
			desc = fmt.Sprintf("Task being sent signal %v: %v", sig, reason)
		}
	case api.TaskRestartSignal:
		if event.RestartReason != "" {
			desc = event.RestartReason
		} else {
			desc = "Task signaled to restart"
		}
	case api.TaskDriverMessage:
		desc = event.DriverMessage
	case api.TaskLeaderDead:
		desc = "Leader Task in Group dead"
	default:
		desc = event.Message
	}

	return
}

// waitJobRunning checks the status of a job to ensure it's running.
// This is helpful with batch jobs since there's really no other way
// to determine deployment success.
func (d *Deployment) waitJobRunning() (bool, error) {
	q := &api.QueryOptions{
		WaitIndex: 0,
		WaitTime:  time.Duration(5 * time.Second),
	}

	for {
		job, meta, err := d.client.Jobs().Info(*d.job.Name, q)
		if err != nil {
			return false, errors.Wrapf(err, "failed to get job info")
		}

		if meta.LastIndex <= q.WaitIndex {
			continue
		}
		q.WaitIndex = meta.LastIndex

		switch *job.Status {
		case structs.JobStatusRunning:
			logging.Info("job \"%s\" has status \"%s\"", *job.Name, *job.Status)
			return true, nil
		case structs.JobStatusPending:
			logging.Debug("job \"%s\" has status \"%s\"", *job.Name, *job.Status)
			continue
		default:
			logging.Error("job \"%s\" has status \"%s\"", *job.Name, *job.Status)
			return false, nil
		}
	}
}

// formatAllocMetrics returns a slice of log message lines
// based on the state of a Nomad AllocationMetric
func formatAllocMetrics(metrics *api.AllocationMetric, scores bool, prefix string) (out []string) {
	// Print a helpful message if we have an eligibility problem
	if metrics.NodesEvaluated == 0 {
		out = append(out, fmt.Sprintf("%s* no nodes were eligible for evaluation", prefix))
	}

	// Print a helpful message if the user has asked for a DC that has no
	// available nodes.
	for dc, available := range metrics.NodesAvailable {
		if available == 0 {
			out = append(out, fmt.Sprintf("%s* no nodes are available in datacenter %q", prefix, dc))
		}
	}

	// Print filter info
	for class, num := range metrics.ClassFiltered {
		out = append(out, fmt.Sprintf("%s* class %q filtered %d nodes", prefix, class, num))
	}
	for cs, num := range metrics.ConstraintFiltered {
		out = append(out, fmt.Sprintf("%s* constraint %q filtered %d nodes", prefix, cs, num))
	}

	// Print exhaustion info
	if ne := metrics.NodesExhausted; ne > 0 {
		out = append(out, fmt.Sprintf("%s* resources exhausted on %d nodes", prefix, ne))
	}
	for class, num := range metrics.ClassExhausted {
		out = append(out, fmt.Sprintf("%s* class %q exhausted on %d nodes", prefix, class, num))
	}
	for dim, num := range metrics.DimensionExhausted {
		out = append(out, fmt.Sprintf("%s* dimension %q exhausted on %d nodes", prefix, dim, num))
	}

	// Print quota info
	for _, dim := range metrics.QuotaExhausted {
		out = append(out, fmt.Sprintf("%s* quota limit hit %q", prefix, dim))
	}

	// Print scores
	if scores {
		for name, score := range metrics.Scores {
			out = append(out, fmt.Sprintf("%s* score %q = %f", prefix, name, score))
		}
	}
	return
}

// limits the length of the string.
func limit(s string, length int) string {
	if len(s) < length {
		return s
	}

	return s[:length]
}

// setIDLength sets the length of UUIDs in log messages
func (d *Deployment) setIDLength(verbose bool) {
	if verbose {
		d.idLen = 36
	} else {
		d.idLen = 8
	}
}
