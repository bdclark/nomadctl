package cmd

import (
	"fmt"
	"os"
	"strings"
	"text/template"

	"github.com/bdclark/nomadctl/logging"
	"github.com/spf13/viper"

	consul "github.com/hashicorp/consul/api"
	"github.com/spf13/cobra"
)

// kvCmd represents the base "kv" command
var kvCmd = &cobra.Command{
	Use:   "kv",
	Short: "commands for interacting with Consul",
}

// kvListCmd represents the "kv list" command
var kvListCmd = &cobra.Command{
	Use:   "list [PREFIX]",
	Short: "list jobs stored in Consul",
	Long: `Lists jobs stored in Consul based on specified prefix.

The list command expects job configs to be stored in Consul in the
format "${PREFIX}/${JOB}/*". The prefix must be supplied as an argument
if not set via configuration file or environment variable.

By default, the list command will list job names only, but if the format
flag is used with a template string, other settings within the job's
keyspace can be displayed. The job name is {{ .Key }} and the job's config
is {{ .Value }}, so "{{ .Job }} {{ .Value.deploy.auto_promote }}\n" would
display the job name and auto promotion setting for all jobs at PREFIX.`,
	Args: cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		initConfig(cmd)

		type jobKeyPair struct {
			Key   string
			Value interface{}
		}

		prefix := viper.GetString("prefix")
		if len(args) == 1 && args[0] != "" {
			prefix = args[0]
		}
		if prefix == "" {
			usageError(cmd, "prefix is required")
		}
		if !strings.HasSuffix(prefix, "/") {
			prefix = prefix + "/"
		}

		client, err := consul.NewClient(consul.DefaultConfig())
		if err != nil {
			bail(err, 1)
		}

		list, _, err := client.KV().List(prefix, nil)
		if err != nil {
			bail(err, 1)
		}

		kvMap, err := explode(&list, prefix)
		if err != nil {
			bail(err, 1)
		}

		pairs := make([]*jobKeyPair, 0, len(kvMap))
		for k, v := range kvMap {
			pairs = append(pairs, &jobKeyPair{
				Key:   k,
				Value: v,
			})
		}

		t, _ := cmd.Flags().GetString("format")
		if t == "" {
			t = "{{ .Key }}"
		}

		tmpl, err := template.New("test").Parse(t + "\n")
		for _, v := range pairs {
			tmpl.Execute(os.Stdout, v)
		}

	},
}

// kvSetCmd represents the "kv set" command
var kvSetCmd = &cobra.Command{
	Use:   "set JOBKEY SUBKEY VALUE",
	Short: "set job related key in Consul",
	Long: `Sets a Consul key to the specified value, with the KV path being
an (optional) prefix, a job-key and a sub-key.

The required JOBKEY argument is a Consul KV path and is expected to have one
or more sub-keys. If a "prefix" is specified via command-line flag, config
file setting or environment variable, the the actual JOBKEY becomes
"${PREFIX}/${JOBKEY}".

The required SUBKEY argument is combined with the JOBKEY (and optional prefix)
to form a full Consul KV path ${PREFIX}/${JOBKEY}/${SUBKEY}.

This command makes the most sense when a prefix is configured as a default.
For example, if the prefix "nomad/jobs" is set in a configuration file, then
"nomadctl kv set myjob deploy/auto_promotion true" will write to the kv path
"nomad/jobs/myjob/deploy/auto_promotion".`,
	Args: cobra.ExactArgs(3),
	Run: func(cmd *cobra.Command, args []string) {
		initConfig(cmd)

		key := fmt.Sprintf("%s/%s", canonicalizeJobKey(args[0]), args[1])
		value := args[2]

		force, _ := cmd.Flags().GetBool("yes")
		if !force {
			yes := askForConfirmation(fmt.Sprintf("Are you sure you want to set: %s?", key))
			if !yes {
				fmt.Fprintln(os.Stderr, "No changes made.")
				os.Exit(0)
			}
		}

		logging.Debug("writing value \"%\" to key \"%s\"", value, key)

		client, err := consul.NewClient(consul.DefaultConfig())
		if err != nil {
			bail(err, 1)
		}

		_, err = client.KV().Put(&consul.KVPair{Key: key, Value: []byte(value)}, nil)
		if err != nil {
			bail(err, 1)
		}
		fmt.Fprintln(os.Stderr, fmt.Sprintf("Successfully wrote to: %s", key))
	},
}

func init() {
	rootCmd.AddCommand(kvCmd)
	kvCmd.AddCommand(kvListCmd)
	kvCmd.AddCommand(kvSetCmd)

	addConfigFlags(kvListCmd)
	kvListCmd.Flags().String("format", "", "format and display jobs using a Go template")

	addConfigFlags(kvSetCmd)
	addConsulFlags(kvSetCmd)
	kvSetCmd.Flags().Bool("yes", false, "skips asking for confirmation")
}
