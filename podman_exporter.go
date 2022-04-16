package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"os"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/omnibrian/podman-exporter/podmanapi"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/common/promlog"
	"github.com/prometheus/common/promlog/flag"
	"github.com/prometheus/common/version"
	"github.com/prometheus/exporter-toolkit/web"
	webflag "github.com/prometheus/exporter-toolkit/web/kingpinflag"
	"gopkg.in/alecthomas/kingpin.v2"
)

const (
	namespace = "podman"
)

type metricInfo struct {
	Desc *prometheus.Desc
	Type prometheus.ValueType
}

func newContainerMetric(metricName string, docString string, t prometheus.ValueType, constLabels prometheus.Labels) metricInfo {
	return metricInfo{
		Desc: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "container", metricName),
			docString,
			[]string{"container_id", "name"},
			constLabels,
		),
		Type: t,
	}
}

var (
	cpu            = newContainerMetric("cpu", "Current CPU usage.", prometheus.GaugeValue, nil)
	cpuAverage     = newContainerMetric("cpu_average", "Average CPU usage.", prometheus.GaugeValue, nil)
	cpuUsageTotal  = newContainerMetric("cpu_usage_total", "Total CPU usage.", prometheus.GaugeValue, nil)
	cpuUsageKernel = newContainerMetric("cpu_usage_kernel", "Kernel CPU usage.", prometheus.GaugeValue, nil)
	memUsage       = newContainerMetric("mem_usage", "Current memory usage.", prometheus.GaugeValue, nil)
	memLimit       = newContainerMetric("mem_limit", "Memory limit.", prometheus.GaugeValue, nil)
	memPerc        = newContainerMetric("mem_percent", "Percentage of memory used.", prometheus.GaugeValue, nil)
	netInput       = newContainerMetric("net_input", "Inbound network traffic.", prometheus.GaugeValue, nil)
	netOutput      = newContainerMetric("net_output", "Outbount network traffic.", prometheus.GaugeValue, nil)
	blockInput     = newContainerMetric("block_input", "Block traffic in.", prometheus.GaugeValue, nil)
	blockOutput    = newContainerMetric("block_output", "Block traffic out.", prometheus.GaugeValue, nil)
	pids           = newContainerMetric("pids", "Number of PIDs running.", prometheus.GaugeValue, nil)

	podmanInfo = prometheus.NewDesc(prometheus.BuildFQName(namespace, "version", "info"), "Podman version info.", []string{"version"}, nil)
	podmanUp   = prometheus.NewDesc(prometheus.BuildFQName(namespace, "", "up"), "Was the last scrape of Podman successful.", nil, nil)
)

// Exporter collects Podman stats using podman v3 API similar to the
// podman-stats(1) command and exports them using the prometheus metrics
// package.
type Exporter struct {
	client http.Client

	up                           prometheus.Gauge
	totalScrapes, scrapeFailures prometheus.Counter
	logger                       log.Logger
}

// NewExporter returns an initialized Exporter.
func NewExporter(podmanSocket string, logger log.Logger) (*Exporter, error) {
	if _, err := os.Stat(podmanSocket); err != nil {
		return nil, err
	}

	client := http.Client{
		Transport: &http.Transport{
			DisableCompression: true,
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				return net.Dial("unix", podmanSocket)
			},
		},
	}

	return &Exporter{
		client: client,
		up: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "up",
			Help:      "Was the last scrape of podman successful.",
		}),
		totalScrapes: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "exporter_scrapes_total",
			Help:      "Current total podman scrapes.",
		}),
		scrapeFailures: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "exporter_scrape_failures_total",
			Help:      "Current total podman scrape failures.",
		}),
		logger: logger,
	}, nil
}

