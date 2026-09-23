FROM golang:1.24-alpine AS builder

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/dev-work-tracker ./cmd/server

FROM builder AS test
RUN go test ./... && go vet ./...

FROM alpine:3.21
RUN apk add --no-cache ca-certificates tzdata \
    && addgroup -S app \
    && adduser -S -G app app
COPY --from=builder /out/dev-work-tracker /usr/local/bin/dev-work-tracker
USER app
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/dev-work-tracker"]
