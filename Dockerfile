FROM golang:1.26-alpine AS build

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -buildvcs=false -ldflags="-s -w" -o /out/discord-free-cloud ./cmd/app

FROM alpine:3.22

RUN apk add --no-cache ca-certificates
WORKDIR /app
COPY --from=build /out/discord-free-cloud /app/discord-free-cloud
COPY frontend /app/frontend

ENV HOST=0.0.0.0 \
    NO_BROWSER=true \
    NO_PROMPT=true \
    DB_PATH=/var/data/drive.db

EXPOSE 8080
VOLUME ["/var/data"]
CMD ["/app/discord-free-cloud"]
