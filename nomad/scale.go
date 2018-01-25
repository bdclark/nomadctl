package nomad

import (
	"fmt"

	"github.com/bdclark/nomadctl/logging"
	"github.com/hashicorp/nomad/api"
)

// AdjustTaskGroupCount raises/lowers the count of a task group
func (n *Client) AdjustTaskGroupCount(job *api.Job, groupName string, delta int) error {
	for _, tg := range job.TaskGroups {
		if *tg.Name == groupName {
			newCount := intToPtr(ptrToInt(tg.Count) + delta)
			if *newCount < 0 {
				return fmt.Errorf("Count cannot be less than zero")
			}
			logging.Info("scaling group \"%s\" of job \"%s\" from %d to %d", groupName, *job.Name, *tg.Count, *newCount)
			tg.Count = newCount
			if _, _, err := n.Jobs().Register(job, nil); err != nil {
				return err
			}
			return nil
		}
	}
	return fmt.Errorf("could not find task group: %s", groupName)
}

// SetTaskGroupCount sets the count of a task group to the given count
func (n *Client) SetTaskGroupCount(job *api.Job, groupName string, count int) error {
	for _, tg := range job.TaskGroups {
		if *tg.Name == groupName {
			newCount := intToPtr(count)
			if *tg.Count == *newCount {
				return nil // nothing to do
			}
			logging.Info("scaling group \"%s\" of job \"%s\" from %d to %d", groupName, *job.Name, *tg.Count, *newCount)
			tg.Count = newCount
			if _, _, err := n.Jobs().Register(job, nil); err != nil {
				return err
			}
			return nil
		}
	}
	return fmt.Errorf("could not find task group: %s", groupName)
}
