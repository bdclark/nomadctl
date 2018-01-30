package cmd

import (
	"github.com/bdclark/nomadctl/deploy"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var redeployCmd = &cobra.Command{
	Use:   "redeploy JOB",
	Short: "Re-deploy a job, causing a rolling restart",
	Long: `Re-deploys an existing Nomad job or task group(s) within a job,
effectively causing a rolling restart of the targeted job/group(s).

The re-deployment is performed by adding/updating a particular
Meta key within the relevant task group(s) of a job, then deploying
the updated job.

Normal job update settings apply, including canaries. If canaries 
are configured, you can use the "--auto-promote" flag to automatically
promote the deployment after the canary(s) are healthy.`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		initConfig(cmd)

		groups, _ := cmd.Flags().GetStringSlice("group")

		_, err := deploy.ReDeploy(&deploy.RedeploymentInput{
			JobName:        args[0],
			TaskGroupNames: groups,
			AutoPromote:    viper.GetBool("deploy.auto_promote"),
			Verbose:        false,
		})
		if err != nil {
			bail(err, 1)
		}
	},
}

func init() {
	rootCmd.AddCommand(redeployCmd)

	addConfigFlags(redeployCmd)
	redeployCmd.Flags().Bool("auto-promote", false, "automatically promote canary deployment")
	redeployCmd.Flags().StringSlice("group", []string{}, "group to redeploy (can be supplied multiple times)")
}
