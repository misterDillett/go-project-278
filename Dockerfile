FROM node:20-alpine AS frontend-builder
WORKDIR /build/frontend

COPY package*.json ./

RUN --mount=type=cache,target=/root/.npm \
  npm ci --prefer-offline --no-audit

RUN mkdir -p /app/public
RUN cp -r node_modules/@hexlet/project-url-shortener-frontend/dist/* /app/public/ || true

FROM golang:1.25-alpine AS backend-builder
RUN apk add --no-cache git
WORKDIR /build/code

COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
  go mod download

RUN go install github.com/pressly/goose/v3/cmd/goose@latest

COPY . .

RUN --mount=type=cache,target=/root/.cache/go-build \
  CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /build/app .


FROM alpine:3.19

RUN apk add --no-cache ca-certificates tzdata bash caddy

WORKDIR /app

COPY --from=backend-builder /build/app /app/bin/app

COPY --from=frontend-builder /app/public /app/public

COPY --from=backend-builder /build/code/db/migrations /app/db/migrations
COPY --from=backend-builder /go/bin/goose /usr/local/bin/goose

COPY bin/run.sh /app/bin/run.sh
COPY Caddyfile /etc/caddy/Caddyfile

RUN chmod +x /app/bin/run.sh

EXPOSE 80

CMD ["/app/bin/run.sh"]
