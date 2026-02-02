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
	@echo "make package			- create venv and package lambda function"
	@echo "make publish			- package and publish lambda function"

# Build lambda and create ZIP file
package:
	@echo ""
	@echo "***********************"
	@echo "*   Packaging Python lambda   *"
	@echo "***********************"
	@echo ""
	cd $(WORKING_DIR)/lambda/ ; \
		mkdir bin; \
		cd event_lambda/; \
			zip -r $(WORKING_DIR)/lambda/bin/$(PACKAGE_NAME) .

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