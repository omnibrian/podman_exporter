FROM golang:1.17 as builder
ARG ARCH="amd64"

WORKDIR /workspace

COPY go.mod go.mod
COPY go.sum go.sum
# cache deps before building and copying source so that source changes don't
# invalidate downloaded layer
RUN go mod download

COPY . .

RUN GOOS=linux GOARCH=$ARCH make build

# final image
FROM gcr.io/distroless/base

WORKDIR /

COPY --from=builder /workspace/bin/podman_exporter /podman_exporter

ENTRYPOINT [ "/podman_exporter" ]
