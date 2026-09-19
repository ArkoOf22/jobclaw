# Grafana Cloud metrics for JobClaw

JobClaw exposes pipeline health as Prometheus metrics. Grafana Alloy scrapes them
on the host and pushes them to Grafana Cloud, which stores and graphs them.

**Why push rather than pull.** The metrics endpoint is on loopback with no
authentication, and inbound SSH to this host is pinned to a stale address with
access over SSM. Pushing out means no inbound port, no security group change, and
nothing new exposed to the internet.

**Why not run Prometheus and Grafana here.** They want 250-450MB resident. This is
a 2GB `t3.small` that has already taken OOM kills, so a local stack would make the
problem it is meant to observe worse. Alloy alone is bounded at 256MB and idles far
below that.

## What is already running

| Piece | State |
| :--- | :--- |
| `jobclaw metrics` endpoint | `jobclaw-metrics.service`, enabled, on `127.0.0.1:9090` |
| Metrics exposed | jobs total, by source, shortlisted by source, by recommendation, applications by status |
| Alloy config | `deploy/alloy/config.alloy` |
| Alloy systemd override | `deploy/alloy/alloy-override.conf` |

Confirm the endpoint before wiring anything to it:

```bash
curl -s http://127.0.0.1:9090/healthz          # -> ok
curl -s http://127.0.0.1:9090/metrics | head   # -> jobclaw_jobs_total ...
```

## Step 1: create the Grafana Cloud stack (in the browser)

