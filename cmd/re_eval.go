package cmd

import (
	"github.com/bdclark/nomadctl/logging"
	"github.com/bdclark/nomadctl/nomad"
	"github.com/hashicorp/nomad/api"
	"github.com/spf13/cobra"
)

// reEvalCmd represents the re-eval command
var reEvalCmd = &cobra.Command{
	Use:   "re-eval [JOB]",
	Short: "Re-evaluate a job or all jobs",
	Long:  `Forces a re-evaluation of a specific Nomad job or all jobs in the cluster.`,
	Args:  cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {

		allFlag, _ := cmd.Flags().GetBool("all")

		nomad, err := nomad.NewNomadClient(nil)
		if err != nil {
			bail(err, 1)
		}

		if len(args) == 0 && allFlag {
			// iterate and re-evaluate all jobs
			jobs, _, err := nomad.Jobs().List(&api.QueryOptions{})
			if err != nil {
				bail(err, 1)
			}

			for _, job := range jobs {
				logging.Info("evaluating %s", job.Name)
				if _, _, err := nomad.Jobs().ForceEvaluate(job.ID, nil); err != nil {
					logging.Error("  %s", err)
				}
			}
		} else if len(args) == 1 && !allFlag {
			// re-evaluate one job
			logging.Info("evaluating %s", args[0])
			if _, _, err := nomad.Jobs().ForceEvaluate(args[0], nil); err != nil {
				logging.Error("  %s", err)
			}
		} else {
			usageError(cmd, "requires exactly 1 arg or --all")
		}

		logging.Info("Done")
	},
}

func init() {
	rootCmd.AddCommand(reEvalCmd)

	reEvalCmd.Flags().Bool("all", false, "re-evaluate all jobs")
}
