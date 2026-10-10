FROM golang:1.27-alpine AS builder

WORKDIR /app

COPY go.mod ./
RUN go mod download

COPY . .

RUN go build -o user-service .

FROM alpine:3.22

WORKDIR /app

COPY --from=builder /app/user-service .

EXPOSE 8081

CMD ["./user-service"]