package cmd

import (
	"os"

	"github.com/bdclark/nomadctl/deploy"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var planCmd = &cobra.Command{
	Use:   "plan",
	Short: "Plan a job from a template",
	Long: `Renders a Nomad job template using Consul-Template then invokes
the scheduler in a dry-run mode to determine what would happen if the job
is submitted.

To plan a job with the template source and options specified on the
command-line, use the "plan template" sub-command. To plan a job with the
template source and options specified in Consul, use the "plan kv"
sub-command.`,
}

var planTemplateCmd = &cobra.Command{
	Use:   "plan SOURCE",
	Short: "Plan a job from a template",
	Long: `Renders a Nomad job template using Consul-Template then invokes
the scheduler in a dry-run mode to determine what would happen if the job
is submitted.

See "nomadctl help render template" for details regarding the template
source and template rendering options.

Once rendered, the plan is executed and shown as standard output.
Display options can be set with command-line flags.`,
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
	Long: `Renders a Nomad job template using Consul-Template then invokes
the scheduler in a dry-run mode to determine what would happen if the job
is submitted.

The required JOBKEY argument is a Consul KV path and is expected to have one
or more sub-keys. If a "prefix" is specified via command-line flag, config
file setting or environment variable, the the actual JOBKEY becomes
"${PREFIX}/${JOBKEY}".

See "nomadctl help render kv" for details regarding the template source,
rendering options, and supported Consul keys.

Once rendered, unless the "quiet" command-line flag is specified, a
structured diff between the local and remote job is displayed to give
insight into what the scheduler will attempt to do and why.

One of the following exit codes will be returned:
* 0: No allocations created or destroyed.
* 1: Allocations created or destroyed.
* 255: Error determining plan results.`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		initConfig(cmd)
		doPlan(cmd, args[0])
	},
}

func init() {
	rootCmd.AddCommand(planCmd)
	planCmd.AddCommand(planTemplateCmd)
	planCmd.AddCommand(planKVCmd)

	addPlanFlags(planCmd)

	addConsulFlags(planKVCmd)
	addPlanFlags(planKVCmd)
}

func doPlan(cmd *cobra.Command, consulJobKey string) {
	// render template (and set related consul config if applicable)
	jobspec := doRender(cmd, consulJobKey, 255)

	// create new deployment
	deployment, err := deploy.NewDeployment(&deploy.NewDeploymentInput{Jobspec: &jobspec})
	if err != nil {
		bail(err, 255)
	}

	// run a deployment plan
	changes, err := deployment.Plan(viper.GetBool("plan.quiet"), viper.GetBool("plan.verbose"),
		viper.GetBool("plan.diff"), viper.GetBool("plan.no_color"))
	if err != nil {
		bail(err, 255)
	}

	// exit non-zero if allocation changes
	if changes {
		os.Exit(1)
	}
}
