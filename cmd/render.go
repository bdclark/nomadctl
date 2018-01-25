package cmd

import (
	"fmt"
	"os"

	"github.com/bdclark/nomadctl/template"
	consul "github.com/hashicorp/consul/api"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// renderCmd represents the render command
var renderCmd = &cobra.Command{
	Use:   "render TEMPLATE",
	Short: "Render a Nomad job template to stdout",
	Long:  `Renders a Nomad job template using Consul-Template to standard output.`,
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		viper.Set("template.source", args[0])
		fmt.Fprintf(os.Stdout, "%s", doRender(cmd, ""))
	},
}

var renderKVCmd = &cobra.Command{
	Use:   "kv JOBKEY",
	Short: "Render a Nomad job template from Consul",
	Long: `Renders a Nomad job template using Consul-Template to standard output,
using configuration information from Consul.`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Fprintf(os.Stdout, "%s", doRender(cmd, args[0]))
	},
}

func init() {
	rootCmd.AddCommand(renderCmd)
	renderCmd.AddCommand(renderKVCmd)

	addTemplateFlags(renderCmd)
	addConsulFlags(renderKVCmd)
	addTemplateFlags(renderKVCmd)
}

func doRender(cmd *cobra.Command, consulJobKey string) []byte {
	// bind cli flags to viper
	bindFlags(cmd)

	// update viper settings from Consul
	if consulJobKey != "" {
		client, err := consul.NewClient(consul.DefaultConfig())
		if err != nil {
			bail(err, 1)
		}

		err = setConfigFromKV(cmd, client, consulJobKey)
		if err != nil {
			bail(err, 1)
		}

		if k := canonicalizeJobKey(consulJobKey); consulJobKey != "" && k != "" {
			os.Setenv("NOMADCTL_PREFIX", viper.GetString("prefix"))
			os.Setenv("NOMADCTL_CLI_JOB_KEY", consulJobKey)
			os.Setenv("NOMADCTL_JOB_KEY", k)
		}
	}

	// set template input from viper
	config := &template.NewTemplateInput{
		Source:        viper.GetString("template.source"),
		Contents:      viper.GetString("template.contents"),
		LeftDelim:     viper.GetString("template.left_delimeter"),
		RightDelim:    viper.GetString("template.right_delimeter"),
		ErrMissingKey: viper.GetBool("template.error_on_missing_key"),
		Options:       viper.GetStringMapString("template.options"),
	}

	// create template
	template, err := template.NewTemplate(config)
	if err != nil {
		bail(err, 1)
	}

	// render template
	output, err := template.Render()
	if err != nil {
		bail(err, 1)
	}
	return output
}
