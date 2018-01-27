package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/bdclark/nomadctl/deploy"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// deployCmd represents the deploy command
var deployCmd = &cobra.Command{
	Use:   "deploy TEMPLATE_SOURCE",
	Short: "Deploy a job defined from a template",
	Long: `Renders a Nomad job template using Consul-Template then deploys
the resulting job to Nomad.

The specified job key is a prefix that is expected to have one or more
sub-keys. If a "prefix" is specified via command-line flag, config file,
or environment variable, the the actual job key becomes "<prefix>/<jobkey>".

See "nomadctl help render kv" for details regarding the template source,
rendering options, and supported Consul keys. In addition to the template-
related Consul keys, the following deployment-related Consul keys are
supported:

"<jobkey>/deploy/auto_promote" - same as "--auto-promote" command-line flag
"<jobkey>/deploy/force_count" - same as "--force-count" command-line flag

Once rendered, the job is registered with Nomad and monitored until
the deployment is complete. If the deployment fails, details of
the failed allocation(s) are logged.

If the job is configured with canary(s), the deployment can be
automatically promoted once the canary(s) are healthy using the
"auto-promote" command-line flag or related config file, environment
variable, or Consul KV setting.

By default, if a remote job is running with the same name, nomadctl
will update the count within each task group to match that of the
remote job so the number of resulting allocations will not change.
Use the "force-count" command-line flag or related config file,
environment variable, or Consul KV setting to force the deployment
to use the count(s) defined in the job template.`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		initConfig(cmd)
		viper.Set("template.source", args[0])
		doDeploy(cmd, "")
	},
}

var deployKVCmd = &cobra.Command{
	Use:   "kv JOBKEY",
	Short: "Deploy a job defined in Consul",
	Long: `Renders a Nomad job template using Consul-Template then deploys
the resulting job to Nomad using configuration information
stored in Consul.

See "nomadctl help render" for details regarding the template source
and template rendering options.

Once rendered, the job is registered with Nomad and monitored until
the deployment is complete. If the deployment fails, details of
the failed allocation(s) are logged.

If the job is configured with canary(s), the deployment can be
automatically promoted once the canary(s) are healthy using the
"auto-promote" command-line flag or related config file / environment
variable setting.

By default, if a remote job is running with the same name, nomadctl
will update the count within each task group to match that of the
remote job so the number of resulting allocations will not change.
Use the "force-count" command-line flag or related config file /
environment variable setting to force the deployment to use
the count(s) defined in the job template.`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		initConfig(cmd)
		doDeploy(cmd, args[0])
	},
}

func init() {
	rootCmd.AddCommand(deployCmd)
	deployCmd.AddCommand(deployKVCmd)

	addConfigFlags(deployCmd)
	addDeployFlags(deployCmd)

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

	if viper.GetBool("deploy.plan") {
		changes, err := deployment.Plan(false, true, false)
		if err != nil {
			bail(err, 1)
		}

		if changes && !viper.GetBool("deploy.force") {
			if confirm := askForConfirmation("Changes found, continue deployment?"); !confirm {
				fmt.Fprintln(os.Stderr, "Abandoning deloyment.")
				os.Exit(0)
			}
		}
	}

	// deploy
	if _, err = deployment.Deploy(); err != nil {
		bail(err, 1)
	}
}

func askForConfirmation(s string) bool {
	reader := bufio.NewReader(os.Stdin)

	for {
		fmt.Printf("%s [y/n]: ", s)

		response, err := reader.ReadString('\n')
		if err != nil {
			log.Fatal(err)
		}

		response = strings.ToLower(strings.TrimSpace(response))

		if response == "y" || response == "yes" {
			return true
		} else if response == "n" || response == "no" {
			return false
		}
	}
}
