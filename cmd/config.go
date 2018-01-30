package cmd

import (
	"fmt"
	"os"
	"strings"

	homedir "github.com/mitchellh/go-homedir"
	"github.com/spf13/viper"

	"github.com/bdclark/nomadctl/logging"
	consul "github.com/hashicorp/consul/api"
	"github.com/spf13/cobra"
)

func addConfigFlags(cmd *cobra.Command) {
	cmd.Flags().String("log-level", "INFO", "logging level")
	cmd.Flags().String("config", "", "config file to use (default is $HOME/.nomadctl.yaml)")
}

func initConfig(cmd *cobra.Command) {
	// setup logging level
	if level, _ := cmd.Flags().GetString("log-level"); level != "" {
		logging.SetLevel(level)
	}

	// set configuration defaults
	viper.SetDefault("prefix", "")
	viper.SetDefault("template", map[string]interface{}{
		"left_delimeter":       "{{",
		"right_delimeter":      "}}",
		"source":               "",
		"contents":             "",
		"error_on_missing_key": false,
		"options":              make(map[string]interface{}),
	})
	viper.SetDefault("deploy", map[string]interface{}{
		"auto_promote":      false,
		"force_count":       false,
		"plan":              false,
		"skip_confirmation": false,
	})
	viper.SetDefault("plan", map[string]interface{}{
		"no_color": false,
		"diff":     true,
		"verbose":  false,
	})

	// bind viper to command-line flags
	bindFlag(cmd, "prefix", "prefix")
	bindFlag(cmd, "template.left_delimeter", "left-delim")
	bindFlag(cmd, "template.right_delimeter", "right-delim")
	bindFlag(cmd, "template.error_on_missing_key", "err-missing-key")
	bindFlag(cmd, "deploy.auto_promote", "auto-promote")
	bindFlag(cmd, "deploy.force_count", "force-count")
	bindFlag(cmd, "deploy.plan", "plan")
	bindFlag(cmd, "deploy.skip_confirmation", "yes")
	bindFlag(cmd, "plan.no_color", "no-color")
	bindFlag(cmd, "plan.diff", "diff")
	bindFlag(cmd, "plan.verbose", "verbose")

	// bind viper to environment variables
	viper.SetEnvPrefix("nomadctl")
	viper.AutomaticEnv()
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	// read in config file
	cfgFile, _ := cmd.Flags().GetString("config")

	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		home, err := homedir.Dir()
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		viper.AddConfigPath(home)
		viper.SetConfigName(".nomadctl")
	}

	if err := viper.ReadInConfig(); err == nil {
		logging.Debug("using config file \"%s\"", viper.ConfigFileUsed())
	} else {
		logging.Debug("failed to read config file \"%s\": %v", viper.ConfigFileUsed(), err)
	}
}

// addConsulFlags adds consul related flags the given command
func addConsulFlags(cmd *cobra.Command) {
	cmd.Flags().String("prefix", "", "Consul KV prefix to combine with job key")
}

// addTemplateFlags adds template related flags to the given command
func addTemplateFlags(cmd *cobra.Command) {
	cmd.Flags().String("left-delim", "", "left-delimiter in template")
	cmd.Flags().String("right-delim", "", "right-delimiter in template")
	cmd.Flags().Bool("err-missing-key", false, "whether template should error on missing map key")
	cmd.Flags().StringSlice("option", []string{}, "template option for getter (can be supplied multiple times)")
}

// addDeployFlags adds deployment related flags to the given command
func addDeployFlags(cmd *cobra.Command) {
	addTemplateFlags(cmd)
	cmd.Flags().Bool("auto-promote", false, "automatically promote canary deployment")
	cmd.Flags().Bool("force-count", false, "force task group counts to match template")
	cmd.Flags().Bool("plan", false, "run job plan before deploying")
	cmd.Flags().Bool("yes", false, "skips asking for confirmation if plan changes found")
}

