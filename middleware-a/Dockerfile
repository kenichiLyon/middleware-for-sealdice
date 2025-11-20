FROM golang:1.25-alpine AS build
WORKDIR /src
COPY go.mod .
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags "-s -w" -o /out/middleware-a

FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata
RUN adduser -D -u 10001 app
WORKDIR /app
COPY --from=build /out/middleware-a /app/middleware-a
USER app
EXPOSE 8081
ENTRYPOINT ["/app/middleware-a"]
CMD ["-config","/app/config.json"]