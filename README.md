# Podman Exporter for Prometheus

This is a simple server that uses libpod from podman to scrape data from podman-stats(1) and exports them via HTTP for Prometheus consumption.

**Note:** This exporter makes all podman requests through the podman unix socket which requires that the podman systemd service is started to create that socket:

```
systemctl enable --now podman.socket
```

## Getting Started

To run it:

```
./podman_exporter [flags]
```

Help on flags:

```
./podman_exporter --help
```

## Usage with Podman

```
sudo podman run \
  --detach \
  --volume /var/run/podman/podman.sock:/var/run/podman/podman.sock:ro \
  --privileged \
  --publish 9101:9101 \
  --name podman-exporter \
  omnibrian/podman-exporter
```

## Development

### Building

```
make build
```

### Testing

```
make test
```

### TLS and Basic Authentication

The Podman Exporter supports TLS and basic authentication through [prometheus/exporter-toolkit](https://github.com/prometheus/exporter-toolkit).

To use TLS and/or basic authentication, you need to pass a configuration file using the `--web.config.file` parameter. The format of the file is described [in the exporter-toolkit repository](https://github.com/prometheus/exporter-toolkit/blob/master/docs/web-configuration.md).

## License

Apache License 2.0, see [LICENSE](LICENSE).
