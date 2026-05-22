# larm02

A terminal UI for [Prometheus Alertmanager](https://prometheus.io/docs/alerting/latest/alertmanager/), inspired by [karma](https://github.com/prymitive/karma) and [k9s](https://github.com/derailed/k9s).

Browse and filter active alerts from one or more Alertmanager instances without leaving the terminal.

## Installation

Download a pre-built binary from the [releases page](https://github.com/yeniklas/larm02/releases), or install from source:

```sh
go install github.com/yeniklas/larm02/cmd/larm02@latest
```

## Configuration

Create `~/.config/larm02/config.yaml`:

```yaml
alertmanagers:
  - name: production
    url: http://alertmanager.prod:9093
  - name: staging
    url: http://alertmanager.staging:9093
refresh_interval: 30s
```

Pass a different config file with `--config`:

```sh
larm02 --config /path/to/config.yaml
```

| Field | Default | Description |
|---|---|---|
| `alertmanagers` | — | List of Alertmanager instances to query |
| `alertmanagers[].name` | — | Display name shown in the header |
| `alertmanagers[].url` | — | Base URL of the Alertmanager instance |
| `refresh_interval` | `30s` | How often to poll for new alerts |
| `healthchecks` | — | Named watchdog filter sets (see below) |
| `acknowledgement.duration` | `15m` | How long the silence lasts |
| `acknowledgement.author` | `larm02` | `createdBy` field on the silence |
| `acknowledgement.comment` | `ACK! … on %NOW%` | Comment template; `%NOW%` is replaced with the current UTC time |

### Healthchecks (watchdog alerts)

`healthchecks` is a map from a display name to a list of filters. larm02 expects at least one active alert matching **all** filters in each set. If a set has no matches, a warning is shown in the UI. Alerts matched by a healthcheck are hidden from the main alert list.

This is designed for use with a [Dead Man's Switch](https://en.wikipedia.org/wiki/Dead_man%27s_switch) alert that fires constantly to confirm the alerting pipeline is alive.

```yaml
healthchecks:
  watchdog:
    - alertname=Watchdog
  infra-watchdog:
    - alertname=Watchdog
    - severity=none
```

## Usage

```sh
larm02                              # launch with default config
larm02 --config /path/to/config    # use a custom config file
larm02 --version                   # print version and exit
larm02 --self-update                # update to the latest release
```

## Keybindings

| Key | Action |
|---|---|
| `j` / `↓` | Move down |
| `k` / `↑` | Move up |
| `Enter` | Open alert detail |
| `ESC` | Close detail / clear filter |
| `/` | Filter alerts |
| `:` | Command mode |
| `a` | Acknowledge selected alert |
| `r` | Refresh now |
| `?` | Toggle help |
| `q` / `Ctrl+C` | Quit |

### Filtering

Press `/` to enter filter mode. Filters support:

- `key=value` — exact label match (e.g. `severity=critical`)
- `key=~value` — substring match (e.g. `alertname=~cpu`)
- plain text — substring match against alertname and instance name

Press `Enter` to apply or `ESC` to cancel. Press `ESC` again from normal mode to clear an active filter.

### Command mode

Press `:` to enter a command:

| Command | Action |
|---|---|
| `alerts` | Return to the alerts view |
| `quit` / `q` | Quit |

### Acknowledgement

Press `a` on any alert (in the list or in detail view) to acknowledge it. larm02 posts a short-lived silence to the Alertmanager instance the alert came from, matching all of the alert's labels exactly. The silence is created with the configured `author` and `comment`.

```yaml
acknowledgement:
  duration: 15m
  author: your-name
  comment: "ACK! Acknowledged on %NOW%"
```

This pairs well with [kthxbye](https://github.com/prymitive/kthxbye), which automatically extends the silence while the alert keeps firing and lets it expire once the alert resolves.

## Multi-instance support

larm02 queries all configured Alertmanager instances concurrently. Alerts are merged by fingerprint and tagged with their source instance name in the table. Errors from individual instances are shown inline without hiding alerts from healthy instances.

## License

[GPL-3.0](LICENSE)
