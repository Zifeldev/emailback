FROM golang:1.24-alpine AS builder
WORKDIR /src

COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

COPY . .


RUN --mount=type=cache,target=/go/pkg/mod \
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -trimpath -ldflags="-s -w" -o /out/emailback ./service/cmd/main.go

FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata

RUN addgroup -S app && adduser -S -G app appuser

WORKDIR /app
COPY --from=builder /out/emailback /app/emailback

EXPOSE 8080
USER appuser

ENV GIN_MODE=release

ENTRYPOINT ["/app/emailback"]