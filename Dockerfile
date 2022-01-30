FROM golang:1.17 as builder

WORKDIR /workspace

COPY go.mod go.mod
COPY go.sum go.sum
# cache deps before building and copying source so that source changes don't
# invalidate downloaded layer
RUN go mod download

COPY . .

RUN make build

# final image
FROM debian:bullseye-slim

WORKDIR /

COPY --from=builder /workspace/bin/podman_exporter /podman_exporter

EXPOSE     9101
ENTRYPOINT [ "/podman_exporter" ]
