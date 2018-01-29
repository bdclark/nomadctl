package cmd

import (
	"fmt"
	"os"
	"strings"
	"text/template"

	"github.com/pkg/errors"
	"github.com/spf13/viper"

	consul "github.com/hashicorp/consul/api"
	"github.com/spf13/cobra"
)

var kvCmd = &cobra.Command{
	Use:   "kv",
	Short: "commands for interacting with Consul",
}

var kvListCmd = &cobra.Command{
	Use:   "list [PREFIX]",
	Short: "list jobs stored in Consul",
	Args:  cobra.MaximumNArgs(1),
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

		t, _ := cmd.Flags().GetString("template")
		if t == "" {
			t = "{{ .Key }}"
		}

		tmpl, err := template.New("test").Parse(t + "\n")
		for _, v := range pairs {
			tmpl.Execute(os.Stdout, v)
		}

	},
}

func init() {
	rootCmd.AddCommand(kvCmd)
	kvCmd.AddCommand(kvListCmd)

	kvListCmd.Flags().String("template", "", "format and display allocation using a Go template")
}

// explode is used to expand a list of keypairs into a deeply-nested hash.
func explode(pairs *consul.KVPairs, prefix string) (map[string]interface{}, error) {
	m := make(map[string]interface{})
	for _, pair := range *pairs {
		key := strings.TrimPrefix(pair.Key, prefix)
		if err := explodeHelper(m, key, string(pair.Value[:]), key); err != nil {
			return nil, errors.Wrap(err, "explode")
		}
	}
	return m, nil
}

// explodeHelper is a recursive helper for explode.
func explodeHelper(m map[string]interface{}, k, v, p string) error {
	if strings.Contains(k, "/") {
		parts := strings.Split(k, "/")
		top := parts[0]
		key := strings.Join(parts[1:], "/")

		if _, ok := m[top]; !ok {
			m[top] = make(map[string]interface{})
		}
		nest, ok := m[top].(map[string]interface{})
		if !ok {
			return fmt.Errorf("not a map: %q: %q already has value %q", p, top, m[top])
		}
		return explodeHelper(nest, key, v, k)
	}

	if k != "" {
		m[k] = v
	}

	return nil
}
