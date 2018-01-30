package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	consul "github.com/hashicorp/consul/api"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
)

// askForConfirmation presents a message as a yes/no question
// and returns true if the response is yes
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

// explode is used to expand a list of keypairs into a deeply-nested hash.
func explode(pairs *consul.KVPairs, prefix string) (map[string]interface{}, error) {
	m := make(map[string]interface{})
	for _, pair := range *pairs {
		key := strings.TrimPrefix(pair.Key, prefix)
		if err := explodeHelper(m, key, string(pair.Value), key); err != nil {
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
