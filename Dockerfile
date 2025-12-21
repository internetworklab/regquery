FROM --platform=$BUILDPLATFORM golang:1.24-bookworm AS builder-basis
ARG TARGETOS
ARG TARGETARCH

WORKDIR /app/regquery

COPY go.mod go.mod
COPY go.sum go.sum

RUN GOOS=${TARGETOS} GOARCH=${TARGETARCH} go mod download

FROM --platform=$BUILDPLATFORM golang:1.24-bookworm AS builder
ARG TARGETOS
ARG TARGETARCH

COPY --from=builder-basis /go/pkg /go/pkg

WORKDIR /app/regquery

COPY . .

RUN GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -o bin/regquery ./main.go

FROM debian:bookworm

RUN \
  apt-get update -y && apt-get install -y ca-certificates 

COPY --from=builder /app/regquery/bin/regquery /usr/local/bin/regquery
