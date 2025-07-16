# 🧪 Custom Exporter for Prometheus
A flexible Prometheus exporter with YAML-based configuration for collecting metrics from shell commands, HTTP endpoints, or files.

## 🔧 Features

* ⚙️ **YAML-Driven Configuration** - Define exporters easily in a single YAML file.
* 🧾 **Multiple Sources** - Collect metrics from:
  * Shell commands
  * HTTP APIs
  * Local files
* 🔍 **Flexible Parsing** - Use `regex`, `json`, `line`, or `split` to extract metrics.
* 📊 **Gauge & Counter** metric types.
* 📡 Exposes `/metrics`, `/uptime`, and `/health` endpoints.
* 🖥️ Serves a static HTML page at `/`.

## 🚀 Getting Started

### 1. Clone and Build

```bash
git clone https://github.com/kirillsaidov/custom-exporter.git
cd custom-exporter
go build -o custom-exporter cmd/custom-exporter/main.go
```

### 2. Run the Exporter
#### Run manully
```bash
./custom-exporter --config export.yaml --port 9100
```

* Default config file: `export.yaml`
* Default port: `9100`

#### Setup `systemd` service
Create `custom-exporter.service` file:
```sh
[Unit]
Description=Custom Exporter for Prometheus
After=network.target
StartLimitBurst=5

[Service]
Type=simple
ExecStart=%h/custom-exporter/custom-exporter --config %h/custom-exporter/export.yaml --port 9100
Restart=on-failure
RestartSec=5

[Install]
WantedBy=default.target
```
`%h` - stands for `HOME` directory.

Run the service:
```sh
# 1. create service directory and copy file
mkdir -p ~/.config/systemd/user
cp custom-exporter.service ~/.config/systemd/user/

# 2. enable and start the service
systemctl --user daemon-reload
systemctl --user enable custom-exporter
systemctl --user start custom-exporter

# 3. check status
systemctl --user status custom-exporter
```

### 3. Example YAML Configuration

Here’s a minimal example of `export.yaml`:

```yaml
exporters:
  - name: "example_command_metric"
    type: "command"
    command: "echo 42"
    interval: 10
    metric_type: "gauge"
    parser:
      type: "regex"
      pattern: "(\\d+)"
    labels:
      source: "shell"
    description: "A simple shell-based metric"
```

For full configuration examples, see [`export.yaml.example`](./export.yaml.example).

## 🧠 Parser Types

| Type    | Description                                     |
| ------- | ----------------------------------------------- |
| `regex` | Extracts value via regex (first capture group). |
| `json`  | Navigates JSON using dot-path (`key.subkey`).   |
| `line`  | Gets a specific line by number (`line_num`).    |
| `split` | Splits text by delimiter and selects index.     |

## 📦 Output Example

```bash
# curl http://localhost:9100/metrics

# HELP example_command_metric A simple shell-based metric
# TYPE example_command_metric gauge
example_command_metric{source="shell"} 42
```

## 🖥️ HTTP Endpoints

| Endpoint   | Description                   |
| ---------- | ----------------------------- |
| `/metrics` | Prometheus metrics            |
| `/uptime`  | Uptime in seconds             |
| `/health`  | Simple health check (`OK`)    |
| `/`        | Serves static HTML (optional) |

## 🛠 Advanced Usage

* Multiple exporters with different intervals and sources
* Dynamic labels
* Built-in metric caching
* Error logging for failed fetches/parsing

## LICENSE
MIT.

