package cmd

import (
	"fmt"
	"os"
	"strings"
	"text/template"

	"github.com/bdclark/nomadctl/logging"
	consul "github.com/hashicorp/consul/api"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// kvCmd represents the base "kv" command
var kvCmd = &cobra.Command{
	Use:   "kv",
	Short: "Interact with Consul KV store",
}

// kvListCmd represents the "kv list" command
var kvListCmd = &cobra.Command{
	Use:   "list [PREFIX]",
	Short: "List jobs stored in Consul",
	Long: `Lists jobs stored in Consul at the specified PREFIX.

If the prefix is not specified as an argument, it is required to be set via
configuration file or environment variable.

The optional format flag allows additional configuration within each job's
configuration to be displayed. Use "{{ .Key }}" for the job name and
"{{ .Value }}" for the job's config. For example, the format string
"{{ .Key }} {{ .Value.deploy.auto_promote }}" would display the auto-
promotion setting for every job.`,
	Args: cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		initConfig(cmd)

		// set prefix
		prefix := viper.GetString("prefix")
		if len(args) == 1 && args[0] != "" {
			prefix = args[0]
		}
		if !strings.HasSuffix(prefix, "/") {
			prefix += "/"
		}

		// get KV list from Consul
		client, err := consul.NewClient(consul.DefaultConfig())
		if err != nil {
			bail(err, 1)
		}
		list, _, err := client.KV().List(prefix, nil)
		if err != nil {
			bail(err, 1)
		}

		// explode KV list into a nested map
		kvMap, err := explode(&list, prefix)
		if err != nil {
			bail(err, 1)
		}

		type jobKeyPair struct {
			Key   string
			Value interface{}
		}

		// convert the map to slice of structs for templating
		pairs := make([]jobKeyPair, 0, len(kvMap))
		for k, v := range kvMap {
			pairs = append(pairs, jobKeyPair{Key: k, Value: v})
		}

		format, _ := cmd.Flags().GetString("format")
		if format == "" {
			format = "{{ .Key }}"
		}

		// iterate over jobs using template
		tmpl, err := template.New("list").Parse(format + "\n")
		if err != nil {
			bail(err, 1)
		}
		for _, pair := range pairs {
			tmpl.Execute(os.Stdout, pair)
		}
	},
}

// kvSetCmd represents the "kv set" command
var kvSetCmd = &cobra.Command{
	Use:   "set JOBKEY SUBKEY VALUE",
	Short: "Set a job-related key in Consul",
	Long: `A convenience utility to set a job-related key in Consul without requiring
the consul binary or other external tools.

The required JOBKEY argument is a Consul KV path and is expected to have one
or more sub-keys. If a "prefix" is specified via command-line flag, config
file setting or environment variable, the the actual JOBKEY becomes
"${PREFIX}/${JOBKEY}".

The required SUBKEY argument is combined with the JOBKEY (and optional
prefix) to form the complete KV path ${PREFIX}/${JOBKEY}/${SUBKEY}.

The set command is most useful when a prefix is set as a default. For
example, "prefix" is set to "nomad/jobs" in a config file, then
"nomadctl kv set myjob deploy/auto_promote true" would write to the
key "nomad/jobs/myjob/deploy/auto_promote".`,
	Args: cobra.ExactArgs(3),
	Run: func(cmd *cobra.Command, args []string) {
		initConfig(cmd)

		subkey := strings.TrimPrefix(args[1], "/")
		key := fmt.Sprintf("%s/%s", canonicalizeJobKey(args[0]), subkey)
		value := args[2]

		force, _ := cmd.Flags().GetBool("yes")
		if !force {
			if yes := askForConfirmation(fmt.Sprintf("OK to write to \"%s\"?", key)); !yes {
				fmt.Fprintln(os.Stderr, "No changes made.")
				os.Exit(0)
			}
		}

		logging.Debug("writing value \"%s\" to key \"%s\"", value, key)

		client, err := consul.NewClient(consul.DefaultConfig())
		if err != nil {
			bail(err, 1)
		}

		pair := &consul.KVPair{Key: key, Value: []byte(value)}
		_, err = client.KV().Put(pair, nil)
		if err != nil {
			bail(err, 1)
		}

		fmt.Fprintf(os.Stderr, "Successfully wrote to \"%s\".\n", key)
	},
}

var kvGetCmd = &cobra.Command{
	Use:   "get JOBKEY SUBKEY",
	Short: "Get a job-related key in Consul",
	Long: `A convenience utility to get a job-related key in Consul without requiring
the consul binary or other external tools.

The required JOBKEY argument is a Consul KV path and is expected to have one
or more sub-keys. If a "prefix" is specified via command-line flag, config
file setting or environment variable, the the actual JOBKEY becomes
"${PREFIX}/${JOBKEY}".

The required SUBKEY argument is combined with the JOBKEY (and optional
prefix) to form the complete KV path ${PREFIX}/${JOBKEY}/${SUBKEY}.

The get command is most useful when a prefix is set as a default. For
example, "prefix" is set to "nomad/jobs" in a config file, then
"nomadctl kv get myjob deploy/auto_promote" would get the value of
key "nomad/jobs/myjob/deploy/auto_promote".`,
	Args: cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		initConfig(cmd)

		subkey := strings.TrimPrefix(args[1], "/")
		key := fmt.Sprintf("%s/%s", canonicalizeJobKey(args[0]), subkey)

		client, err := consul.NewClient(consul.DefaultConfig())
		if err != nil {
			bail(err, 1)
		}

		kv, _, err := client.KV().Get(key, nil)
		if err != nil {
			bail(err, 1)
		}
		if kv == nil {
			fmt.Fprintf(os.Stderr, "No key exists at: %s\n", key)
			os.Exit(1)
		}

		fmt.Println(string(kv.Value))
	},
}

func init() {
	rootCmd.AddCommand(kvCmd)
	kvCmd.AddCommand(kvListCmd)
	kvCmd.AddCommand(kvSetCmd)
	kvCmd.AddCommand(kvGetCmd)

	addConfigFlags(kvListCmd)
	kvListCmd.Flags().String("format", "", "format job list with Go template")

	addConfigFlags(kvSetCmd)
	addConsulFlags(kvSetCmd)
	kvSetCmd.Flags().Bool("yes", false, "skips asking for confirmation")

	addConsulFlags(kvGetCmd)
}
