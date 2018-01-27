package cmd

import (
	"fmt"
	"os"

	"github.com/bdclark/nomadctl/nomad"
	"github.com/spf13/cobra"
)

// gcCmd represents the gc command
var gcCmd = &cobra.Command{
	Use:   "gc",
	Short: "Force cluster garbage collection",
	Long:  `The gc command will force a Nomad cluster garbage collection.`,
	Args:  cobra.ExactArgs(0),
	Run: func(cmd *cobra.Command, args []string) {

		nomad, err := nomad.NewNomadClient(nil)
		if err != nil {
			bail(err, 1)
		}

		if err = nomad.System().GarbageCollect(); err != nil {
			bail(err, 1)
		}

		fmt.Fprintln(os.Stderr, "Done")
	},
}

func init() {
	rootCmd.AddCommand(gcCmd)
}
