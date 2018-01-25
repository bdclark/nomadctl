package deploy

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/scheduler"
	"github.com/mitchellh/colorstring"
	"github.com/pkg/errors"
	"golang.org/x/crypto/ssh/terminal"
)

/*
	The functions below related to output formatting of a Nomad job plan were borrowed
	from the Nomad source at https://github.com/hashicorp/nomad/blob/master/command/plan.go
*/

// Plan executes a Nomad plan, writes the output to standard out, and
// returns whether any allocations will be created/destroyed
func (d *Deployment) Plan(verbose, diff, noColor bool) (bool, error) {
	// force the region to be that of the job.
	if r := d.job.Region; r != nil {
		d.client.SetRegion(*r)
	}

	// force the namespace to be that of the job.
	if n := d.job.Namespace; n != nil {
		d.client.SetNamespace(*n)
	}

	resp, _, err := d.client.Jobs().Plan(d.job, true, nil)
	if err != nil {
		return false, errors.Wrap(err, "plan failed")
	}
	d.jobModifyIndex = resp.JobModifyIndex

	colorize := &colorstring.Colorize{
		Colors:  colorstring.DefaultColors,
		Disable: noColor || !terminal.IsTerminal(int(os.Stdout.Fd())),
		Reset:   true,
	}

	// print the job diff
	if diff {
		fmt.Println(fmt.Sprintf("%s\n", colorize.Color(strings.TrimSpace(formatJobDiff(resp.Diff, verbose)))))
	}

	// print the scheduler dry-run output
	fmt.Println(colorize.Color("[bold]Scheduler dry-run:[reset]"))
	fmt.Println(colorize.Color(formatDryRun(resp, d.job)))
	fmt.Println()

	// print any warnings
	if resp.Warnings != "" {
		fmt.Println(colorize.Color(fmt.Sprintf("[bold][yellow]Job Warnings:\n%s[reset]\n", resp.Warnings)))
	}

	// print job index info
	fmt.Println(colorize.Color(fmt.Sprintf("[reset][bold]Job Modify Index: %d[reset]", resp.JobModifyIndex)))

	// Check for allocation changes and return accordingly
	for _, d := range resp.Annotations.DesiredTGUpdates {
		if d.Stop+d.Place+d.Migrate+d.DestructiveUpdate+d.Canary > 0 {
			return true, nil
		}
	}
	return false, nil
}

// formatDryRun produces a string explaining the results of the dry run.
func formatDryRun(resp *api.JobPlanResponse, job *api.Job) string {
	var rolling *api.Evaluation
	for _, eval := range resp.CreatedEvals {
		if eval.TriggeredBy == "rolling-update" {
			rolling = eval
		}
	}

	var out string
	if len(resp.FailedTGAllocs) == 0 {
		out = "[bold][green]- All tasks successfully allocated.[reset]\n"
	} else {
		// Change the output depending on if we are a system job or not
		if job.Type != nil && *job.Type == "system" {
			out = "[bold][yellow]- WARNING: Failed to place allocations on all nodes.[reset]\n"
		} else {
			out = "[bold][yellow]- WARNING: Failed to place all allocations.[reset]\n"
		}
		sorted := sortedTaskGroupFromMetrics(resp.FailedTGAllocs)
		for _, tg := range sorted {
			metrics := resp.FailedTGAllocs[tg]

			noun := "allocation"
			if metrics.CoalescedFailures > 0 {
				noun += "s"
			}
			out += fmt.Sprintf("%s[yellow]Task Group %q (failed to place %d %s):\n[reset]", strings.Repeat(" ", 2), tg, metrics.CoalescedFailures+1, noun)
			out += fmt.Sprintf("[yellow]%s[reset]\n\n", formatAllocMetrics(metrics, false, strings.Repeat(" ", 4)))
		}
		if rolling == nil {
			out = strings.TrimSuffix(out, "\n")
		}
	}

	if rolling != nil {
		out += fmt.Sprintf("[green]- Rolling update, next evaluation will be in %s.\n", rolling.Wait)
	}

	if next := resp.NextPeriodicLaunch; !next.IsZero() && !job.IsParameterized() {
		loc, err := job.Periodic.GetLocation()
		if err != nil {
			out += fmt.Sprintf("[yellow]- Invalid time zone: %v", err)
		} else {
			now := time.Now().In(loc)
			out += fmt.Sprintf("[green]- If submitted now, next periodic launch would be at %s (%s from now).\n",
				formatTime(next), formatTimeDifference(now, next, time.Second))
		}
	}

	out = strings.TrimSuffix(out, "\n")
	return out
}

