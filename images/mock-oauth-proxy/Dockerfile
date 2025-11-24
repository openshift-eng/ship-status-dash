FROM registry.access.redhat.com/ubi9/ubi:latest AS builder

WORKDIR /go/src/ship-status-dash

ENV PATH="/go/bin:${PATH}"
ENV GOPATH="/go"

COPY . .

RUN dnf install -y go make && \
    go build -mod=vendor -o mock-oauth-proxy ./cmd/mock-oauth-proxy

FROM registry.access.redhat.com/ubi9/ubi:latest AS base

RUN mkdir -p /app

COPY --from=builder /go/src/ship-status-dash/mock-oauth-proxy /app/mock-oauth-proxy

WORKDIR /app

EXPOSE 8443

LABEL io.k8s.display-name="SHIP Status Mock OAuth Proxy" \
      io.openshift.tags="oauth-proxy,ship-status"

CMD ["./mock-oauth-proxy"]

