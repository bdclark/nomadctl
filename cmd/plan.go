package cmd

import (
	"os"

	"github.com/bdclark/nomadctl/deploy"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var planCmd = &cobra.Command{
	Use:   "plan TEMPLATE",
	Short: "Plan a job from a template",
	Long: `Renders a Nomad job template using Consul-Template then envokes
the scheduler in a dry-run mode to determine what would happen if the job
is submitted.

See "nomadctl help render" for details regarding the template source,
rendering options, and supported Consul keys.

Once rendered, the plan is executed and shown as standard output.
Display options can be set with various command-line flags.`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		initConfig(cmd)
		viper.Set("template.source", args[0])
		doPlan(cmd, "")
	},
}

var planKVCmd = &cobra.Command{
	Use:   "kv JOBKEY",
	Short: "Plan a job defined in Consul",
	Long: `Renders a Nomad job template using Consul-Template then envokes
the scheduler in a dry-run mode to determine what would happen if the job
is submitted.

The specified job key is a prefix that is expected to have one or more
sub-keys. If a "prefix" is specified via command-line flag, config file,
or environment variable, the the actual job key becomes "<prefix>/<jobkey>".

See "nomadctl help render kv" for details regarding the template source,
rendering options, and supported Consul keys.

Once rendered, the plan is executed and shown as standard output.
Display options can be set with various command-line flags`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		initConfig(cmd)
		doPlan(cmd, args[0])
	},
}

func init() {
	rootCmd.AddCommand(planCmd)
	planCmd.AddCommand(planKVCmd)

	addConfigFlags(planCmd)
	addTemplateFlags(planCmd)

	addConfigFlags(planKVCmd)
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
