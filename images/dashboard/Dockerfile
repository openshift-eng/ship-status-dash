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

RUN dnf module enable nodejs:18 -y && \
    dnf install -y go make npm && \
    make build-dashboard && \
    make build-frontend

FROM registry.access.redhat.com/ubi9/ubi:latest AS base

RUN mkdir -p /app/static

COPY --from=builder /go/src/ship-status-dash/dashboard /app/dashboard
COPY --from=builder /go/src/ship-status-dash/frontend/build /app/static

WORKDIR /app

EXPOSE 8080

LABEL io.k8s.display-name="SHIP Status Dashboard" \
      io.openshift.tags="dashboard,ship-status"

CMD ["./dashboard"]
