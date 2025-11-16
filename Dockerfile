FROM golang:1.25 AS buildgo
WORKDIR /app
COPY . .
RUN CGO_ENABLED=0 go build -o ./build/main .

FROM alpine:3
WORKDIR /app
COPY --from=buildgo /app/build/main /app/main
EXPOSE 1001
ENTRYPOINT ["/app/main"]
