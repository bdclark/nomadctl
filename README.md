# nomadctl

Nomadctl is a utility to help manage Nomad. It can render templates using
Consul-Template, manage deployments (including automatic promotion of canary
deployments), restart jobs and task groups, and a few other utility functions.

## Commands
TODO: Add command documentation. In the meantime, see `nomadctl --help`.

## Configuration
Nomadctl supports the configuration settings shown below. Not all settings are
applicable to every command.  See the help (`--help` or `-h`) for the specific
command to understand the setting's purpose.

Config file                     | Supported Commands    | Consul Key (set per job)                  | CLI flag (override) | Default
--------------------------------|-------------------------------------------|-------------------------------------------|---------------------|--------
`template.left_delimeter`       | render, deploy, plan  | `<job_key>/template/left_delimeter`       | `--left-delim`      | `{{`
`template.right_delimeter`      | render, deploy, plan  | `<job_key>/template/right_delimeter`      | `--right-delim`     | `}}`
`template.error_on_missing_key` | render, deploy, plan  | `<job_key>/template/error_on_missing_key` | `--err-missing-key` | `false`
`deploy.auto_promote`           | deploy                | `<job_key>/deploy/auto_promote`           | `--auto-promote`    | `false`
`deploy.force_count`            | deploy                | `<job_key>/deploy/force_count`            | `--force-count`     | `false`
`deploy.plan`                   | deploy                | `<job_key>/deploy/plan`                   | `--plan`            | `false`
`deploy.skip_confirmation`      | deploy                | `<job_key>/deploy/skip_confirmation`      | `--yes`             | `false`
`plan.no_color`                 | plan, deploy          | _not applicable_                          | `--no-color`        | `false`
`plan.diff`                     | plan, deploy          | _not applicable_                          | `--diff`            | `true`
`plan.verbose`                  | plan, deploy          | _not applicable_                          | `--verbose`         | `false`

### Environment Variables
Any of the config settings can also be set via environment variable using the
pattern `NOMADCTL_<KEY>`, where `<KEY>` is the uppercased config file key shown
above with `.` replaced `_`. For example, to set the left and right template
delimeters via environment variables, you could use:

```shell
export NOMADCTL_TEMPLATE_LEFT_DELIMETER="[["
export NOMADCTL_TEMPLATE_RIGHT_DELIMETER="]]"
```

### Config File
Default settings used across multiple jobs/commands can be set in a config file.
Supported formats are JSON, TOML, YAML, and HCL. By default, nomadctl looks
for the file `$HOME/.nomadctl.(yaml|yml|json|hcl)`, however a specific file can be
specified with the `--config` flag.

Below is an example config file showing application defaults:

```yaml
# ~/.nomadctl.yaml
prefix: ""
template:
  left_delimeter: "{{"
  right_delimeter: "}}"
  error_on_missing_key: false
deploy:
  auto_promote: false
  force_count: false
  plan: false
  skip_confirmation: false
plan:
  no_color: false
  diff: true
  verbose: false
```

### Configuration Precedence
Nomadctl uses the following precedence order when evaluating config settings.
Each item takes precedence over the item below it:

* CLI flag
* Consul key/value
* Environment variable
* Config file
* Application default
