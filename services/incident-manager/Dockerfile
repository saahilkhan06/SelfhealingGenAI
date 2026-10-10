FROM golang:1.25-alpine AS builder
WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY main.go .

RUN go build -o incident-manager main.go

FROM alpine:latest

WORKDIR /app

COPY --from=builder /app/incident-manager .

EXPOSE 8090

CMD ["./incident-manager"]