// addPlanFlags adds plan related flags to the given command
func addPlanFlags(cmd *cobra.Command) {
	addConfigFlags(cmd)
	addTemplateFlags(cmd)
	cmd.Flags().Bool("no-color", false, "disable colorized output")
	cmd.Flags().Bool("diff", true, "show diff between remote job and planned job")
	cmd.Flags().Bool("verbose", false, "verbose plan output")
}

// setConfigFromKV sets viper keys based on values in Consul, but only sets
// them if those viper keys are not being set with CLI flags
func setConfigFromKV(cmd *cobra.Command, client *consul.Client, jobKey string) error {
	if p := viper.GetString("prefix"); p != "" && strings.HasPrefix(jobKey, p) {
		logging.Warning("supplied job key \"%s\" contains configured prefix \"%s\", was this your intent?", jobKey, p)
	}

	prefix := canonicalizeJobKey(jobKey) + "/"

	pairs, _, err := client.KV().List(prefix, nil)
	if err != nil {
		return err
	}

	for _, pair := range pairs {
		key := pair.Key[len(prefix):]
		value := string(pair.Value)
		if value == "" {
			continue
		}

		switch key {
		case "template/source":
			setConfigFromKVHelper(cmd, "source", key, value)
		case "template/left_delimeter":
			setConfigFromKVHelper(cmd, "left-delim", key, value)
		case "template/right_delimeter":
			setConfigFromKVHelper(cmd, "left-delim", key, value)
		case "template/error_on_missing_key":
			setConfigFromKVHelper(cmd, "err-missing-key", key, value)
		case "deploy/auto_promote":
			setConfigFromKVHelper(cmd, "auto-promote", key, value)
		case "deploy/force_count":
			setConfigFromKVHelper(cmd, "force-count", key, value)
		}

		// getter options
		if strings.HasPrefix(key, "template/options/") {
			viperKey := strings.Replace(key, "/", ".", 2)
			logging.Debug("using template option from consul key %s", key)
			viper.Set(viperKey, value)
		}
	}

	// parsing template option flags here so they will override consul settings
	parseTemplateOptionFlags(cmd)

	return nil
}

// setsetConfigFromKVHelper is used by the setConfigFromKV function, and sets a
// viper key (from a consul key) if the given cli flag is not set for a command
func setConfigFromKVHelper(cmd *cobra.Command, flag, key string, value string) {
	viperKey := strings.Replace(key, "/", ".", 1)

	if f := cmd.Flags().Lookup(flag); f != nil && f.Changed {
		logging.Debug("ignoring consul key %s because %s flag set", key, flag)
		return
	}
	viper.Set(viperKey, value)
}

// bindFlag binds a command to a viper key only if that flag is associated
// with the given command
func bindFlag(cmd *cobra.Command, key string, flag string) {
	if f := cmd.Flags().Lookup(flag); f != nil {
		viper.BindPFlag(key, f)
	}
}

// canonicalizeJobKey returns the properly formatted full key name of a
// consul job key including the prefix
func canonicalizeJobKey(jobKey string) string {
	prefix := strings.TrimSuffix(viper.GetString("prefix"), "/")
	jobKey = strings.TrimSuffix(jobKey, "/")

	switch {
	case prefix == "" && jobKey == "":
		return ""
	case prefix == "":
		return jobKey
	default:
		return fmt.Sprintf("%s/%s", prefix, jobKey)
	}
}

// parseTemplateOptionFlags loops through "option" flag(s) provided
// and sets (overrides) them in the templation options map
func parseTemplateOptionFlags(cmd *cobra.Command) {
	if cmd.Flags().Lookup("option") == nil {
		return
	}

	options, _ := cmd.Flags().GetStringSlice("option")
	for _, option := range options {
		if !strings.Contains(option, "=") {
			usageError(cmd, fmt.Sprintf("option \"%s\" not in form of key=value", option))
		}

		parts := strings.SplitN(option, "=", 2)
		viperKey := fmt.Sprintf("template.options.%s", parts[0])

		if viper.GetString(viperKey) != "" {
			logging.Debug("template option \"%s\" previously set, overriding with cli flag", parts[0])
		}
		viper.Set(viperKey, parts[1])
	}
}
