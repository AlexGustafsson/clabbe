FROM --platform=${BUILDPLATFORM} golang:1.25.5 as builder

WORKDIR /src

# Use the toolchain specified in go.mod, or newer
ENV GOTOOLCHAIN=auto

COPY go.mod go.sum .
RUN go mod download && go mod verify

COPY cmd cmd
COPY internal internal

ARG TARGETARCH
RUN GOARCH=${TARGETARCH} CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags='-extldflags=-static -w -s' -o bot cmd/bot/main.go

FROM scratch

COPY --from=builder /src/bot /bot

ENTRYPOINT ["/bot"]
