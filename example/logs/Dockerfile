# phase 1, build stage
FROM golang:1.26-alpine3.23 as builder

WORKDIR /app/code

# Install timezone data and CA certificates explicitly for the final image.
RUN apk add --no-cache tzdata ca-certificates

# Cache dependencies first.
COPY ./go.mod /app/code/go.mod
COPY ./go.sum /app/code/go.sum
RUN go env -w GOPROXY=https://goproxy.cn,direct && go mod download

# Copy the full source tree and build the demo binary.
COPY . /app/code
RUN CGO_ENABLED=0 go build -ldflags "-s -w" -o main ./example/logs


# phase 2, runtime stage
FROM busybox:1.37.0

WORKDIR /app

# Copy runtime assets from the builder stage.
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /app/code/main /app/main
COPY --from=builder /app/code/example/logs/config /app/config
ENV TZ=Asia/Shanghai

# env --------- start
ENV BIND_ADDR=""
ENV COMMIT_HASH=""
ENV ERR_FILE=""
ENV JWT_MAILBOX_KEY=""
ENV LOG_FILE=""
ENV PPROF_LISTEN_ADDR=":6060"
ENV PROMETHEUS_LISTEN_ADDR=":9090"
ENV SERVICE_DOC=""
ENV SERVICE_ID=""
ENV SERVICE_VERSION=""
# env --------- end

EXPOSE 8080
EXPOSE 9090
EXPOSE 6060

# Run the demo service.
CMD ["./main"]
