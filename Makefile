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
	@echo "make test			- run dockerized tests locally"
	@echo "make test-ci			- run dockerized tests for Jenkins"
	@echo "make package			- create venv and package lambda function"
	@echo "make publish			- package and publish lambda function"

# Run dockerized tests (can be used locally)
test:
	docker-compose -f docker-compose.test.yml down --remove-orphans
	mkdir -p data conf
	chmod -R 777 data conf
	docker-compose -f docker-compose.test.yml up --exit-code-from local_tests local_tests
	make clean

# Run dockerized tests (used on Jenkins)
test-ci:
	docker-compose -f docker-compose.test.yml down --remove-orphans
	mkdir -p data plugins conf logs
	chmod -R 777 conf
	@IMAGE_TAG=$(IMAGE_TAG) docker-compose -f docker-compose.test.yml up --exit-code-from=ci_tests ci_tests

# Spin down active docker containers.
docker-clean:
	docker-compose -f docker-compose.test.yml down

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
	@echo ""
	@echo "***********************"
	@echo "*   Building integration Service lambda   *"
	@echo "***********************"
	@echo ""
	cd $(WORKING_DIR)/lambda/service; \
  		env GOOS=linux GOARCH=amd64 go build -o $(WORKING_DIR)/lambda/bin/service/pennsieve_integration_service; \
		cd $(WORKING_DIR)/lambda/bin/service/ ; \
			zip -r $(WORKING_DIR)/lambda/bin/service/$(SERVICE_PACKAGE_NAME) .

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
	@echo ""
	@echo "*************************"
	@echo "*   Publishing Service lambda   *"
	@echo "*************************"
	@echo ""
	aws s3 cp $(WORKING_DIR)/lambda/bin/service/$(SERVICE_PACKAGE_NAME) s3://$(LAMBDA_BUCKET)/$(SERVICE_NAME)/service/
	rm -rf $(WORKING_DIR)/lambda/bin/service/$(SERVICE_PACKAGE_NAME)