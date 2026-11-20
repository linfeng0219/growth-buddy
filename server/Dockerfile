FROM golang:1.23-bookworm AS builder
WORKDIR /src
RUN apt-get update && apt-get install -y --no-install-recommends gcc libsqlite3-dev \
    && rm -rf /var/lib/apt/lists/*
COPY . .
RUN CGO_ENABLED=1 CGO_LDFLAGS="-Wl,-Bstatic -lsqlite3 -Wl,-Bdynamic -lm -ldl -lpthread" \
    go build -trimpath -ldflags="-s -w -X main.SystemVersion=$(TZ=Asia/Shanghai date +%Y.%m.%d-%H%M)" -o /out/growth-buddy-server .

FROM debian:12-slim
WORKDIR /app
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates tzdata \
    && rm -rf /var/lib/apt/lists/*
ENV TZ=Asia/Shanghai
COPY --from=builder /out/growth-buddy-server /app/growth-buddy-server
RUN chmod +x /app/growth-buddy-server && mkdir -p /app/data/releases
EXPOSE 3000
CMD ["/app/growth-buddy-server"]
