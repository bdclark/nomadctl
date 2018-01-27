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
	Short: "Render a job template to stdout",
	Long: `Renders a Nomad job template using Consul-Template to standard output.

The specified template source can either be a path to a template on the
local filesystem, or a remote artifact. Similar to Nomad's artifact
retrieval, nomadctl downloads remote artifacts using the go-getter
library, which permits the downloading of artifacts from a variety of
locations using a URL as the input source.  Go-getter options can be
supplied with command-line flags or config file settings.

The template is rendered using Consul-Template using the default
delimeters of "{{" and "}}", but can be overriden with command-line
flags or config file settings.`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		initConfig(cmd)
		viper.Set("template.source", args[0])
		fmt.Fprintf(os.Stdout, "%s", doRender(cmd, ""))
	},
}

var renderKVCmd = &cobra.Command{
	Use:   "kv JOBKEY",
	Short: "Render a job template from Consul",
	Long: `Renders a Nomad job template using Consul-Template to standard output,
using configuration information stored in Consul.

The specified job key is a prefix that is expected to have one or more
sub-keys.  If a "prefix" is specified via command-line flag, config file,
or environment variable, the the actual job key becomes "<prefix>/<jobkey>".

The following Consul keys are supported:
"<jobkey>/template/source" the source of the template
"<jobkey>/template/contents" template contents (mutually exlusive of source)
"<jobkey>/template/left_delimeter" same as --left-delim flag
"<jobkey>/template/right_delimeter" same as --right-delim flag
"<jobkey>/template/error_on_missing_key" same as --err-missing-key flag
"<jobkey>/template/options/*" go-getter options if source is URL

The specified template source can either be a path to a template on the
local filesystem, or a remote artifact. Similar to Nomad's artifact
retrieval, nomadctl downloads remote artifacts using the go-getter
library, which permits the downloading of artifacts from a variety of
locations using a URL as the input source.  Go-getter options can be
supplied with consul keys, command-line flags or config file settings.

Settings in Consul override config file and environment variable settings,
However, if a command-line flag is specfied, it overrides anything
found in Consul.`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		initConfig(cmd)
		fmt.Fprintf(os.Stdout, "%s", doRender(cmd, args[0]))
	},
}

func init() {
	rootCmd.AddCommand(renderCmd)
	renderCmd.AddCommand(renderKVCmd)

	addConfigFlags(renderCmd)
	addTemplateFlags(renderCmd)

	addConfigFlags(renderKVCmd)
	addConsulFlags(renderKVCmd)
	addTemplateFlags(renderKVCmd)
}

func doRender(cmd *cobra.Command, consulJobKey string) []byte {
	initConfig(cmd)

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