1. Sign up at [grafana.com](https://grafana.com) and choose the free tier. It
   includes hosted Prometheus and Grafana, with no credit card and no agent
   licence. The free allowance is far above what this sends.
2. Once the stack exists, open **Connections → Add new connection →
   Hosted Prometheus metrics** (sometimes shown as *Prometheus* under
   *Send Metrics*).
3. Choose the option for sending via an agent / `remote_write`. Grafana shows
   three values. Copy all of them:
   - **Remote write endpoint** — looks like
     `https://prometheus-prod-NN-REGION.grafana.net/api/prom/push`
   - **Username / instance ID** — a number, not your email
   - **Password / API token** — generate one with metrics push scope

The token is shown once. Treat it like a password.

## Where to run the commands

> **Every shell command in this document runs on the EC2 host, not on the Mac.**
>
> The Mac is for editing only. It has no `apt-get`, no systemd, and no database,
> so these commands fail there with `command not found`. Get onto the host first:
>
> ```bash
> ssh jobclaw          # lands as `ubuntu`; sudo works from there
> ```
>
> Or run a single command without an interactive shell:
>
> ```bash
> ssh jobclaw 'sudo apt-get update'
> ```
>
> `ubuntu` is the login user and the only one with the SSH key. Project-owned
> work goes through `sudo -u openclaw`. See `environment.md`.

## Step 2: install Alloy on the host

Grafana Agent reached end of life in November 2025; Alloy is its replacement.
From the [Alloy Linux install docs](https://grafana.com/docs/alloy/latest/set-up/install/linux/),
add the Grafana apt repository and install.

**On the EC2 host** (`ssh jobclaw` first):

```bash
sudo apt-get install -y gpg
sudo mkdir -p /etc/apt/keyrings
wget -q -O - https://apt.grafana.com/gpg.key \
  | gpg --dearmor | sudo tee /etc/apt/keyrings/grafana.gpg > /dev/null
sudo chmod 644 /etc/apt/keyrings/grafana.gpg
echo "deb [signed-by=/etc/apt/keyrings/grafana.gpg] https://apt.grafana.com stable main" \
  | sudo tee /etc/apt/sources.list.d/grafana.list
sudo apt-get update
sudo apt-get install -y alloy
```

Installing does not start anything useful: the unit needs
`/etc/alloy/grafana-cloud.env` (step 3) and fails loudly without it. So it is safe
to install before the credentials exist.

## Step 3: install the config and credentials

```bash
cd /home/openclaw/jobclaw
sudo cp deploy/alloy/config.alloy /etc/alloy/config.alloy

sudo mkdir -p /etc/systemd/system/alloy.service.d
sudo cp deploy/alloy/alloy-override.conf \
        /etc/systemd/system/alloy.service.d/override.conf
```

Create the credentials file with the three values from step 1. It is root-only and
deliberately not in git:

```bash
sudo tee /etc/alloy/grafana-cloud.env > /dev/null <<'EOF'
GRAFANA_CLOUD_PROM_URL=https://prometheus-prod-NN-REGION.grafana.net/api/prom/push
GRAFANA_CLOUD_PROM_USER=1234567
GRAFANA_CLOUD_PROM_TOKEN=glc_paste_the_token_here
EOF

sudo chown root:root /etc/alloy/grafana-cloud.env
sudo chmod 600 /etc/alloy/grafana-cloud.env
```

Check the config parses before starting anything:

```bash
sudo alloy fmt /etc/alloy/config.alloy > /dev/null && echo "config OK"
```

## Step 4: start it

```bash
sudo systemctl daemon-reload
sudo systemctl enable --now alloy
systemctl status alloy --no-pager
journalctl -u alloy -n 30 --no-pager
```

A healthy start logs the components coming up and no `remote_write` errors. The
failures worth recognising:

| Log says | Cause |
| :--- | :--- |
| `401 Unauthorized` | Wrong username (must be the numeric instance ID) or a bad token |
| `404` on push | Endpoint URL copied from the wrong region or missing `/api/prom/push` |
| `connection refused` to `127.0.0.1:9090` | `jobclaw-metrics.service` is not running |
| Unit fails immediately | `/etc/alloy/grafana-cloud.env` missing; `EnvironmentFile` has no `-`, so this fails loudly on purpose |

## Step 5: confirm data arrived

In Grafana Cloud, open **Explore**, pick the Prometheus data source, and run:

```promql
jobclaw_jobs_total
jobclaw_shortlisted_by_source
```

First samples land within a minute or two of Alloy starting. If Explore autocompletes
the metric names, the push is working.

## Step 6: import the dashboard

`deploy/alloy/jobclaw-dashboard.json` is a starter dashboard covering both halves
of the question: is the job search producing anything, and what is this box
costing.

In Grafana Cloud: **Dashboards → New → Import**, paste the file contents, and pick
your Prometheus data source when prompted.

| Row | Panels |
| :--- | :--- |
| Pipeline | jobs discovered, actionable now, shortlisted by source, recommendation mix, jobs by source, applications by status |
| Host cost and stability | OOM kills, memory available, memory pressure (PSI), CPU, disk free |

Two panels are worth calling out:

- **OOM kills** turns red on any increase. This box has been through two kills
  already and both were found in `dmesg` long after the fact.
- **Memory pressure (PSI)** is the leading indicator. It climbs before a kill, so
  it is the one to watch if you want warning rather than an autopsy.

The default time range is 7 days and the timezone is set to IST, matching the
discovery schedule.

## What gets collected

Two scrape targets.

**JobClaw pipeline**, every 60s. The database only changes when the six-hourly timer
runs, so a faster interval adds samples without adding information.

- `jobclaw_jobs_total`
- `jobclaw_jobs_scored_total`
- `jobclaw_jobs_by_source{source}`
- `jobclaw_shortlisted_by_source{source}` — which board is worth scraping
- `jobclaw_jobs_by_recommendation{recommendation}`
- `jobclaw_applications_by_status{status}`

**Host health**, every 30s, from a hand-picked node_exporter collector set rather
than the default ~40, to stay well inside the free tier's active series limit:

- `cpu`, `meminfo`, `filesystem`, `loadavg`, `stat`
- `vmstat` — includes `node_vmstat_oom_kill`, so an OOM kill becomes a visible
  counter increment rather than something discovered later in `dmesg`
- `pressure` — PSI memory stall time, which climbs *before* a kill

That last pair is the point of including host metrics at all: `openclaw-gateway`
has been OOM-killed twice on this box, and until now the only evidence was in
`dmesg` after the fact.

## Limits worth knowing

- **This measures the pipeline, not outcomes.** Shortlist counts are a visibility
  proxy. Applications sent, replies, and interviews are the numbers that matter,
  and they are not instrumented: submission happens by hand on the employer's form
  and is only recorded when `jobclaw mark` is run.
- **Gauges, not history.** Every JobClaw metric is a current count read from the
  database. Grafana builds the time series by sampling it. A re-score changes the
  numbers retroactively in the database but not in the already-stored samples, so
  a step change on the graph can mean a rules change rather than a discovery run.
- **The free tier is enough here**, by a wide margin. Roughly 15 JobClaw series and
  a few dozen host series against a 10k active-series allowance.
