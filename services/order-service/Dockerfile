FROM golang:1.27-alpine AS builder

WORKDIR /app

COPY go.mod ./
RUN go mod download

COPY . .

RUN go build -o order-service .

FROM alpine:3.22

WORKDIR /app

COPY --from=builder /app/order-service .

EXPOSE 8082

CMD ["./order-service"]