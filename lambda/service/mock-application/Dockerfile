FROM golang:1.21.0-alpine3.18 as build
WORKDIR /service
# Copy dependencies list
COPY go.mod ./
# Build
COPY main.go .
RUN go build -o main main.go
# Copy artifacts to a clean image
FROM alpine:3.18
COPY --from=build /service/main /main
ENTRYPOINT [ "/main" ]