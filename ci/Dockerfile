ARG app_name=policy-machine
ARG app_version=latest

FROM golang:1.23 AS build-env
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY cmd/ cmd/
COPY internal/ internal/
COPY pkg/ pkg/

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags "-w -s -X github.com/kumarabd/policy-machine/internal/config.ApplicationName=$app_name -X github.com/kumarabd/policy-machine/internal/config.ApplicationVersion=$app_version" -a -o service ./cmd

FROM gcr.io/distroless/base:nonroot
WORKDIR /app
COPY --from=build-env /app/internal/config/config.yaml ./config.yaml
COPY --from=build-env /app/service ./
ENTRYPOINT ["./service", "run", "--config", "config.yaml"]
