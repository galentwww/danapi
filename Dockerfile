# syntax=docker/dockerfile:1

FROM golang:1.21-alpine AS builder

ARG TARGETOS
ARG TARGETARCH

WORKDIR /src

RUN apk add --no-cache ca-certificates tzdata

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH:-amd64} go build -trimpath -ldflags="-s -w" -o /out/dandanplay-middleware .

FROM alpine:3.20

RUN apk add --no-cache ca-certificates tzdata \
	&& addgroup -S app \
	&& adduser -S -G app app

WORKDIR /app

COPY --from=builder /out/dandanplay-middleware /app/dandanplay-middleware

USER app

EXPOSE 8080

ENTRYPOINT ["/app/dandanplay-middleware"]
