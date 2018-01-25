package nomad

import (
	"fmt"

	"github.com/hashicorp/nomad/api"
)

// Client is a Nomad API client
type Client struct {
	*api.Client
}

// NewNomadClient creates a new Nomad API client
func NewNomadClient(c *api.Config) (*Client, error) {
	if c == nil {
		c = api.DefaultConfig()
	}

	client, err := api.NewClient(c)
	if err != nil {
		return nil, err
	}

	return &Client{client}, nil
}

// GetNodeID gets a Nomad node ID from a node name,
// and errors if more/less than one matching name is found
func (n *Client) GetNodeID(name string) (id string, err error) {
	nodes, _, err := n.Nodes().List(&api.QueryOptions{})
	if err != nil {
		return
	}

	var nodeIDs []string
	for _, node := range nodes {
		if node.Name == name {
			nodeIDs = append(nodeIDs, node.ID)
		}
	}

	if len(nodeIDs) == 1 {
		id = nodeIDs[0]
	} else {
		err = fmt.Errorf("found %d nodes matching name `%s`", len(nodeIDs), name)
	}
	return
}

// boolToPtr returns the pointer to a bool
func boolToPtr(b bool) *bool {
	return &b
}

// intToPtr returns the pointer to an int
func intToPtr(i int) *int {
	return &i
}

// ptrToInt returns the value of an *int
func ptrToInt(i *int) int {
	return *i
}
