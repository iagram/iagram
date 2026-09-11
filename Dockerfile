# Multi-stage: build the UI, build the Go binary with it embedded, ship a
# small image with just the binary, CA certificates and git (tofu init uses
# it for some module sources). OpenTofu itself is downloaded on first run
# into /home/iagram/.iagram, so mount that as a volume to keep it.
FROM node:22-alpine AS web
WORKDIR /src/web
COPY web/package.json web/package-lock.json ./
RUN npm ci --silent
COPY web/ ./
RUN npm run build

FROM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=web /src/internal/web/dist ./internal/web/dist
ARG VERSION=dev
RUN CGO_ENABLED=0 go build -ldflags "-s -w -X github.com/iagram/iagram/internal/cli.Version=${VERSION}" -o /out/iagram ./cmd/iagram

FROM alpine:3.21
RUN apk add --no-cache ca-certificates git \
 && adduser -D -h /home/iagram iagram
COPY --from=build /out/iagram /usr/local/bin/iagram
USER iagram
WORKDIR /work
VOLUME ["/home/iagram/.iagram"]
EXPOSE 7777
ENV IAGRAM_TELEMETRY=0
ENTRYPOINT ["iagram"]
CMD ["up", "--no-open", "--host", "0.0.0.0"]
