FROM --platform=${BUILDPLATFORM} golang:1.25.5 AS builder

WORKDIR /src

# Use the toolchain specified in go.mod, or newer
ENV GOTOOLCHAIN=auto

COPY go.mod go.sum .
RUN go mod download && go mod verify

COPY cmd cmd
COPY internal internal

ARG TARGETARCH
RUN GOARCH=${TARGETARCH} CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags='-extldflags=-static -w -s' -o bot cmd/bot/main.go

FROM python:3.14.2-alpine

RUN python3 -m pip install yt-dlp==2025.12.08

COPY --from=builder /src/bot /bot

ENTRYPOINT ["/bot"]
