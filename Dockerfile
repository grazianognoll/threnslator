# syntax=docker/dockerfile:1
FROM golang:1.22 AS build
WORKDIR /src
COPY go.mod .
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /app


FROM alpine:latest as cloudflared
RUN wget -q https://github.com/cloudflare/cloudflared/releases/latest/download/cloudflared-linux-amd64 -O /usr/local/bin/cloudflared && \
    chmod +x /usr/local/bin/cloudflared

FROM alpine:latest
WORKDIR /app
COPY --from=build /app /app/app
COPY --from=cloudflared /usr/local/bin/cloudflared /usr/local/bin/cloudflared
EXPOSE 8080

# Default environment variables
ENV PORT=8080
ENV TUNNEL_TOKEN=""

# Copy startup script
COPY start.sh /app/
RUN chmod +x /app/start.sh

# Note: For local development only - remove this line for production!
# COPY .env /app/.env  # Uncomment this line to embed .env file (NOT recommended for production)

CMD ["/bin/sh", "/app/start.sh"]