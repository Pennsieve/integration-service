FROM golang:1.20.2-alpine3.16 as build
WORKDIR /service
# Copy dependencies list
COPY go.mod go.sum ./
# Build
COPY main.go . 
COPY handler ./handler
COPY models ./models
COPY store ./store
COPY trigger ./trigger
COPY clients ./clients
COPY utils ./utils
RUN go build -o main main.go
# Copy artifacts to a clean image
FROM alpine:3.16
RUN apk add --update curl
COPY --from=build /service/main /main
COPY ./entry_script.sh /entry_script.sh
# install aws-lambda-rie
RUN curl -Lo aws-lambda-rie https://github.com/aws/aws-lambda-runtime-interface-emulator/releases/latest/download/aws-lambda-rie-arm64 \
&& chmod +x aws-lambda-rie && mv aws-lambda-rie /usr/local/bin/aws-lambda-rie

ENTRYPOINT [ "/entry_script.sh" ]