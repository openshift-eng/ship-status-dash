FROM registry.access.redhat.com/ubi9/ubi:latest AS builder

WORKDIR /go/src/ship-status-dash

ENV PATH="/go/bin:${PATH}"
ENV GOPATH="/go"

COPY . .

RUN dnf install -y go make && \
    go build -mod=vendor -o migrate ./cmd/migrate

FROM registry.access.redhat.com/ubi9/ubi:latest AS base

RUN mkdir -p /app

COPY --from=builder /go/src/ship-status-dash/migrate /app/migrate

WORKDIR /app

LABEL io.k8s.display-name="SHIP Status Database Migration" \
      io.openshift.tags="migration,ship-status"

CMD ["./migrate"]

