package cmd

import (
	"fmt"
	"os"

	"github.com/bdclark/nomadctl/nomad"
	"github.com/spf13/cobra"
)

// drainCmd represents the drain command
var drainCmd = &cobra.Command{
	Use:   "drain",
	Short: "Drain a Nomad node and wait until done",
	Long:  `Drains a Nomad node, waiting until all allocations are no longer running.`,
	Args:  cobra.ExactArgs(0),
	Run: func(cmd *cobra.Command, args []string) {

		var nodeID string
		self, _ := cmd.Flags().GetBool("self")
		id, _ := cmd.Flags().GetString("id")
		name, _ := cmd.Flags().GetString("name")

		nomad, err := nomad.NewNomadClient(nil)
		if err != nil {
			bail(err, 1)
		}

		switch {
		case id != "":
			nodeID = id
		case self:
			nodeID = ""
		case name != "":
			if nodeID, err = nomad.GetNodeID(name); err != nil {
				bail(err, 1)
			}
		default:
			cmd.Usage()
			fmt.Fprintln(os.Stderr, "\nMust supply --id, --name, or --self")
			os.Exit(1)
		}

		if err = nomad.ManagedDrain(nodeID); err != nil {
			bail(err, 1)
		}

		fmt.Fprintln(os.Stderr, "Done")
	},
}

func init() {
	rootCmd.AddCommand(drainCmd)

	drainCmd.Flags().Bool("self", false, "drain current node")
	drainCmd.Flags().String("id", "", "node id to drain")
	drainCmd.Flags().String("name", "", "node name to drain (must be unique in cluster)")
}
