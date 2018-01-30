# nomadctl

Nomadctl is a utility to help manage Nomad. It can render templates using
Consul-Template, manage deployments (including automatic promotion of canary
deployments), restart jobs and task groups, and a few other utility functions.

## Installation
There's a few ways to download and install nomadctl:

#### Using Homebrew
```
brew install bdclark/tap/nomadctl
```

#### Using Go Get
```
go get github.com/bdclark/nomadctl
```

#### Manually
Download your desired flavor from the [GitHub releases][1] page and install manually.

## Commands and Usage
Use `nomadctl help` and `nomadctl help <command>` for help and usage. The following
commands are currently available.

* `render (template|kv)` - Render a template to stdout, either specified locally (`template`) or using configuration specified in Consul (`kv`).
* `plan (template|kv)` - Plan a job from a template specified locally (`template`) or using configuration specified in Consul (`kv`).
* `deploy (template|kv)` - Deploy a job, either with template and deploy options specified locally (`template`) or using configuration specified in Consul (`kv`).
* `drain` - Drain a client node and wait until done.
* `gc` - Force cluster garbage collection.
* `kv list` - List jobs stored in Consul.
* `kv set` - Set a job-specific Consul key.
* `re-eval` - Re-evaluate a job or all jobs.
* `redeploy` - Re-deploy a job, causing a "rolling restart".
* `restart` - Restart a job or task group.
* `scale (up|down|set|get)` - Scale a task group up or down.

## Configuration
Options for each nomadctl command can be supplied via command-line flag.
However, some commands allow options to be set or defaulted via configuration
file, environment variable or Consul keys.

### Configuration File
Default settings used across multiple jobs/commands can be set in a config file.
Supported formats are JSON, TOML, YAML, and HCL. By default, nomadctl looks
for the file `$HOME/.nomadctl.(yaml|yml|json|hcl)`, however a specific file can be
specified with the `--config` flag.

Below is an example config file showing application defaults:

```yaml
# ~/.nomadctl.yaml

# prefix is used in various "kv" sub-commands where a JOBKEY is required.
# If prefix is set, the supplied JOBKEY becomes ${PREFIX}/${JOBKEY}.
prefix: ""

# render, plan, deploy, and redeploy commands use these settings
template:
  left_delimeter: '{{'
  right_delimeter: '}}'
  error_on_missing_key: false
  options: {}

# deploy and redeploy commands use these settings
deploy:
  auto_promote: false
  force_count: false
  plan: false
  skip_confirmation: false

# the plan command uses these settings
plan:
  no_color: false
  diff: true
  quiet: false
  verbose: false
```

### Environment Variables
Any of the config settings can also be set via environment variable using the
pattern `NOMADCTL_<KEY>`, where `<KEY>` is the upper-cased config file key
shown above with `.` replaced with `_`. For example, to set the left and right
template delimeters via environment variables, you could use:

```shell
export NOMADCTL_TEMPLATE_LEFT_DELIMETER="[["
export NOMADCTL_TEMPLATE_RIGHT_DELIMETER="]]"
```

### Consul Keys
The following Consul keys are supported and are equivalent to their related
config file settings, but are job-specific:

```
${JOBKEY}/template/left_delimeter
${JOBKEY}/template/right_delimeter
${JOBKEY}/template/error_on_missing_key
${JOBKEY}/template/options/*
${JOBKEY}/deploy/auto_promote
${JOBKEY}/deploy/force_count
${JOBKEY}/deploy/plan
${JOBKEY}/deploy/skip_confirmation
```

### Configuration Precedence
Nomadctl uses the following precedence order when evaluating config settings.
Each item takes precedence over the item below it:

* Command-line flag
* Consul key/value
* Environment variable
* Configuration file
* Application default

## Nomad and Consul Client Configuration
Nomad client settings cannot be set via command-line flags, but must instead be
configured via the standard Nomad environment variables:

* `NOMAD_ADDR` - The address of the Nomad server, default: http://127.0.0.1:4646.
* `NOMAD_REGION` - The region of the Nomad server to forward commands to.
* `NOMAD_CACERT` - Path to CA cert file to verify Nomad server SSL cert.
* `NOMAD_CAPATH` - Path to directory of CA cert files to verify server SSL cert.
* `NOMAD_CLIENT_CERT` - Path to client cert for TLS authentication to Nomad.
* `NOMAD_CLIENT_KEY` - Path to an private key matching the client cert.
* `NOMAD_SKIP_VERIFY` - Do not verify TLS certificate (not recommended).
* `NOMAD_TOKEN` - The ACL token to use to authenticate API requests.

For any of the "kv" sub-commands that query Consul, client settings must
be configured via the standard Consul environment variables. See the
[Consul docs][2] for details.


[1]:https://github.com/bdclark/nomadctl/releases
[2]:https://www.consul.io/docs/commands/index.html#environment-variables
