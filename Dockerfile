# Build stage
FROM golang:1.24-alpine AS builder

RUN apk add --no-cache git make
RUN go install github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@latest

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY api/openapi.yml api/
COPY configs/oapi/ configs/oapi/
COPY Makefile .

RUN make generate-models generate-server

COPY cmd/ cmd/
COPY internal/ internal/

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags='-w -s -extldflags "-static"' \
    -a -installsuffix cgo \
    -o app ./cmd

# Final stage - minimal alpine image
FROM alpine:latest

RUN apk --no-cache add ca-certificates tzdata wget && \
    update-ca-certificates

RUN addgroup -g 1000 appuser && \
    adduser -D -u 1000 -G appuser appuser

WORKDIR /app

COPY --from=builder /app/app .
RUN chown -R appuser:appuser /app

USER appuser

EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1

CMD ["./app"]
