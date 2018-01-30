package cmd

import (
	"fmt"
	"os"
	"strconv"

	"github.com/bdclark/nomadctl/nomad"
	"github.com/spf13/cobra"
)

// scaleCmd represents the scale command
var scaleCmd = &cobra.Command{
	Use:   "scale",
	Short: "Scale a job or task group",
	Long: `Scales the number of instances of a Nomad task group up, down,
or to a specific count.`,
}

var scaleGetCmd = &cobra.Command{
	Use:   "get JOB GROUP",
	Short: "Get the current count of a Nomad task group",
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		jobName := args[0]
		tgName := args[1]

		nomad, err := nomad.NewNomadClient(nil)
		if err != nil {
			bail(err, 1)
		}

		job, _, err := nomad.Jobs().Info(jobName, nil)
		if err != nil {
			bail(err, 1)
		}

		for _, tg := range job.TaskGroups {
			if *tg.Name == tgName {
				fmt.Fprintln(os.Stdout, *tg.Count)
				os.Exit(0)
			}
		}
		bail(fmt.Errorf("could not find task group: %s", tgName), 1)
	},
}

var scaleUpCmd = &cobra.Command{
	Use:   "up JOB GROUP COUNT",
	Short: "Scale a task group up by the given count",
	Args:  cobra.ExactArgs(3),
	Run: func(cmd *cobra.Command, args []string) {
		jobName := args[0]
		tgName := args[1]
		delta, err := strconv.Atoi(args[2])
		if err != nil {
			bail(err, 1)
		}
		scaleAdjust(jobName, tgName, delta)
	},
}

var scaleDownCmd = &cobra.Command{
	Use:   "down JOB GROUP COUNT",
	Short: "Scale a task group down by the given count",
	Run: func(cmd *cobra.Command, args []string) {
		jobName := args[0]
		tgName := args[1]
		delta, err := strconv.Atoi(args[2])
		if err != nil {
			bail(err, 1)
		}
		scaleAdjust(jobName, tgName, -delta)
	},
}

var scaleSetCmd = &cobra.Command{
	Use:   "set JOB GROUP COUNT",
	Short: "Scale a task group to a given count",
	Args:  cobra.ExactArgs(3),
	Run: func(cmd *cobra.Command, args []string) {
		jobName := args[0]
		tgName := args[1]
		count, err := strconv.Atoi(args[2])
		if err != nil {
			bail(err, 1)
		}

		nomad, err := nomad.NewNomadClient(nil)
		if err != nil {
			bail(err, 1)
		}

		job, _, err := nomad.Jobs().Info(jobName, nil)
		if err != nil {
			bail(err, 1)
		}

		if err := nomad.SetTaskGroupCount(job, tgName, count); err != nil {
			bail(err, 1)
		}
	},
}

func init() {
	rootCmd.AddCommand(scaleCmd)
	scaleCmd.AddCommand(scaleGetCmd)
	scaleCmd.AddCommand(scaleUpCmd)
	scaleCmd.AddCommand(scaleDownCmd)
	scaleCmd.AddCommand(scaleSetCmd)
}

func scaleAdjust(jobName string, tgName string, delta int) {
	nomad, err := nomad.NewNomadClient(nil)
	if err != nil {
		bail(err, 1)
	}

	job, _, err := nomad.Jobs().Info(jobName, nil)
	if err != nil {
		bail(err, 1)
	}

	if err := nomad.AdjustTaskGroupCount(job, tgName, delta); err != nil {
		bail(err, 1)
	}
}