func sortedTaskGroupFromMetrics(groups map[string]*api.AllocationMetric) []string {
	tgs := make([]string, 0, len(groups))
	for tg := range groups {
		tgs = append(tgs, tg)
	}
	sort.Strings(tgs)
	return tgs
}

// formatJobDiff produces an annoted diff of the job. If verbose mode is
// set, added or deleted task groups and tasks are expanded.
func formatJobDiff(job *api.JobDiff, verbose bool) string {
	marker, _ := getDiffString(job.Type)
	out := fmt.Sprintf("%s[bold]Job: %q\n", marker, job.ID)

	// Determine the longest markers and fields so that the output can be
	// properly aligned.
	longestField, longestMarker := getLongestPrefixes(job.Fields, job.Objects)
	for _, tg := range job.TaskGroups {
		if _, l := getDiffString(tg.Type); l > longestMarker {
			longestMarker = l
		}
	}

	// Only show the job's field and object diffs if the job is edited or
	// verbose mode is set.
	if job.Type == "Edited" || verbose {
		fo := alignedFieldAndObjects(job.Fields, job.Objects, 0, longestField, longestMarker)
		out += fo
		if len(fo) > 0 {
			out += "\n"
		}
	}

	// Print the task groups
	for _, tg := range job.TaskGroups {
		_, mLength := getDiffString(tg.Type)
		keyPrefix := longestMarker - mLength
		out += fmt.Sprintf("%s\n", formatTaskGroupDiff(tg, keyPrefix, verbose))
	}

	return out
}

// formatTaskGroupDiff produces an annotated diff of a task group. If the
// verbose field is set, the task groups fields and objects are expanded even if
// the full object is an addition or removal. tgPrefix is the number of spaces to prefix
// the output of the task group.
func formatTaskGroupDiff(tg *api.TaskGroupDiff, tgPrefix int, verbose bool) string {
	marker, _ := getDiffString(tg.Type)
	out := fmt.Sprintf("%s%s[bold]Task Group: %q[reset]", marker, strings.Repeat(" ", tgPrefix), tg.Name)

	// Append the updates and colorize them
	if l := len(tg.Updates); l > 0 {
		order := make([]string, 0, l)
		for updateType := range tg.Updates {
			order = append(order, updateType)
		}

		sort.Strings(order)
		updates := make([]string, 0, l)
		for _, updateType := range order {
			count := tg.Updates[updateType]
			var color string
			switch updateType {
			case scheduler.UpdateTypeIgnore:
			case scheduler.UpdateTypeCreate:
				color = "[green]"
			case scheduler.UpdateTypeDestroy:
				color = "[red]"
			case scheduler.UpdateTypeMigrate:
				color = "[blue]"
			case scheduler.UpdateTypeInplaceUpdate:
				color = "[cyan]"
			case scheduler.UpdateTypeDestructiveUpdate:
				color = "[yellow]"
			case scheduler.UpdateTypeCanary:
				color = "[light_yellow]"
			}
			updates = append(updates, fmt.Sprintf("[reset]%s%d %s", color, count, updateType))
		}
		out += fmt.Sprintf(" (%s[reset])\n", strings.Join(updates, ", "))
	} else {
		out += "[reset]\n"
	}

	// Determine the longest field and markers so the output is properly
	// aligned
	longestField, longestMarker := getLongestPrefixes(tg.Fields, tg.Objects)
	for _, task := range tg.Tasks {
		if _, l := getDiffString(task.Type); l > longestMarker {
			longestMarker = l
		}
	}

	// Only show the task groups's field and object diffs if the group is edited or
	// verbose mode is set.
	subStartPrefix := tgPrefix + 2
	if tg.Type == "Edited" || verbose {
		fo := alignedFieldAndObjects(tg.Fields, tg.Objects, subStartPrefix, longestField, longestMarker)
		out += fo
		if len(fo) > 0 {
			out += "\n"
		}
	}

	// Output the tasks
	for _, task := range tg.Tasks {
		_, mLength := getDiffString(task.Type)
		prefix := longestMarker - mLength
		out += fmt.Sprintf("%s\n", formatTaskDiff(task, subStartPrefix, prefix, verbose))
	}

	return out
}

