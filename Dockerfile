# Build stage
ARG goversion
FROM golang:${goversion}-alpine as builder
RUN apk add build-base git mercurial
RUN addgroup -S --gid 2000 spire
RUN adduser -S -G spire --uid 1000 spire
USER spire:spire
ADD --chown=spire:spire go.mod /spire/go.mod
ADD --chown=spire:spire proto/spire/go.mod /spire/proto/spire/go.mod
RUN cd /spire && go mod download
ADD --chown=spire:spire . /spire
WORKDIR /spire
RUN make build

# Common base
FROM alpine AS spire-base
RUN apk --no-cache add dumb-init
RUN apk --no-cache add ca-certificates
RUN addgroup -S --gid 2000 spire
RUN adduser -S -G spire --uid 1000 spire
RUN mkdir -m 750 -p /opt/spire/bin && chown -R spire:spire /opt/spire

# SPIRE Server
FROM spire-base AS spire-server
COPY --from=builder /spire/bin/spire-server /opt/spire/bin/spire-server
RUN mkdir -m 775 -p /run/spire/sockets && chown -R spire:spire /run/spire
WORKDIR /opt/spire
USER spire:spire
ENTRYPOINT ["/usr/bin/dumb-init", "/opt/spire/bin/spire-server", "run"]
CMD []

# SPIRE Agent
FROM spire-base AS spire-agent
COPY --from=builder /spire/bin/spire-agent /opt/spire/bin/spire-agent
RUN mkdir -m 775 -p /run/spire/sockets && chown -R spire:spire /run/spire
WORKDIR /opt/spire
USER spire:spire
ENTRYPOINT ["/usr/bin/dumb-init", "/opt/spire/bin/spire-agent", "run"]
CMD []

# K8S Workload Registrar
FROM spire-base AS k8s-workload-registrar
COPY --from=builder /spire/bin/k8s-workload-registrar /opt/spire/bin/k8s-workload-registrar
RUN mkdir -m 775 -p /run/spire/sockets && chown -R spire:spire /run/spire
WORKDIR /opt/spire
USER spire:spire
ENTRYPOINT ["/usr/bin/dumb-init", "/opt/spire/bin/k8s-workload-registrar"]
CMD []

# OIDC Discovery Provider
FROM spire-base AS oidc-discovery-provider
COPY --from=builder /spire/bin/oidc-discovery-provider /opt/spire/bin/oidc-discovery-provider
WORKDIR /opt/spire
USER spire:spire
ENTRYPOINT ["/usr/bin/dumb-init", "/opt/spire/bin/oidc-discovery-provider"]
CMD []
