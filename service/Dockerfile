FROM golang:1.27-alpine AS build

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/eia-service ./cmd/server

FROM alpine:3.22

RUN apk add --no-cache ca-certificates && adduser -D -u 10001 eia
COPY --from=build /out/eia-service /usr/local/bin/eia-service
USER eia
ENV EIA_ADDR=:8080
EXPOSE 8080
ENTRYPOINT ["eia-service"]
