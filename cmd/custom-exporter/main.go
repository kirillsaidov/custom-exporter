package main

import (
	"bytes"
	"context"
	"embed"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"gopkg.in/yaml.v3"
)

// Global variable to store server start time
var serverStartTime = time.Now()

// Embed the HTML file at compile time
//go:embed static/index.html
var htmlContent embed.FS

// Config represents the main configuration structure
type Config struct {
    Exporters []Exporter `yaml:"exporters"`
}

// Exporter represents a single metric exporter configuration
type Exporter struct {
    Name        string            `yaml:"name"`
    Type        string            `yaml:"type"`                 // "command", "http", "file"
    Command     string            `yaml:"command,omitempty"`
    URL         string            `yaml:"url,omitempty"`
    FilePath    string            `yaml:"file_path,omitempty"`
    Interval    int               `yaml:"interval"`             // seconds
    MetricType  string            `yaml:"metric_type"`          // "gauge", "counter"
    Parser      Parser            `yaml:"parser"`
    Labels      map[string]string `yaml:"labels,omitempty"`
    Description string            `yaml:"description"`
}

// Parser defines how to parse the output
type Parser struct {
    Type     string `yaml:"type"`                               // "regex", "json", "line", "split"
    Pattern  string `yaml:"pattern,omitempty"`
    JsonPath string `yaml:"json_path,omitempty"`
    LineNum  int    `yaml:"line_num,omitempty"`
    Split    string `yaml:"split,omitempty"`
    Index    int    `yaml:"index,omitempty"`
}

// CustomCollector implements prometheus.Collector
type CustomCollector struct {
    config    *Config
    metrics   map[string]prometheus.Metric
    lastFetch map[string]time.Time
    mutex     sync.RWMutex
}

// NewCustomCollector creates a new custom collector
func NewCustomCollector(config *Config) *CustomCollector {
    return &CustomCollector{
        config: config,
        metrics: make(map[string]prometheus.Metric),
        lastFetch: make(map[string]time.Time),
    }
}

// Describe implements prometheus.Collector
func (c *CustomCollector) Describe(ch chan<- *prometheus.Desc) {
    // Prometheus will call this to get metric descriptions
    for _, exporter := range c.config.Exporters {
        desc := prometheus.NewDesc(
            exporter.Name,
            exporter.Description,
            nil,
            exporter.Labels,
        )
        ch <- desc
    }
}

// Collect implements prometheus.Collector
func (c *CustomCollector) Collect(ch chan<- prometheus.Metric) {
    for _, exporter := range c.config.Exporters {
        // Read cached data with mutex to avoid race conditions
        c.mutex.RLock()
        lastFetchTime, exists := c.lastFetch[exporter.Name]
        cachedMetric, hasCached := c.metrics[exporter.Name]
        c.mutex.RUnlock()

        // Check if we need to fetch new data based on interval
        if exists && time.Since(lastFetchTime) < time.Duration(exporter.Interval)*time.Second {
            // Use cached metric if available
            if hasCached {
                ch <- cachedMetric
                continue
            }
        }

        // Fetch new data
        value, err := c.fetchData(exporter)
        if err != nil {
            log.Printf("Error fetching data for %s: %v", exporter.Name, err)
        }

        // Create metric
        desc := prometheus.NewDesc(
            exporter.Name,
            exporter.Description,
            nil, 
            exporter.Labels,
        )

        // Make const metric based on type: gauge, counter
        var metric prometheus.Metric
        switch exporter.MetricType {
        case "counter":
            metric = prometheus.MustNewConstMetric(desc, prometheus.CounterValue, value)
        default: // gauge
            metric = prometheus.MustNewConstMetric(desc, prometheus.GaugeValue, value)
        }

        // Cache the metric and update fetch time with mutex to avoid race conditions
        c.mutex.Lock()
        c.metrics[exporter.Name] = metric
        c.lastFetch[exporter.Name] = time.Now()
        c.mutex.Unlock()

        ch <- metric
    }
}

// fetchData fetches data based on the exporter configuration
func (c *CustomCollector) fetchData(exporter Exporter) (float64, error) {
    var rawData string
    var err error 

    switch exporter.Type {
    case "command":
        rawData, err = c.executeCommand(exporter.Command)
    case "http":
        rawData, err = c.fetchHTTP(exporter.URL)
    case "file": 
        rawData, err = c.readFile(exporter.FilePath)
    default:
        return 0, fmt.Errorf("unsupported exporter type: %s", exporter.Type)
    }

    if err != nil {
        return 0, err
    }

    // Parse the raw data
    return c.parseData(rawData, exporter.Parser)
}

// executeCommand executes a shell command and returns the output
func (c *CustomCollector) executeCommand(command string) (string, error) {
    // Add timeout to prevent hanging
    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()

    // Init command
    var out bytes.Buffer
    cmd := exec.CommandContext(ctx, "sh", "-c", command)
    cmd.Stdout = &out 
    cmd.Stderr = &out

    // Run command
    err := cmd.Run()
    if err != nil {
        return "", fmt.Errorf("command execution failed: %v, output: %s", err, out.String())
    }

    return out.String(), nil 
}

// fetchHTTP fetchs data from HTTP endpoint
func (c *CustomCollector) fetchHTTP(url string) (string, error) {
    client := &http.Client{Timeout: 10 * time.Second}
    resp, err := client.Get(url)
    if err != nil {
        return "", err
    }
    defer resp.Body.Close()

    body, err := io.ReadAll(resp.Body)
    if err != nil {
        return "", err 
    }

    return string(body), nil
}

