// These types are built for parsing output from podman socket REST API.
//
// Based on documentation at: https://docs.podman.io/en/latest/_static/api.html

package podmanapi

// for podman path: /v3.0.0/libpod/version
type Version struct {
	Platform      Platform
	Version       string
	ApiVersion    string
	MinAPIVersion string
	GitCommit     string
	GoVersion     string
	Os            string
	Arch          string
	KernelVersion string
	BuildTime     string
}

type Platform struct {
	Name string
}

// for podman path: /v3.0.0/libpod/containers/stats
type ContainerStatsReport struct {
	Error error
	Stats []ContainerStats
}

type ContainerStats struct {
	AvgCPU        float64
	ContainerID   string
	Name          string
	PerCPU        []uint64
	CPU           float64
	CPUNano       uint64
	CPUSystemNano uint64
	DataPoints    int64
	SystemNano    uint64
	MemUsage      uint64
	MemLimit      uint64
	MemPerc       float64
	NetInput      uint64
	NetOutput     uint64
	BlockInput    uint64
	BlockOutput   uint64
	PIDs          uint64
	UpTime        uint64
	Duration      uint64
}
