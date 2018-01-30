package template

import (
	"fmt"
	"io/ioutil"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/bdclark/nomadctl/logging"
	getter "github.com/hashicorp/go-getter"
	"github.com/pkg/errors"
)

// getterArtifact represents an artifact to retrieve using getter
type getterArtifact struct {
	source  string
	options map[string]string
	path    string
}

// newGetterArtifact returns a new getterArtifact
func newGetterArtifact(source string, options map[string]string) *getterArtifact {

	g := &getterArtifact{
		source:  source,
		options: make(map[string]string),
	}

	for k, v := range options {
		// sanitize by only adding supported options
		switch k {
		case "checksum", "archive", "ref", "rev", "sshkey",
			"aws_access_key_id", "aws_access_key_secret", "aws_access_token",
			"region", "version":
			g.options[k] = v
		case "path":
			g.path = v
		default:
			logging.Debug("not adding \"%s\" to getter options", k)
		}
	}
	return g
}

// getURL builds a formatted URL for getter to use
func (g *getterArtifact) getURL() (string, error) {
	u, err := url.Parse(g.source)
	if err != nil {
		return "", errors.Wrap(err, fmt.Sprintf("failed to parse source URL %q", g.source))
	}

	q := u.Query()
	for k, v := range g.options {
		q.Add(k, v)
	}
	u.RawQuery = q.Encode()
	return u.String(), nil
}

// isSimpleLocalFile determines if the source is an uncompressed local file.
func (g *getterArtifact) isSimpleLocalFile() (bool, error) {
	// if archive option exists, assume it needs decompressed
	if _, ok := g.options["archive"]; ok {
		return false, nil
	}

	// determine if extension matches a getter Decompressor
	extensions := reflect.ValueOf(getter.Decompressors).MapKeys()
	for _, ext := range extensions {
		if strings.HasSuffix(g.source, fmt.Sprintf(".%s", ext)) {
			logging.Debug("template source has compressed extension %s", ext)
			return false, nil
		}
	}

	// get working directory
	pwd, err := os.Getwd()
	if err != nil {
		return false, errors.Wrap(err, "failed to get working directory")
	}

	// get "detected" source
	detectedSrc, err := getter.Detect(g.source, pwd, getter.Detectors)
	if err != nil {
		return false, errors.Wrap(err, "getter detect")
	}
	logging.Debug("detected source %s", detectedSrc)

	if strings.HasPrefix(detectedSrc, "file://") {
		if stat, err := os.Stat(g.source); err != nil {
			return false, err
		} else if stat.IsDir() {
			logging.Debug("detected source is a directory")
			return false, nil
		}
		return true, nil
	}

	return false, nil
}

// getContents uses getter to retrieve a file's contents.
// If the source contains anything other than a single file,
// path option must be specified to read the correct file within source.
func (g *getterArtifact) getContents() (string, error) {
	// create temp directory for getter destination
	dir, err := ioutil.TempDir("", "nomadctl")
	if err != nil {
		return "", errors.Wrap(err, "temp dir")
	}
	defer os.RemoveAll(dir)

	// HACK: update dir to be a non-existent sub-dir so getter doesn't fail
	dir = filepath.Join(dir, "dest")

	// get working directory
	pwd, err := os.Getwd()
	if err != nil {
		return "", errors.Wrap(err, "failed to get working directory")
	}

	url, err := g.getURL()
	if err != nil {
		return "", err
	}

	// build getter client, using dir mode in the event the retreived
	// source has multiple files
	client := &getter.Client{
		Src:  url,
		Dst:  dir,
		Pwd:  pwd,
		Mode: getter.ClientModeDir,
	}

	// get the source into the destination folder
	err = client.Get()
	if err != nil {
		return "", errors.Wrap(err, "getter")
	}

	// if path is not set and only one file exists in the retrieved source,
	// then use it.  Otherwise fail.
	if g.path == "" {
		files, err := ioutil.ReadDir(dir)
		if err != nil {
			return "", fmt.Errorf("failed to read temp dir")
		}

		if len(files) == 1 && !files[0].IsDir() {
			fname := files[0].Name()
			if b, err := ioutil.ReadFile(fname); err == nil {
				return string(b[:]), nil
			}
			return "", fmt.Errorf("retrieved \"%s\", failed to read \"%s\"", g.source, fname)
		}
		return "", fmt.Errorf("path required to read file from \"%s\"", g.source)
	}

	// read given path from within the retrieved source
	data, err := ioutil.ReadFile(filepath.Join(dir, g.path))
	if err != nil {
		return "", fmt.Errorf("retrieved \"%s\", failed to read \"%s\"", g.source, g.path)
	}
	return string(data[:]), nil
}
