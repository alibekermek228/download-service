FROM golang:1.26-alpine AS build

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/download-service ./cmd/service

FROM alpine:3.22
RUN adduser -D -u 10001 app
USER app
COPY --from=build /out/download-service /usr/local/bin/download-service
EXPOSE 8081
ENTRYPOINT ["download-service"]