// formatTaskDiff produces an annotated diff of a task. If the verbose field is
// set, the tasks fields and objects are expanded even if the full object is an
// addition or removal. startPrefix is the number of spaces to prefix the output of
// the task and taskPrefix is the number of spaces to put between the marker and
// task name output.
func formatTaskDiff(task *api.TaskDiff, startPrefix, taskPrefix int, verbose bool) string {
	marker, _ := getDiffString(task.Type)
	out := fmt.Sprintf("%s%s%s[bold]Task: %q",
		strings.Repeat(" ", startPrefix), marker, strings.Repeat(" ", taskPrefix), task.Name)
	if len(task.Annotations) != 0 {
		out += fmt.Sprintf(" [reset](%s)", colorAnnotations(task.Annotations))
	}

	if task.Type == "None" {
		return out
	} else if (task.Type == "Deleted" || task.Type == "Added") && !verbose {
		// Exit early if the job was not edited and it isn't verbose output
		return out
	} else {
		out += "\n"
	}

	subStartPrefix := startPrefix + 2
	longestField, longestMarker := getLongestPrefixes(task.Fields, task.Objects)
	out += alignedFieldAndObjects(task.Fields, task.Objects, subStartPrefix, longestField, longestMarker)
	return out
}

// formatObjectDiff produces an annotated diff of an object. startPrefix is the
// number of spaces to prefix the output of the object and keyPrefix is the number
// of spaces to put between the marker and object name output.
func formatObjectDiff(diff *api.ObjectDiff, startPrefix, keyPrefix int) string {
	start := strings.Repeat(" ", startPrefix)
	marker, markerLen := getDiffString(diff.Type)
	out := fmt.Sprintf("%s%s%s%s {\n", start, marker, strings.Repeat(" ", keyPrefix), diff.Name)

	// Determine the length of the longest name and longest diff marker to
	// properly align names and values
	longestField, longestMarker := getLongestPrefixes(diff.Fields, diff.Objects)
	subStartPrefix := startPrefix + keyPrefix + 2
	out += alignedFieldAndObjects(diff.Fields, diff.Objects, subStartPrefix, longestField, longestMarker)

	endprefix := strings.Repeat(" ", startPrefix+markerLen+keyPrefix)
	return fmt.Sprintf("%s\n%s}", out, endprefix)
}

// formatFieldDiff produces an annotated diff of a field. startPrefix is the
// number of spaces to prefix the output of the field, keyPrefix is the number
// of spaces to put between the marker and field name output and valuePrefix is
// the number of spaces to put infront of the value for aligning values.
func formatFieldDiff(diff *api.FieldDiff, startPrefix, keyPrefix, valuePrefix int) string {
	marker, _ := getDiffString(diff.Type)
	out := fmt.Sprintf("%s%s%s%s: %s",
		strings.Repeat(" ", startPrefix),
		marker, strings.Repeat(" ", keyPrefix),
		diff.Name,
		strings.Repeat(" ", valuePrefix))

	switch diff.Type {
	case "Added":
		out += fmt.Sprintf("%q", diff.New)
	case "Deleted":
		out += fmt.Sprintf("%q", diff.Old)
	case "Edited":
		out += fmt.Sprintf("%q => %q", diff.Old, diff.New)
	default:
		out += fmt.Sprintf("%q", diff.New)
	}

	// Color the annotations where possible
	if l := len(diff.Annotations); l != 0 {
		out += fmt.Sprintf(" (%s)", colorAnnotations(diff.Annotations))
	}

	return out
}

