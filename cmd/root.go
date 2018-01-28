package cmd

import (
	"fmt"
	"os"

	"github.com/bdclark/nomadctl/version"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

// rootCmd is the main entrypoint into the CLI
var rootCmd = &cobra.Command{
	Use:   "nomadctl",
	Short: "Nomadctl is a utility to help manage Nomad.",
	Long: `Nomadctl is a utility to help manage Nomad.

Nomad client settings must be configured via the standard Nomad
environment variables:

NOMAD_ADDR: The address of the Nomad server, default: http://127.0.0.1:4646.
NOMAD_REGION: The region of the Nomad server to forward commands to.
NOMAD_CACERT: Path to CA cert file to verify Nomad server SSL cert.
NOMAD_CAPATH: Path to directory of CA cert files to verify server SSL cert.
NOMAD_CLIENT_CERT: Path to client cert for TLS authentication to Nomad.
NOMAD_CLIENT_KEY: Path to an private key matching the client cert.
NOMAD_SKIP_VERIFY: Do not verify TLS certificate (not recommended).
NOMAD_TOKEN: The ACL token to use to authenticate API requests.

For any of the "kv" subcommands that query Consul, client settings must
be configured via the standard Consul environment variables. See
https://www.consul.io/docs/commands/index.html#environment-variables.`,
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