// Describe describes all the metrics ever exported by the Podman exporter. It
// implements prometheus.Collector.
func (e *Exporter) Describe(ch chan<- *prometheus.Desc) {
	ch <- podmanInfo
	ch <- podmanUp

	ch <- cpu.Desc
	ch <- cpuAverage.Desc
	ch <- cpuUsageTotal.Desc
	ch <- cpuUsageKernel.Desc
	ch <- memUsage.Desc
	ch <- memLimit.Desc
	ch <- memPerc.Desc
	ch <- netInput.Desc
	ch <- netOutput.Desc
	ch <- blockInput.Desc
	ch <- blockOutput.Desc
	ch <- pids.Desc

	ch <- e.totalScrapes.Desc()
	ch <- e.scrapeFailures.Desc()
}

// Collect fetches the stats from configured Podman ContainerEngine and delivers
// them as Prometheus metrics. It implements prometheus.Collector.
func (e *Exporter) Collect(ch chan<- prometheus.Metric) {
	e.totalScrapes.Inc()
	up := e.scrape(ch)
	if up != 1 {
		e.scrapeFailures.Inc()
	}

	ch <- prometheus.MustNewConstMetric(podmanUp, prometheus.GaugeValue, up)
	ch <- e.totalScrapes
	ch <- e.scrapeFailures
}

// scrape calls into podman to get metrics and publish them to prometheus.Metric
// channel.
func (e *Exporter) scrape(ch chan<- prometheus.Metric) (up float64) {
	e.totalScrapes.Inc()
	var err error

	var podmanVersion podmanapi.Version
	if err = e.podmanGet("v3.0.0/libpod/version", "", &podmanVersion); err != nil {
		return 0
	}
	ch <- prometheus.MustNewConstMetric(podmanInfo, prometheus.GaugeValue, 1, podmanVersion.Version)

	var podmanStats podmanapi.ContainerStatsReport
	query := url.Values{}
	query.Add("stream", "false")
	if err = e.podmanGet("v3.0.0/libpod/containers/stats", query.Encode(), &podmanStats); err != nil {
		return 0
	}
	for _, podmanStat := range podmanStats.Stats {
		ch <- prometheus.MustNewConstMetric(cpu.Desc, cpu.Type, podmanStat.CPU, podmanStat.ContainerID, podmanStat.Name)
		ch <- prometheus.MustNewConstMetric(cpuAverage.Desc, cpuAverage.Type, podmanStat.AvgCPU, podmanStat.ContainerID, podmanStat.Name)
		ch <- prometheus.MustNewConstMetric(cpuUsageTotal.Desc, cpuUsageTotal.Type, float64(podmanStat.CPUNano), podmanStat.ContainerID, podmanStat.Name)
		ch <- prometheus.MustNewConstMetric(cpuUsageKernel.Desc, cpuUsageKernel.Type, float64(podmanStat.CPUSystemNano), podmanStat.ContainerID, podmanStat.Name)
		ch <- prometheus.MustNewConstMetric(memUsage.Desc, memUsage.Type, float64(podmanStat.MemUsage), podmanStat.ContainerID, podmanStat.Name)
		ch <- prometheus.MustNewConstMetric(memLimit.Desc, memLimit.Type, float64(podmanStat.MemLimit), podmanStat.ContainerID, podmanStat.Name)
		ch <- prometheus.MustNewConstMetric(memPerc.Desc, memPerc.Type, podmanStat.MemPerc, podmanStat.ContainerID, podmanStat.Name)
		ch <- prometheus.MustNewConstMetric(netInput.Desc, netInput.Type, float64(podmanStat.NetInput), podmanStat.ContainerID, podmanStat.Name)
		ch <- prometheus.MustNewConstMetric(netOutput.Desc, netOutput.Type, float64(podmanStat.NetOutput), podmanStat.ContainerID, podmanStat.Name)
		ch <- prometheus.MustNewConstMetric(blockInput.Desc, blockInput.Type, float64(podmanStat.BlockInput), podmanStat.ContainerID, podmanStat.Name)
		ch <- prometheus.MustNewConstMetric(blockOutput.Desc, blockOutput.Type, float64(podmanStat.BlockOutput), podmanStat.ContainerID, podmanStat.Name)
		ch <- prometheus.MustNewConstMetric(pids.Desc, pids.Type, float64(podmanStat.PIDs), podmanStat.ContainerID, podmanStat.Name)
	}

	return 1
}

