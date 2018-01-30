package template

import (
	"io/ioutil"
	"log"
	"os"

	"github.com/bdclark/nomadctl/logging"
	ctConfig "github.com/hashicorp/consul-template/config"
	ctManager "github.com/hashicorp/consul-template/manager"
	"github.com/hashicorp/logutils"
	"github.com/pkg/errors"
)

// Template is the internal representation of a template
type Template struct {
	source        string
	contents      string
	leftDelim     string
	rightDelim    string
	errMissingKey bool
}

// NewTemplateInput represents the input to a new Template
type NewTemplateInput struct {
	Source        string
	Contents      string
	LeftDelim     string
	RightDelim    string
	ErrMissingKey bool
	Options       map[string]string
}

// NewTemplate generates a new template
func NewTemplate(i *NewTemplateInput) (*Template, error) {
	if i == nil {
		i = &NewTemplateInput{}
	}

	var t Template
	t.contents = i.Contents
	t.leftDelim = i.LeftDelim
	t.rightDelim = i.RightDelim
	t.errMissingKey = i.ErrMissingKey

	if i.Source != "" {
		artifact := newGetterArtifact(i.Source, i.Options)
		isLocal, err := artifact.isSimpleLocalFile()
		if err != nil {
			return nil, errors.Wrap(err, "failed to determine if source is remote")
		}
		if isLocal {
			t.source = i.Source
		} else {
			contents, err := artifact.getContents()
			if err != nil {
				return nil, errors.Wrap(err, "failed to get contents")
			}
			t.contents = contents
		}
	}

	if t.leftDelim == "" {
		t.leftDelim = "{{"
	}
	if t.rightDelim == "" {
		t.rightDelim = "}}"
	}

	return &t, nil
}

// Render renders the template using Consul-Template
func (t *Template) Render() ([]byte, error) {

	config := &ctConfig.Config{
		Templates: &ctConfig.TemplateConfigs{
			&ctConfig.TemplateConfig{
				Source:        ctConfig.String(t.source),
				Contents:      ctConfig.String(t.contents),
				LeftDelim:     ctConfig.String(t.leftDelim),
				RightDelim:    ctConfig.String(t.rightDelim),
				ErrMissingKey: ctConfig.Bool(t.errMissingKey),
			},
		},
	}

	if logging.IsDebug() {
		filter := &logutils.LevelFilter{
			Levels:   []logutils.LogLevel{"TRACE", "DEBUG", "INFO", "WARN", "ERR"},
			MinLevel: logutils.LogLevel("DEBUG"),
			Writer:   os.Stderr,
		}
		log.SetFlags(log.Ldate | log.Ltime)
		log.SetOutput(filter)
	} else {
		// disable trace/debug logging in consul-template rendering
		log.SetFlags(0)
		log.SetOutput(ioutil.Discard)
	}

	r, err := ctManager.NewRunner(config, true, true)
	if err != nil {
		return nil, err
	}

	// Discard runner output
	r.SetOutStream(ioutil.Discard)
	r.SetErrStream(ioutil.Discard)

	go r.Start()

	for {
		select {
		case <-r.DoneCh:
			for _, k := range r.RenderEvents() {
				return k.Contents, nil
			}
		case err, _ = <-r.ErrCh:
			return nil, err
		}
	}
}
