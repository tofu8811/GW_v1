FROM golang:1.26-alpine AS build

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/gateway-api ./cmd/gateway

FROM alpine:3.22

WORKDIR /app

COPY --from=build /out/gateway-api /app/gateway-api

EXPOSE 8080

ENTRYPOINT ["/app/gateway-api"]