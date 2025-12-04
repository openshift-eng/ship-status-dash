FROM registry.access.redhat.com/ubi9/ubi:latest AS builder

WORKDIR /go/src/ship-status-dash

ENV PATH="/go/bin:${PATH}"
ENV GOPATH="/go"

COPY . .

RUN dnf install -y go make && \
    go build -mod=vendor -o mock-monitored-component ./cmd/mock-monitored-component

FROM registry.access.redhat.com/ubi9/ubi:latest AS base

RUN mkdir -p /app

COPY --from=builder /go/src/ship-status-dash/mock-monitored-component /app/mock-monitored-component

WORKDIR /app

LABEL io.k8s.display-name="SHIP Mock Monitored Component" \
      io.openshift.tags="mock,testing,ship-status"

CMD ["./mock-monitored-component"]

