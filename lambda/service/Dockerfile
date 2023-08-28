FROM golang:1.21.0-alpine as build
WORKDIR /service
# Copy files
COPY . ./
# Build
RUN go build -o main main.go
# Copy artifacts to a clean image
FROM alpine:3.18
RUN apk add --update curl
COPY --from=build /service/main /main
COPY ./entry_script.sh /entry_script.sh
# install aws-lambda-rie
RUN curl -Lo aws-lambda-rie https://github.com/aws/aws-lambda-runtime-interface-emulator/releases/latest/download/aws-lambda-rie-arm64 \
&& chmod +x aws-lambda-rie && mv aws-lambda-rie /usr/local/bin/aws-lambda-rie

ENTRYPOINT [ "/entry_script.sh" ]