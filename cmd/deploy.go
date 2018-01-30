package cmd

import (
	"fmt"
	"os"

	"github.com/bdclark/nomadctl/deploy"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var deployCmd = &cobra.Command{
	Use:   "deploy",
	Short: "Deploy a job and monitor its progress",
	Long: `Renders a Nomad job template using Consul-Template then deploys
the resulting job to Nomad.

To deploy a job with the template source and deployment options specified
on the command-line, use the "deploy template" sub-command. To deploy a job
with the template source and deployment options specified in Consul, use
use the "deploy kv" sub-command.`,
}

var deployTemplateCmd = &cobra.Command{
	Use:   "template SOURCE",
	Short: "Deploy a job specified locally",
	Long: `Renders a Nomad job template using Consul-Template then deploys
the resulting job to Nomad.

See "nomadctl help render template" for details regarding the template
source and template rendering options.

Once rendered, the job is registered with Nomad and monitored until
the deployment is complete. If the deployment fails, details of
the failed allocation(s) are logged.

If the job is configured with canary(s), the deployment can be
automatically promoted once the canary(s) are healthy using the
"auto-promote" command-line flag or related config file or environment
variable setting.

By default, if a remote job is running with the same name, nomadctl
will update the count within each task group to match that of the
remote job. Use the "force-count" command-line flag or related config
file or environment variable setting to force the deployment to use
the count(s) defined in the job template.`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		initConfig(cmd)
		viper.Set("template.source", args[0])
		doDeploy(cmd, "")
	},
}

var deployKVCmd = &cobra.Command{
	Use:   "kv JOBKEY",
	Short: "Deploy a job specified in Consul",
	Long: `Renders a Nomad job template using Consul-Template then deploys
the resulting job to Nomad with configuration information stored in Consul.

The required JOBKEY argument is a Consul KV path and is expected to have one
or more sub-keys. If a "prefix" is specified via command-line flag, config
file setting or environment variable, the the actual JOBKEY becomes
"${PREFIX}/${JOBKEY}".

See "nomadctl help render kv" for details regarding the template source,
rendering options, and supported Consul keys. In addition to the template-
related Consul keys, the following deployment-related Consul keys are
supported:

"${JOBKEY}/deploy/auto_promote" same as "--auto-promote" flag
"${JOBKEY}/deploy/force_count" same as "--force-count" flag

Once rendered, the job is registered with Nomad and monitored until
the deployment is complete. If the deployment fails, details of
the failed allocation(s) are logged.

If the job is configured with canary(s), the deployment can be
automatically promoted once the canary(s) are healthy using the
"auto-promote" command-line flag, config file setting, environment
variable, or Consul key.

By default, if a remote job is running with the same name, nomadctl
will update the count within each task group to match that of the
remote job so the number of resulting allocations will not change.
Use the "force-count" command-line flag or related config file,
environment variable, or Consul KV setting to force the deployment
to use the count(s) defined in the job template.

Settings in Consul override config file and environment variable settings,
However, if a command-line flag is specified, it overrides the related
setting found in Consul.`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		initConfig(cmd)
		doDeploy(cmd, args[0])
	},
}

func init() {
	rootCmd.AddCommand(deployCmd)
	deployCmd.AddCommand(deployTemplateCmd)
	deployCmd.AddCommand(deployKVCmd)

	addConfigFlags(deployTemplateCmd)
	addDeployFlags(deployTemplateCmd)

	addConfigFlags(deployKVCmd)
	addConsulFlags(deployKVCmd)
	addDeployFlags(deployKVCmd)
}

func doDeploy(cmd *cobra.Command, consulJobKey string) {
	// render template (and set related consul config if applicable)
	jobspec := doRender(cmd, consulJobKey)

	// create deployment
	deployment, err := deploy.NewDeployment(&deploy.NewDeploymentInput{
		AutoPromote:      viper.GetBool("deploy.auto_promote"),
		UseTemplateCount: viper.GetBool("deploy.force_count"),
		Verbose:          false,
		Jobspec:          &jobspec,
	})
	if err != nil {
		bail(err, 1)
	}

	// run a job plan if specified
	if viper.GetBool("deploy.plan") {
		changes, err := deployment.Plan(false, true, false)
		if err != nil {
			bail(err, 1)
		}

		if changes && !viper.GetBool("deploy.force") {
			if confirm := askForConfirmation("Changes found, continue deployment?"); !confirm {
				fmt.Fprintln(os.Stderr, "Abandoning deployment.")
				os.Exit(0)
			}
		}
	}

	// deploy
	if _, err = deployment.Deploy(); err != nil {
		bail(err, 1)
	}
}
