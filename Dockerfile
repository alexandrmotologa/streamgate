# Stage 1: Build Web Console (React 19)
FROM node:22-alpine AS web-builder
WORKDIR /app/web
COPY web/package*.json ./
RUN npm ci
COPY web/ ./
RUN npm run build

# Stage 2: Build Go Gateway Daemon
FROM golang:1.24-alpine AS go-builder
WORKDIR /app
RUN apk add --no-cache git ca-certificates
COPY go.mod go.sum ./
RUN go mod download
COPY . .
# Copy compiled web assets into web/dist
COPY --from=web-builder /app/web/dist ./web/dist
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o /streamgate ./cmd/streamgate

# Stage 3: Minimal Scratch Runtime
FROM alpine:3.21
RUN apk --no-cache add ca-certificates tzdata
WORKDIR /
COPY --from=go-builder /streamgate /streamgate

EXPOSE 8080
HEALTHCHECK --interval=10s --timeout=3s --retries=3 CMD wget -qO- http://127.0.0.1:8080/healthz || exit 1

ENTRYPOINT ["/streamgate"]
CMD ["-host", "0.0.0.0", "-port", "8080"]
