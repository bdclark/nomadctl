package cmd

import (
	"fmt"
	"os"

	"github.com/bdclark/nomadctl/template"
	consul "github.com/hashicorp/consul/api"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var renderCmd = &cobra.Command{
	Use:   "render",
	Short: "Render a job template to stdout",
	Long: `Renders a Nomad job template using Consul-Template to standard output.

To render a template with the source specified on the command-line, use the
"render template" sub-command. To render a template with the source, contents,
or other setttings specified in Consul, use the "render kv" sub-command.`,
}

var renderTemplateCmd = &cobra.Command{
	Use:   "template SOURCE",
	Short: "Render a job template specified locally",
	Long: `Renders a Nomad job template using Consul-Template to standard output.

The specified template source can either be a path to a template on the
local filesystem, or a remote artifact. Similar to Nomad's artifact
retrieval, nomadctl downloads remote artifacts using the go-getter
library, which permits the downloading of artifacts from a variety of
locations using a URL as the input source.  Go-getter options can be
supplied with command-line flags or config file settings.

The template is rendered using Consul-Template using the default
delimeters of "{{" and "}}", but can be overridden with command-line
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
	Short: "Render a job template specified in Consul",
	Long: `Renders a Nomad job template using Consul-Template to standard output
with configuration information stored in Consul.

The required JOBKEY argument is a Consul KV path and is expected to have one
or more sub-keys. If a "prefix" is specified via command-line flag, config
file setting or environment variable, the the actual JOBKEY becomes
"${PREFIX}/${JOBKEY}".

The following Consul keys are supported:

"${JOBKEY}/template/source" the source of the template
"${JOBKEY}/template/contents" template contents (mutually exclusive of source)
"${JOBKEY}/template/left_delimeter" same as "--left-delim" flag
"${JOBKEY}/template/right_delimeter" same as "--right-delim" flag
"${JOBKEY}/template/error_on_missing_key" same as" --err-missing-key" flag
"${JOBKEY}/template/options/*" go-getter options if source is URL

The specified template source can either be a path to a template on the
local filesystem or a remote artifact. Similar to Nomad's artifact
retrieval, nomadctl downloads remote artifacts using the go-getter
library, which permits the downloading of artifacts from a variety of
locations using a URL as the input source.  Go-getter options can be
supplied with consul keys, command-line flags or config file settings.

In addition to standard getter options, a "path" option is supported,
and is required if the source URL is a compressed archive or VCS repo
that constains more than one file.

Settings in Consul override config file and environment variable settings,
However, if a command-line flag is specified, it overrides the related
setting found in Consul.`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		initConfig(cmd)
		fmt.Fprintf(os.Stdout, "%s", doRender(cmd, args[0]))
	},
}

func init() {
	rootCmd.AddCommand(renderCmd)
	renderCmd.AddCommand(renderTemplateCmd)
	renderCmd.AddCommand(renderKVCmd)

	addConfigFlags(renderTemplateCmd)
	addTemplateFlags(renderTemplateCmd)

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
