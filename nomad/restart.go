package nomad

import (
	"fmt"
)

// RestartJob restarts a nomad job by deregistering/registering
func (n *Client) RestartJob(jobName string) error {
	job, _, err := n.Jobs().Info(jobName, nil)
	if err != nil {
		return err
	}

	// stop
	if _, _, err = n.Jobs().Deregister(jobName, false, nil); err != nil {
		return err
	}

	// start
	job.Stop = boolToPtr(false) // start no matter what
	if _, _, err = n.Jobs().Register(job, nil); err != nil {
		return err
	}

	return nil
}

// RestartTaskGroup restarts a Nomad task group by
// temporarily setting the count to zero
func (n *Client) RestartTaskGroup(jobName string, groupName string) error {
	job, _, err := n.Jobs().Info(jobName, nil)
	if err != nil {
		return err
	}

	for _, tg := range job.TaskGroups {
		if *tg.Name == groupName {
			prevCount := tg.Count

			//set to zero
			tg.Count = intToPtr(0)
			if _, _, err := n.Jobs().Register(job, nil); err != nil {
				return err
			}

			// set to previous count
			tg.Count = prevCount
			if _, _, err := n.Jobs().Register(job, nil); err != nil {
				return err
			}

			return nil
		}
	}
	return fmt.Errorf("could not find task group: %s", groupName)
}
