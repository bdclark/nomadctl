package cmd

import (
	"fmt"
	"os"

	"github.com/bdclark/nomadctl/nomad"
	"github.com/spf13/cobra"
)

// restartCmd represents the restart command
var restartCmd = &cobra.Command{
	Use:   "restart JOB",
	Short: "Restart a job or task group",
	Long:  `Restarts a Nomad job or a task group within a job if specified.`,
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {

		group, _ := cmd.Flags().GetString("group")

		nomad, err := nomad.NewNomadClient(nil)
		if err != nil {
			bail(err, 1)
		}

		if group == "" {
			if err := nomad.RestartJob(args[0]); err != nil {
				bail(err, 1)
			}
		} else {
			if err := nomad.RestartTaskGroup(args[0], group); err != nil {
				bail(err, 1)
			}
		}

		fmt.Fprintln(os.Stderr, "Done")
	},
}

func init() {
	rootCmd.AddCommand(restartCmd)

	restartCmd.Flags().String("group", "", "Task group to restart rather than entire job")
}
