FROM golang:1.22 as builder

WORKDIR /src

COPY go.mod go.sum .
RUN go mod download && go mod verify

COPY cmd cmd
COPY internal internal
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags='-extldflags=-static -w -s' -o bot cmd/bot/main.go

FROM scratch

COPY --from=builder /src/bot /bot

ENTRYPOINT ["/bot"]
