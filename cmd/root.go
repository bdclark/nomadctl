package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/bdclark/nomadctl/logging"
	"github.com/bdclark/nomadctl/version"
	homedir "github.com/mitchellh/go-homedir"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// rootCmd is the main entrypoint into the CLI
var rootCmd = &cobra.Command{
	Use:     "nomadctl",
	Short:   "Manage Nomad",
	Version: version.Get(true),
}

// Execute is called from main and executes the rootCmd
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func bail(err error, code int) {
	log.Fatal(err)
	os.Exit(code)
}

func usageError(cmd *cobra.Command, message string, codeOptional ...int) {
	code := 1
	if len(codeOptional) == 1 {
		code = codeOptional[0]
	}

	cmd.Usage()

	if message != "" {
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, message)
	}

	os.Exit(code)
}

func init() {
	cobra.OnInitialize(initConfig)

	rootCmd.PersistentFlags().String("log-level", "INFO", "logging level")
	rootCmd.PersistentFlags().String("config", "", "config file to use (default is $HOME/.nomadctl.yaml)")
}

func initConfig() {
	// setup logging level
	if level, _ := rootCmd.Flags().GetString("log-level"); level != "" {
		logging.SetLevel(level)
	}

	// set configuration defaults
	setViperDefaults()

	// bind viper to environment variables
	viper.SetEnvPrefix("nomadctl")
	viper.AutomaticEnv()
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	// read in config file
	cfgFile, _ := rootCmd.Flags().GetString("config")

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
