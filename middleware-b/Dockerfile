FROM golang:1.25-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags "-s -w" -o /out/middleware-b

FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata
RUN adduser -D -u 10001 app
WORKDIR /app
COPY --from=build /out/middleware-b /app/middleware-b
USER app
EXPOSE 8082
VOLUME ["/data/uploads"]
ENTRYPOINT ["/app/middleware-b"]
CMD ["-config","/app/config.json"]