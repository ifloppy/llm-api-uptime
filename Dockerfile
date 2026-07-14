# syntax=docker/dockerfile:1
FROM golang:1.25-bookworm AS builder

ARG VERSION=dev
ARG COMMIT=unknown
ARG BUILD_DATE=unknown

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath \
    -ldflags="-s -w -X llm-api-uptime/internal/buildinfo.Version=${VERSION} -X llm-api-uptime/internal/buildinfo.Commit=${COMMIT} -X llm-api-uptime/internal/buildinfo.BuildDate=${BUILD_DATE}" \
    -o /out/llm-api-uptime . \
    && mkdir -p /out/data

FROM gcr.io/distroless/static-debian12:nonroot

WORKDIR /opt/llm-api-uptime
COPY --from=builder --chown=65532:65532 /out/llm-api-uptime ./llm-api-uptime
COPY --from=builder --chown=65532:65532 /out/data ./data

ENV WEB_ENABLED=true \
    WEB_PUBLIC=true \
    WEB_PORT=8080 \
    DB_PATH=/opt/llm-api-uptime/data/uptime.db \
    UPDATE_AUTO_STAGE=false

VOLUME ["/opt/llm-api-uptime/data"]
EXPOSE 8080
ENTRYPOINT ["/opt/llm-api-uptime/llm-api-uptime"]
CMD ["--server"]
