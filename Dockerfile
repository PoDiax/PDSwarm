# Stage 1: Build Svelte 5 frontend
FROM node:20-alpine AS web-builder
WORKDIR /app/web
COPY web/package*.json ./
RUN npm ci
COPY web/ ./
RUN npm run build

# Stage 2: Build static Go binary
FROM golang:1.24-alpine AS go-builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=web-builder /app/web/dist ./web/dist
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /app/pdswarm ./cmd/pdswarm

# Stage 3: Final minimal runtime image
FROM alpine:latest
RUN apk --no-cache add ca-certificates tzdata
WORKDIR /app
COPY --from=go-builder /app/pdswarm /app/pdswarm

VOLUME ["/data"]
EXPOSE 9000

ENTRYPOINT ["/app/pdswarm"]
CMD ["-data-dir", "/data", "-host", "0.0.0.0", "-port", "9000"]
