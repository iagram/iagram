# Telemetry

iagram sends a few anonymous usage events so the project can tell whether it is
being used at all. This page is the complete description; the implementation is
one file, [`internal/telemetry/telemetry.go`](../internal/telemetry/telemetry.go),
and its tests assert that nothing outside the list below can be sent.

## Turning it off

Any one of these disables it:

```
iagram telemetry off          # persisted in ~/.iagram/config.json
IAGRAM_TELEMETRY=0            # environment
DO_NOT_TRACK=1                # https://consoledonottrack.com
CI=true                       # never in CI
```

`iagram telemetry status` shows the current state. The first time `iagram up`
runs it prints a notice pointing here.

## What is sent

One JSON object per event to `https://telemetry.iagram.dev/v1/events`:

| Field | Value |
|---|---|
| `event` | `command` |
| `install_id` | Random 128-bit id generated on first run, stored in `~/.iagram/config.json`. Not derived from the machine or the user. Delete the file to rotate it. |
| `version` | iagram version |
| `os`, `arch` | e.g. `darwin`, `arm64` |
| `time` | UTC timestamp |
| `props.command` | `init`, `up`, `validate`, `generate`, `plan`, `apply` |
| `props.nodes`, `props.edges` | Bucketed counts: `0`, `1-5`, `6-20`, `21-100`, `100+` |
| `props.providers` | Provider names used, e.g. `aws` |
| `props.outcome` | `ok`, `error`, `cancelled` |
| `props.duration_s` | Rounded seconds |
| `props.changes` | Plan only: bucketed add+change+destroy count |

## What is never sent

Diagram names, node names, node ids, property values, account ids, regions,
CIDRs, file paths, hostnames, IP addresses, usernames, environment variables,
Terraform output, error messages. Property keys are whitelisted in code;
anything else is dropped before serialisation.

Sending is asynchronous with a two-second timeout and can never change the
result of a command.
