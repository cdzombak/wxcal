ARG BIN_NAME=wxcal
ARG BIN_VERSION=<unknown>

FROM golang:1-alpine AS builder
ARG BIN_NAME
ARG BIN_VERSION

RUN update-ca-certificates
RUN mkdir -p /ical && chmod 775 /ical

WORKDIR /src
COPY . .
RUN go build -ldflags="-X main.ProductVersion=${BIN_VERSION}" -o ./out/${BIN_NAME} .

FROM scratch
ARG BIN_NAME
COPY --from=builder /src/out/${BIN_NAME} /usr/bin/${BIN_NAME}
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /ical /
VOLUME /ical
WORKDIR /ical
ENTRYPOINT ["/usr/bin/wxcal"]

LABEL license="LGPL v2.1"
LABEL maintainer="Chris Dzombak <https://www.dzombak.com>"
LABEL org.opencontainers.image.authors="Chris Dzombak <https://www.dzombak.com>"
LABEL org.opencontainers.image.url="https://github.com/cdzombak/wxcal"
LABEL org.opencontainers.image.documentation="https://github.com/cdzombak/wxcal/blob/main/README.md"
LABEL org.opencontainers.image.source="https://github.com/cdzombak/wxcal.git"
LABEL org.opencontainers.image.version="${BIN_VERSION}"
LABEL org.opencontainers.image.licenses="LGPL v2.1"
LABEL org.opencontainers.image.title="${BIN_NAME}"
LABEL org.opencontainers.image.description="Generate an iCal feed from the weather.gov forecast API"