// readFile reads data from a file
func (c *CustomCollector) readFile(filePath string) (string, error) {
    data, err := os.ReadFile(filePath)
    if err != nil {
        return "", err 
    }
    return string(data), nil 
}

// parseData parses the raw data based on the parser configuration
func (c *CustomCollector) parseData(rawData string, parser Parser) (float64, error) {
    var result string 

    switch parser.Type {
    case "regex":
        re, err := regexp.Compile(parser.Pattern)
        if err != nil {
            return 0, fmt.Errorf("invalid regex pattern: %v", err) 
        }

        matches := re.FindStringSubmatch(rawData)
        if len(matches) < 2 {
            return 0, fmt.Errorf("regex pattern did not match or capture group not found") 
        }
        result = matches[1]
    
    case "json":
        var jsonData interface{}
        err := json.Unmarshal([]byte(rawData), &jsonData)
        if err != nil {
            return 0, fmt.Errorf("failed to parse JSON: %v", err)
        }
        
        // Using simple JSON path parsing (supports bsic dot notation, e.g.: "connections.active")
        result, err = c.extractJSONValue(jsonData, parser.JsonPath)
        if err != nil {
            return 0, err 
        }

    case "line":
        lines := strings.Split(strings.TrimSpace(rawData), "\n")
        if parser.LineNum >= len(lines) || parser.LineNum < 0 {
            return 0, fmt.Errorf("line number %d out of range", parser.LineNum)
        }
        result = strings.TrimSpace(lines[parser.LineNum])

    case "split":
        parts := strings.Split(strings.TrimSpace(rawData), parser.Split)
        if parser.Index >= len(parts) || parser.Index < 0 {
            return 0, fmt.Errorf("split index %d out of range", parser.Index)
        }
        result = strings.TrimSpace(parts[parser.Index])
        
    default:
        // Try to parse the entire string as a number
        result = strings.TrimSpace(rawData)
    }

    // Convert to float64
    value, err := strconv.ParseFloat(result, 64)
    if err != nil {
        return 0, fmt.Errorf("failed to parse value as a number: %v", err)
    }

    return value, nil
}

// extractJSONValue extracts a value from JSON data using a simple dot path
func (c *CustomCollector) extractJSONValue(data interface{}, path string) (string, error) {
    if path == "" {
        return fmt.Sprintf("%v", data), nil 
    }

    parts := strings.Split(path, ".")
    current := data 
    for _, part := range parts {
        switch v := current.(type) {
        case map[string]interface{}:
            if val, exists := v[part]; exists {
                current = val 
            } else {
                return "", fmt.Errorf("JSON path not found: %s", part)
            }

        case []interface{}:
            index, err := strconv.Atoi(part)
            if err != nil {
                return "", fmt.Errorf("invalid array index: %s", part)
            }
            if index >= len(v) || index < 0 {
                return "", fmt.Errorf("array index out of range: %d", index)
            }
            current = v[index]
        default:
            return "", fmt.Errorf("cannot navigate JSON path at: %s", part)
        }
    }

    return fmt.Sprintf("%v", current), nil
}

// loadConfig loads the configuration from a YAML file
func loadConfig(filename string) (*Config, error) {
    data, err := os.ReadFile(filename)
    if err != nil {
        return nil, err
    }

    var config Config
    err = yaml.Unmarshal(data, &config)
    if err != nil {
        return nil, err
    }

    return &config, nil
}

func main() {
    // Define the --config flag with a default value and description
    port := flag.String("port", "9100", "Serve app at the specified port.")
    configPath := flag.String("config", "export.yaml", "Path to the YAML export configuration file.")

    // Parse all command-line flags
    flag.Parse()

    // Load configuration
    config, err := loadConfig(*configPath)
    if err != nil {
        log.Fatalf("Failed to load configuration: %v", err)
    }

    // Create custom collector
    collector := NewCustomCollector(config)

    // Register the collector
    prometheus.MustRegister(collector)

    // Set up HTTP server
    http.Handle("/metrics", promhttp.Handler())

    // Health check endpoint
    http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Content-Type", "text/html")
        w.WriteHeader(http.StatusOK)
        w.Write([]byte("OK"))
    })

    // Uptime endpoint - returns uptime in seconds
    http.HandleFunc("/uptime", func(w http.ResponseWriter, r *http.Request) {
        uptime := time.Since(serverStartTime).Seconds()
        w.Header().Set("Content-Type", "text/plain")
        w.WriteHeader(http.StatusOK)
        w.Write([]byte(fmt.Sprintf("%.0f", uptime)))
    })

    // Root endpoint - serve HTML
    http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
        // Only serve HTML for exact root path
        if r.URL.Path != "/" {
            http.NotFound(w, r)
            return
        }

        // Load HTML to serve at root
        html, err := htmlContent.ReadFile("static/index.html")
        if err != nil {
            log.Fatalf("Failed to load HTML: %v", err)
        }

        // Serve HTML
        w.Header().Set("Content-Type", "text/html")
        w.WriteHeader(http.StatusOK)
        w.Write([]byte(html))
    })

    log.Printf("Custom Exporter starting on port %s", *port)
    log.Printf("Metrics endpoint: http://localhost:%s/metrics", *port)
    log.Fatal(http.ListenAndServe(":"+*port, nil))
}





