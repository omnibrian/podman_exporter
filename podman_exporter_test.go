package main

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"path"
	"sync"
	"syscall"
	"testing"

	"github.com/go-kit/log"
	"github.com/omnibrian/podman-exporter/podmanapi"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

const (
	testSocket = "/tmp/podmanexportertest.sock"
)

var (
	listener      net.Listener
	server        *http.Server
	logger        log.Logger
	podmanVersion podmanapi.Version
	podmanStats   podmanapi.ContainerStatsReport
)

func TestExporter(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Podman Exporter Suite")
}

func serveSocket(handler http.Handler) error {
	if err := syscall.Unlink(testSocket); err != nil && !os.IsNotExist(err) {
		return err
	}

	wg := sync.WaitGroup{}
	wg.Add(1)

	go func() {
		var err error

		listener, err = net.Listen("unix", testSocket)
		if err != nil {
			GinkgoWriter.Println(err)
		}

		server = &http.Server{Handler: handler}
		wg.Done()
		if err = server.Serve(listener); err != nil {
			GinkgoWriter.Println(err)
		}
	}()

	wg.Wait()
	return nil
}

func mockPodmanSocket() error {
	handler := http.NewServeMux()

	handler.HandleFunc("/v3.0.0/libpod/version", func(w http.ResponseWriter, r *http.Request) {
		resp, err := json.Marshal(podmanVersion)
		if err != nil {
			GinkgoWriter.Println(err)
		}

		w.Write(resp)
	})

	handler.HandleFunc("/v3.0.0/libpod/containers/stats", func(w http.ResponseWriter, r *http.Request) {
		resp, err := json.Marshal(podmanStats)
		if err != nil {
			GinkgoWriter.Println(err)
		}

		GinkgoWriter.Printf("%s\n", resp)

		w.Write(resp)
	})

	return serveSocket(handler)
}

func compareMetrics(c prometheus.Collector, fixture string) error {
	expected, err := os.Open(path.Join("test", fixture))
	if err != nil {
		return err
	}

	return testutil.CollectAndCompare(c, expected)
}

var _ = Describe("Podman Exporter", func() {
	BeforeEach(func() {
		logger = log.NewJSONLogger(GinkgoWriter)

		podmanVersion = podmanapi.Version{
			Platform: podmanapi.Platform{
				Name: "linux/amd64/arch-unknown",
			},
			Version:       "3.4.2",
			ApiVersion:    "1.40",
			MinAPIVersion: "1.24",
			GitCommit:     "",
			GoVersion:     "go1.17",
			Os:            "linux",
			Arch:          "amd64",
			KernelVersion: "4.18.0-348.20.1.el8_5.x86_64",
			BuildTime:     "2022-01-13T05:15:49-05:00",
		}
		podmanStats = podmanapi.ContainerStatsReport{
			Stats: []podmanapi.ContainerStats{
				{
					AvgCPU:        0.05,
					ContainerID:   "podman_exporter_id",
					Name:          "podman_exporter_name",
					PerCPU:        nil,
					CPU:           0.06,
					CPUNano:       8765000,
					CPUSystemNano: 8765,
					SystemNano:    1645582942000000000,
					MemUsage:      2500000,
					MemLimit:      8 * 1024 * 1024 * 1024,
					MemPerc:       0.00123,
					NetInput:      789,
					NetOutput:     987,
					BlockInput:    123,
					BlockOutput:   321,
					PIDs:          6,
					UpTime:        8765001,
					Duration:      8765002,
				},
			},
		}
	})

	Describe("NewExporter", func() {
		Context("when podman socket does not exist", func() {
			It("returns an error", func() {
				exporter, err := NewExporter("/tmp/doesnt_exist.sock", logger)
				Expect(err).To(HaveOccurred())
				Expect(exporter).To(BeNil())
			})
		})

		Context("when started with a valid socket", func() {
			It("starts without error", func() {
				err := serveSocket(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					fmt.Fprint(w, "OK")
				}))
				Expect(err).ToNot(HaveOccurred())

				exporter, err := NewExporter(testSocket, logger)
				Expect(err).ToNot(HaveOccurred())
				Expect(exporter).ToNot(BeNil())
				Expect(exporter.logger).To(Equal(logger))
			})
		})
	})

	Describe("Metrics", func() {
		BeforeEach(func() {
			err := mockPodmanSocket()
			Expect(err).ToNot(HaveOccurred())
		})

		AfterEach(func() {
			server.Close()
			listener.Close()
		})

		Context("podmanInfo", func() {
			It("exports metrics when failing to contact podman", func() {
				badSocket := "/tmp/badsocket.sock"
				if _, err := os.Stat(badSocket); err == nil {
					err = os.Remove(badSocket)
					Expect(err).ToNot(HaveOccurred())
				}
				_, err := os.Create(badSocket)
				Expect(err).ToNot(HaveOccurred())
				DeferCleanup(func() error {
					return os.Remove(badSocket)
				})

				exporter, err := NewExporter(badSocket, logger)
				Expect(err).ToNot(HaveOccurred())

				Expect(testutil.CollectAndCount(exporter)).To(Equal(3))
				Expect(compareMetrics(exporter, "error.metrics")).ToNot(HaveOccurred())
			})

			It("exports metrics when no pods", func() {
				podmanStats.Stats = []podmanapi.ContainerStats{}

				exporter, err := NewExporter(testSocket, logger)
				Expect(err).ToNot(HaveOccurred())

				Expect(testutil.CollectAndCount(exporter)).To(Equal(4))
				Expect(compareMetrics(exporter, "default.metrics")).ToNot(HaveOccurred())
			})

			It("exports metrics when pods exist", func() {
				exporter, err := NewExporter(testSocket, logger)
				Expect(err).ToNot(HaveOccurred())

				Expect(testutil.CollectAndCount(exporter)).To(Equal(16))
				Expect(compareMetrics(exporter, "pod_stats.metrics")).ToNot(HaveOccurred())
			})
		})
	})
})
