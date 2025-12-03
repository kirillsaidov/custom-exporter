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
source ./custom-exporter.env
./custom-exporter --config export.yaml --port 9100
```

* Default config file: `export.yaml`
* Default port: `9100`

#### Setup `systemd` service
```bash
# 1. Modify service file path
sed -i "s|path-to-repo|$(pwd)|g" custom-exporter.service

# 2. Copy service file
sudo cp ./custom-exporter.service /etc/systemd/system/

# 3. enable and start the service
sudo systemctl daemon-reload
sudo systemctl enable custom-exporter
sudo systemctl start custom-exporter

# 4. check status
sudo systemctl status custom-exporter
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

