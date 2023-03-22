# syntax=docker/dockerfile:1

##
## Build
##
FROM golang:1.18-buster AS build
COPY /certs/consul-agent-ca.crt /usr/local/share/ca-certificates/consul-agent-ca.crt
RUN update-ca-certificates
WORKDIR /app

COPY go.mod ./
COPY go.sum ./
RUN go mod download

COPY /main/*.go main/
COPY /internal/*.go internal/
WORKDIR /app/main
RUN go build -o /consul-ingress-service-monitor

#
# Deploy
#
FROM gcr.io/distroless/base-debian10

WORKDIR /

COPY --from=build /consul-ingress-service-monitor /consul-ingress-service-monitor
COPY --from=build /etc/ssl/certs /etc/ssl/certs

EXPOSE 10000

USER nonroot:nonroot

ENTRYPOINT ["/consul-ingress-service-monitor"]