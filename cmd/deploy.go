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
	Use:   "deploy TEMPLATE",
	Short: "Deploy a Nomad job from a template",
	Long:  `Deploy a Nomad job`,
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		viper.Set("template.source", args[0])
		doDeploy(cmd, "")
	},
}

var deployKVCmd = &cobra.Command{
	Use:   "kv JOBKEY",
	Short: "Deploy a Nomad job defined in Consul",
	Long:  `Deploy a Nomad job defined in Consul`,
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		doDeploy(cmd, args[0])
	},
}

func init() {
	rootCmd.AddCommand(deployCmd)
	deployCmd.AddCommand(deployKVCmd)

	addTemplateFlags(deployCmd)
	addDeployFlags(deployCmd)

	addConsulFlags(deployKVCmd)
	addTemplateFlags(deployKVCmd)
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
