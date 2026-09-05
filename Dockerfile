# syntax=docker/dockerfile:1
FROM golang:1.24-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /out/azula ./cmd/api

FROM alpine:3.21
RUN apk add --no-cache ca-certificates git \
  && adduser -D -H -u 65532 azula
WORKDIR /app
COPY --from=build /out/azula /usr/local/bin/azula
COPY samples ./samples
RUN mkdir -p /app/uploads /app/data && chown -R azula:azula /app
USER azula
EXPOSE 8080
ENTRYPOINT ["azula"]
