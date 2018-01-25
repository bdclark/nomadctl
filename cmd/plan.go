package cmd

import (
	"os"

	"github.com/bdclark/nomadctl/deploy"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var planCmd = &cobra.Command{
	Use:   "plan TEMPLATE",
	Short: "Plan a Nomad job from a template",
	Long:  `Plan a Nomad job`,
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		viper.Set("template.source", args[0])
		doPlan(cmd, "")
	},
}

var planKVCmd = &cobra.Command{
	Use:   "kv JOBKEY",
	Short: "Plan a Nomad job defined in Consul",
	Long:  `Plan a Nomad job defined in Consul`,
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		doPlan(cmd, args[0])
	},
}

func init() {
	rootCmd.AddCommand(planCmd)
	planCmd.AddCommand(planKVCmd)

	addTemplateFlags(planCmd)

	addConsulFlags(planKVCmd)
	addTemplateFlags(planKVCmd)

	planCmd.PersistentFlags().Bool("no-color", false, "disable colorized output")
	planCmd.PersistentFlags().Bool("diff", true, "show diff between remote job and planned job")
	planCmd.PersistentFlags().Bool("verbose", false, "verbose plan output")

	viper.BindPFlag("plan.no_color", planCmd.PersistentFlags().Lookup("no-color"))
	viper.BindPFlag("plan.diff", planCmd.PersistentFlags().Lookup("diff"))
	viper.BindPFlag("plan.verbose", planCmd.PersistentFlags().Lookup("verbose"))
}

func doPlan(cmd *cobra.Command, consulJobKey string) {
	// render template (and set related consul config if applicable)
	jobspec := doRender(cmd, consulJobKey)

	// create new deployment
	deployment, err := deploy.NewDeployment(&deploy.NewDeploymentInput{Jobspec: &jobspec})
	if err != nil {
		bail(err, 1)
	}

	// run a deployment plan
	changes, err := deployment.Plan(viper.GetBool("plan.verbose"), viper.GetBool("plan.diff"), viper.GetBool("plan.no_color"))
	if err != nil {
		bail(err, 1)
	}

	// exit non-zero if allocation changes
	if changes {
		os.Exit(1)
	}
}
