FROM golang:1.27-alpine AS builder

WORKDIR /app

COPY go.mod ./
RUN go mod download

COPY . .

RUN go build -o payment-service .

FROM alpine:3.22

WORKDIR /app

COPY --from=builder /app/payment-service .

EXPOSE 8083

CMD ["./payment-service"]