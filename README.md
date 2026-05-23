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
    hidden: true          # start hidden; still counted in header
refresh_interval: 30s
group_labels:
  - "@instance"   # virtual label: resolves to the AM instance name
  - severity
columns:
  - label: team
    header: TEAM
    width: 10
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
| `alertmanagers[].hidden` | `false` | Start with this instance hidden from the alert list |
| `refresh_interval` | `30s` | How often to poll for new alerts |
| `group_labels` | — | Labels available for section grouping (cycled with `g`); use `@instance` for the Alertmanager instance name |
| `columns` | — | Extra columns to show in the alert table, backed by label values |
| `columns[].label` | — | Alert label to display |
| `columns[].header` | uppercase label | Column header text |
| `columns[].width` | `12` | Column width in characters |
| `healthchecks` | — | Named watchdog filter sets (see below) |
| `acknowledgement.duration` | `15m` | How long the silence lasts |
| `acknowledgement.author` | `larm02` | `createdBy` field on the silence |
| `acknowledgement.comment` | `ACK! … on %NOW%` | Comment template; `%NOW%` is replaced with the current UTC time |
| `disable_logo` | `false` | Hide the ASCII logo |

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
| `Enter` | Open group / toggle section |
| `ESC` | Go back / clear filter |
| `/` | Filter alerts |
| `:` | Command mode |
| `a` | Acknowledge selected alert |
| `r` | Refresh now |
| `g` | Cycle section grouping label |
| `Space` | Collapse / expand section |
| `i` | Toggle instance visibility |
| `?` | Toggle help |
| `q` / `Ctrl+C` | Quit |

### Alert groups

The main list shows one row per Alertmanager alert group. Each row displays the alertname, worst severity across all alerts in the group, dominant state, and how long ago the oldest alert started. Press `Enter` to open the group and browse its individual alerts, then `Enter` again on an alert to see its full detail.

### Section grouping

Press `g` to cycle through the labels configured in `group_labels`. When a label is active, groups are nested under section headers based on that label's value in each alert. Sections can be collapsed or expanded with `Space` or `Enter`. Alerts whose labels do not include the active label are collected under a `(none)` section at the bottom.

Labels prefixed with `@` are virtual. `@instance` resolves to the Alertmanager instance name (the `name` field in config), making it possible to split the alert list by AM instance — similar to Karma's `@cluster` grid.

The active grouping label is shown in the breadcrumb line. Press `g` once more past the last configured label to return to the flat list.

### Instance visibility

Press `i` to open the instance popup. Navigate with `j`/`k` and toggle each instance with `Space`. Hidden instances are grayed out in the header and their alerts are excluded from the main list; their alert counts are still updated in the header. Press `i` or `ESC` to close the popup.

Instances can also be hidden by default via `hidden: true` in the config.

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

Press `a` on any alert (in the group list or in detail view) to acknowledge it. larm02 posts a short-lived silence to the Alertmanager instance the alert came from, matching all of the alert's labels exactly.

```yaml
acknowledgement:
  duration: 15m
  author: your-name
  comment: "ACK! Acknowledged on %NOW%"
```

This pairs well with [kthxbye](https://github.com/prymitive/kthxbye), which automatically extends the silence while the alert keeps firing and lets it expire once the alert resolves.

## Multi-instance support

larm02 queries all configured Alertmanager instances concurrently. Groups with identical labels across instances are merged into a single row. Errors from individual instances are shown inline without hiding alerts from healthy instances.

## License

[GPL-3.0](LICENSE)
