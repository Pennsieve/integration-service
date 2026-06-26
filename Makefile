.PHONY: help clean test test-ci package publish

LAMBDA_BUCKET ?= "pennsieve-cc-lambda-functions-use1"
WORKING_DIR   ?= "$(shell pwd)"
API_DIR ?= "api"
SERVICE_NAME  ?= "integration-service"
SERVICE_PACKAGE_NAME ?= "integration-service-${IMAGE_TAG}.zip"
PACKAGE_NAME  ?= "${SERVICE_NAME}-${IMAGE_TAG}.zip"

.DEFAULT: help

help:
	@echo "Make Help for $(SERVICE_NAME)"
	@echo ""
	@echo "make package			- build Go lambda and create ZIP file"
	@echo "make publish			- package and publish Go lambda function"

# Build lambda and create ZIP file (Go)
package:
	@echo ""
	@echo "***********************"
	@echo "*   Packaging Go Event lambda   *"
	@echo "***********************"
	@echo ""
	@mkdir -p $(WORKING_DIR)/lambda/bin
	@echo "Building linux/amd64 Go binary..."
	@env GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -ldflags '-s -w' -o $(WORKING_DIR)/lambda/bin/bootstrap ./cmd/event
	@echo "Creating ZIP package $(PACKAGE_NAME)..."
	@cd $(WORKING_DIR)/lambda/bin && zip -j $(WORKING_DIR)/lambda/bin/$(PACKAGE_NAME) bootstrap
	@rm -f $(WORKING_DIR)/lambda/bin/bootstrap

# Copy Service lambda to S3 location
publish:
	@make package
	@echo ""
	@echo "*************************"
	@echo "*   Publishing lambda   *"
	@echo "*************************"
	@echo ""
	aws s3 cp $(WORKING_DIR)/lambda/bin/$(PACKAGE_NAME) s3://$(LAMBDA_BUCKET)/$(SERVICE_NAME)/event_handler/
	rm -rf $(WORKING_DIR)/lambda/bin/$(PACKAGE_NAME)