.PHONY: help clean compile vet tidy package package-event package-webhook package-dbmigrate publish publish-event publish-webhook publish-dbmigrate

LAMBDA_BUCKET              ?= "pennsieve-cc-lambda-functions-use1"
WORKING_DIR                ?= "$(shell pwd)"
SERVICE_NAME               ?= "integration-service"
EVENT_PACKAGE_NAME         ?= "${SERVICE_NAME}-${IMAGE_TAG}.zip"
WEBHOOK_PACKAGE_NAME       ?= "${SERVICE_NAME}-webhook-${IMAGE_TAG}.zip"
DBMIGRATE_IMAGE_NAME       ?= "pennsieve/${SERVICE_NAME}-dbmigrate:${IMAGE_TAG}"
DBMIGRATE_IMAGE_LATEST     ?= "pennsieve/${SERVICE_NAME}-dbmigrate:latest"

.DEFAULT: help

help:
	@echo "Make Help for $(SERVICE_NAME)"
	@echo ""
	@echo "make compile              - compile all packages"
	@echo "make package              - build all lambdas and DB migrator"
	@echo "make package-event        - build the event consumer lambda ZIP"
	@echo "make package-webhook      - build the webhook receiver lambda ZIP"
	@echo "make package-dbmigrate    - build the DB migration Docker image"
	@echo "make publish              - package and publish all artifacts"
	@echo "make publish-event        - publish event consumer lambda to S3"
	@echo "make publish-webhook      - publish webhook receiver lambda to S3"
	@echo "make publish-dbmigrate    - push DB migration image to ECR"

compile: tidy
	go build ./...

vet:
	go vet ./...

tidy:
	go mod tidy

package: tidy package-event package-webhook package-dbmigrate

# Build event consumer lambda ZIP
package-event:
	@echo ""
	@echo "*********************************************"
	@echo "*   Building Event Consumer lambda          *"
	@echo "*********************************************"
	@echo ""
	@mkdir -p $(WORKING_DIR)/lambda/bin/event
	env GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -tags lambda.norpc -ldflags '-s -w' -o $(WORKING_DIR)/lambda/bin/event/bootstrap ./cmd/event
	cd $(WORKING_DIR)/lambda/bin/event && zip -j $(WORKING_DIR)/lambda/bin/event/$(EVENT_PACKAGE_NAME) bootstrap
	rm -f $(WORKING_DIR)/lambda/bin/event/bootstrap

# Build webhook receiver lambda ZIP
package-webhook:
	@echo ""
	@echo "*********************************************"
	@echo "*   Building Webhook Receiver lambda        *"
	@echo "*********************************************"
	@echo ""
	@mkdir -p $(WORKING_DIR)/lambda/bin/webhook
	env GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -tags lambda.norpc -ldflags '-s -w' -o $(WORKING_DIR)/lambda/bin/webhook/bootstrap ./cmd/webhook
	cd $(WORKING_DIR)/lambda/bin/webhook && zip -j $(WORKING_DIR)/lambda/bin/webhook/$(WEBHOOK_PACKAGE_NAME) bootstrap
	rm -f $(WORKING_DIR)/lambda/bin/webhook/bootstrap

# Build DB migration Docker image
package-dbmigrate:
	@echo ""
	@echo "*********************************************"
	@echo "*   Building Database Migration image       *"
	@echo "*********************************************"
	@echo ""
	docker buildx build --platform linux/amd64 -t $(DBMIGRATE_IMAGE_NAME) -f Dockerfile.cloudwrap-dbmigrate .
	docker tag $(DBMIGRATE_IMAGE_NAME) $(DBMIGRATE_IMAGE_LATEST)

publish: package publish-event publish-webhook publish-dbmigrate

# Publish event consumer lambda to S3
publish-event:
	@echo ""
	@echo "*********************************************"
	@echo "*   Publishing Event Consumer lambda        *"
	@echo "*********************************************"
	@echo ""
	aws s3 cp $(WORKING_DIR)/lambda/bin/event/$(EVENT_PACKAGE_NAME) s3://$(LAMBDA_BUCKET)/$(SERVICE_NAME)/event_handler/
	rm -rf $(WORKING_DIR)/lambda/bin/event

# Publish webhook receiver lambda to S3
publish-webhook:
	@echo ""
	@echo "*********************************************"
	@echo "*   Publishing Webhook Receiver lambda      *"
	@echo "*********************************************"
	@echo ""
	aws s3 cp $(WORKING_DIR)/lambda/bin/webhook/$(WEBHOOK_PACKAGE_NAME) s3://$(LAMBDA_BUCKET)/$(SERVICE_NAME)/webhook_handler/
	rm -rf $(WORKING_DIR)/lambda/bin/webhook

# Push DB migration image to ECR
publish-dbmigrate:
	@echo ""
	@echo "*********************************************"
	@echo "*   Publishing Database Migration image     *"
	@echo "*********************************************"
	@echo ""
	docker push $(DBMIGRATE_IMAGE_NAME)

clean:
	rm -rf $(WORKING_DIR)/lambda/bin