// alignedFieldAndObjects is a helper method that prints fields and objects
// properly aligned.
func alignedFieldAndObjects(fields []*api.FieldDiff, objects []*api.ObjectDiff,
	startPrefix, longestField, longestMarker int) string {

	var out string
	numFields := len(fields)
	numObjects := len(objects)
	haveObjects := numObjects != 0
	for i, field := range fields {
		_, mLength := getDiffString(field.Type)
		keyPrefix := longestMarker - mLength
		valPrefix := longestField - len(field.Name)
		out += formatFieldDiff(field, startPrefix, keyPrefix, valPrefix)

		// Avoid a dangling new line
		if i+1 != numFields || haveObjects {
			out += "\n"
		}
	}

	for i, object := range objects {
		_, mLength := getDiffString(object.Type)
		keyPrefix := longestMarker - mLength
		out += formatObjectDiff(object, startPrefix, keyPrefix)

		// Avoid a dangling new line
		if i+1 != numObjects {
			out += "\n"
		}
	}

	return out
}

// getLongestPrefixes takes a list  of fields and objects and determines the
// longest field name and the longest marker.
func getLongestPrefixes(fields []*api.FieldDiff, objects []*api.ObjectDiff) (longestField, longestMarker int) {
	for _, field := range fields {
		if l := len(field.Name); l > longestField {
			longestField = l
		}
		if _, l := getDiffString(field.Type); l > longestMarker {
			longestMarker = l
		}
	}
	for _, obj := range objects {
		if _, l := getDiffString(obj.Type); l > longestMarker {
			longestMarker = l
		}
	}
	return longestField, longestMarker
}

// getDiffString returns a colored diff marker and the length of the string
// without color annotations.
func getDiffString(diffType string) (string, int) {
	switch diffType {
	case "Added":
		return "[green]+[reset] ", 2
	case "Deleted":
		return "[red]-[reset] ", 2
	case "Edited":
		return "[light_yellow]+/-[reset] ", 4
	default:
		return "", 0
	}
}

// colorAnnotations returns a comma concatonated list of the annotations where
// the annotations are colored where possible.
func colorAnnotations(annotations []string) string {
	l := len(annotations)
	if l == 0 {
		return ""
	}

	colored := make([]string, l)
	for i, annotation := range annotations {
		switch annotation {
		case "forces create":
			colored[i] = fmt.Sprintf("[green]%s[reset]", annotation)
		case "forces destroy":
			colored[i] = fmt.Sprintf("[red]%s[reset]", annotation)
		case "forces in-place update":
			colored[i] = fmt.Sprintf("[cyan]%s[reset]", annotation)
		case "forces create/destroy update":
			colored[i] = fmt.Sprintf("[yellow]%s[reset]", annotation)
		default:
			colored[i] = annotation
		}
	}

	return strings.Join(colored, ", ")
}

// formatTime formats the time to string based on RFC822
func formatTime(t time.Time) string {
	if t.Unix() < 1 {
		// It's more confusing to display the UNIX epoch or a zero value than nothing
		return ""
	}
	return t.Format("01/02/06 15:04:05 MST")
}

// formatTimeDifference takes two times and determines their duration difference
// truncating to a passed unit.
// E.g. formatTimeDifference(first=1m22s33ms, second=1m28s55ms, time.Second) -> 6s
func formatTimeDifference(first, second time.Time, d time.Duration) string {
	return second.Truncate(d).Sub(first.Truncate(d)).String()
}
