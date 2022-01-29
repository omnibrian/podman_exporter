package main

import (
	"net/http"
	"os"

	"github.com/go-kit/log/level"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/common/promlog"
	"github.com/prometheus/common/promlog/flag"
	"github.com/prometheus/common/version"
	"github.com/prometheus/exporter-toolkit/web"
	webflag "github.com/prometheus/exporter-toolkit/web/kingpinflag"
	"gopkg.in/alecthomas/kingpin.v2"
)

func RespondSplash(metricsPath string) func(http.ResponseWriter, *http.Request) {
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
	)

	promlogConfig := &promlog.Config{}
	flag.AddFlags(kingpin.CommandLine, promlogConfig)
	kingpin.Version(version.Print("podman_exporter"))
	kingpin.HelpFlag.Short('h')
	kingpin.Parse()
	logger := promlog.New(promlogConfig)

	// TODO: selectedServerMetrics

	level.Info(logger).Log("msg", "Starting podman_exporter", "version", version.Info())
	level.Info(logger).Log("msg", "Build context", "context", version.BuildContext())

	// TODO: NewExporter & prom.MustRegister

	level.Info(logger).Log("msg", "Listening on address", "address", *listenAddress)
	http.Handle(*metricsPath, promhttp.Handler())
	http.HandleFunc("/", RespondSplash(*metricsPath))
	server := &http.Server{Addr: *listenAddress}
	if err := web.ListenAndServe(server, *webConfig, logger); err != nil {
		level.Error(logger).Log("msg", "Error starting HTTP server", "err", err)
		os.Exit(1)
	}
}
