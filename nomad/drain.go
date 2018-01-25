package nomad

import (
	"fmt"
	"time"

	"github.com/bdclark/nomadctl/logging"
	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/nomad/structs"
)

// ManagedDrain drains a node and blocks until allocs are complete
// if nodeID is nil, the current agent will be used
func (n *Client) ManagedDrain(nodeID string) error {
	if nodeID == "" {
		self, err := n.Agent().Self()
		if err != nil {
			return err
		}

		var ok bool
		nodeID, ok = self.Stats["client"]["node_id"]
		if !ok {
			return fmt.Errorf("could not find client node id, is node in client mode?")
		}
	}

	if _, err := n.Nodes().ToggleDrain(nodeID, true, nil); err != nil {
		return err
	}

	q := &api.QueryOptions{
		WaitIndex: 0,
		WaitTime:  time.Duration(10 * time.Second),
	}

	for {

		allocs, meta, err := n.Nodes().Allocations(nodeID, q)
		if err != nil {
			return err
		}

		q.WaitIndex = meta.LastIndex

		pending := 0
		for _, alloc := range allocs {
			switch alloc.ClientStatus {
			case structs.AllocClientStatusRunning, structs.AllocClientStatusPending:
				pending++
			}
		}

		if pending == 0 {
			break
		}

		logging.Info("%d pending allocations remaining", pending)
	}
	return nil
}
