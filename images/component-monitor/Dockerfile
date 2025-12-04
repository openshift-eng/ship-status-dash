FROM registry.access.redhat.com/ubi9/ubi:latest AS builder

# Used for local cross-compilation, empty for ci builds
ARG CGO_ENABLED=
ARG GOOS=
ARG GOARCH=

WORKDIR /go/src/ship-status-dash

ENV PATH="/go/bin:${PATH}"
ENV GOPATH="/go"
ENV CGO_ENABLED=${CGO_ENABLED}
ENV GOOS=${GOOS}
ENV GOARCH=${GOARCH}

COPY . .

RUN dnf install -y go make && \
    go build -mod=vendor -o component-monitor ./cmd/component-monitor

FROM registry.access.redhat.com/ubi9/ubi:latest AS base

COPY --from=builder /go/src/ship-status-dash/component-monitor /app/component-monitor

WORKDIR /app

LABEL io.k8s.display-name="SHIP Component Monitor" \
      io.openshift.tags="monitor,ship-status"

CMD ["./component-monitor"]