// podmanGet builds and executes http call against the configured client with
// the given path and unmarshals json output to given interface.
func (e *Exporter) podmanGet(path string, query string, iface interface{}) (err error) {
	url, err := url.Parse("http://unix")
	if err != nil {
		level.Error(e.logger).Log("msg", "failed to parse unix url", "err", err)
		return err
	}
	url.Path = path
	url.RawQuery = query

	req, err := http.NewRequest("GET", url.String(), nil)
	if err != nil {
		level.Error(e.logger).Log("msg", "failed to create request for podman socket", "url", url.String(), "err", err)
		return err
	}
	req.Header.Add("Content-Type", "application/json")

	resp, err := e.client.Do(req)
	if err != nil {
		level.Error(e.logger).Log("msg", "failed to make request to podman socket", "url", url.String(), "err", err)
		return err
	}

	level.Debug(e.logger).Log("msg", "podman socket request done", "url", url.String(), "statusCode", resp.StatusCode)
	if resp.StatusCode != http.StatusOK {
		err = fmt.Errorf("did not get successful response from podman socket: %d", resp.StatusCode)
		level.Error(e.logger).Log("msg", "did not get successful response from podman socket", "url", url.String(), "statusCode", resp.StatusCode, "err", err)
		return err
	}

	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		level.Error(e.logger).Log("msg", "failed to read response from podman socket", "url", url.String(), "statusCode", resp.StatusCode, "err", err)
		return err
	}

	if err = json.Unmarshal(body, &iface); err != nil {
		level.Error(e.logger).Log("msg", "failed to unmarshal json from podman response", "url", url.String(), "statusCode", resp.StatusCode, "err", err)
		return err
	}

	return nil
}

// respondSplash is a higher order function for returning a splash page pointing
// the user to metricsPath.
func respondSplash(metricsPath string) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`
			<html>
				<head>
					<title>Podman Exporter</title>
				</head>
				<body>
					<h1>Podman Exporter</h1>
					<p><a href='` + metricsPath + `'>Metrics</a></p>
				</body>
			</html>
		`))
	}
}

func main() {
	var (
		webConfig     = webflag.AddFlags(kingpin.CommandLine)
		listenAddress = kingpin.Flag("web.listen-address", "Address to listen on for web interface and telemetry.").Default(":9101").String()
		metricsPath   = kingpin.Flag("web.metrics-path", "Path under which to expose metrics.").Default("/metrics").String()
		podmanSocket  = kingpin.Flag("podman.socket", "Path to the podman socket to scrape.").Default("/var/run/podman/podman.sock").String()
	)

	promlogConfig := &promlog.Config{}
	flag.AddFlags(kingpin.CommandLine, promlogConfig)
	kingpin.Version(version.Print("podman_exporter"))
	kingpin.HelpFlag.Short('h')
	kingpin.Parse()
	logger := promlog.New(promlogConfig)

	level.Info(logger).Log("msg", "Starting podman_exporter", "version", version.Info())
	level.Info(logger).Log("msg", "Build context", "context", version.BuildContext())

	exporter, err := NewExporter(*podmanSocket, logger)
	if err != nil {
		level.Error(logger).Log("msg", "Error starting podman client, is the 'podman.socket' service running?", "err", err)
		os.Exit(1)
	}
	prometheus.MustRegister(exporter)
	prometheus.MustRegister(version.NewCollector("podman_exporter"))

	level.Info(logger).Log("msg", "Listening on address", "address", *listenAddress)
	http.Handle(*metricsPath, promhttp.Handler())
	http.HandleFunc("/", respondSplash(*metricsPath))
	server := &http.Server{Addr: *listenAddress}
	if err := web.ListenAndServe(server, *webConfig, logger); err != nil {
		level.Error(logger).Log("msg", "Error starting HTTP server", "err", err)
		os.Exit(1)
	}
}
