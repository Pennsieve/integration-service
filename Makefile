.PHONY: help clean clean-ci docker-clean docker-image-clean compile vet tidy package package-event package-webhook package-notification package-dbmigrate publish publish-event publish-webhook publish-notification publish-dbmigrate down local-services test test-docker test-ci build-postgres

LAMBDA_BUCKET              ?= "pennsieve-cc-lambda-functions-use1"
WORKING_DIR                ?= "$(shell pwd)"
SERVICE_NAME               ?= "integration-service"
EVENT_PACKAGE_NAME         ?= "${SERVICE_NAME}-${IMAGE_TAG}.zip"
WEBHOOK_PACKAGE_NAME       ?= "${SERVICE_NAME}-webhook-${IMAGE_TAG}.zip"
NOTIFICATION_PACKAGE_NAME  ?= "${SERVICE_NAME}-notification-${IMAGE_TAG}.zip"
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
	@echo "make package-notification - build the notification API lambda ZIP"
	@echo "make package-dbmigrate    - build the DB migration Docker image"
	@echo "make publish              - package and publish all artifacts"
	@echo "make publish-event        - publish event consumer lambda to S3"
	@echo "make publish-webhook      - publish webhook receiver lambda to S3"
	@echo "make publish-notification - publish notification API lambda to S3"
	@echo "make publish-dbmigrate    - push DB migration image to ECR"
	@echo "make test                 - run go test locally against a migrated Postgres container"
	@echo "make test-docker          - run go test inside the docker network (used by Jenkins)"
	@echo "make test-ci              - run go test in CI against pre-seeded postgres image"
	@echo "make build-postgres       - build and push pennsievedb-integration seed image (run after adding migrations)"
	@echo "make clean                - tear down docker-compose resources and remove build artifacts"
	@echo "make clean-ci             - clean + remove dbmigrate Docker images"
	@echo "make down                 - tear down docker-compose.test.yml resources (alias for docker-clean)"

compile:
	go build ./...

vet:
	go vet ./...

tidy:
	go mod tidy

package: package-event package-webhook package-notification package-dbmigrate

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

# Build notification API lambda ZIP
package-notification:
	@echo ""
	@echo "*********************************************"
	@echo "*   Building Notification API lambda        *"
	@echo "*********************************************"
	@echo ""
	@mkdir -p $(WORKING_DIR)/lambda/bin/notification
	env GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -tags lambda.norpc -ldflags '-s -w' -o $(WORKING_DIR)/lambda/bin/notification/bootstrap ./cmd/notification
	cd $(WORKING_DIR)/lambda/bin/notification && zip -j $(WORKING_DIR)/lambda/bin/notification/$(NOTIFICATION_PACKAGE_NAME) bootstrap
	rm -f $(WORKING_DIR)/lambda/bin/notification/bootstrap

# Build DB migration Docker image
package-dbmigrate:
	@echo ""
	@echo "*********************************************"
	@echo "*   Building Database Migration image       *"
	@echo "*********************************************"
	@echo ""
	docker buildx build --platform linux/amd64 -t $(DBMIGRATE_IMAGE_NAME) -f Dockerfile.cloudwrap-dbmigrate .
	docker tag $(DBMIGRATE_IMAGE_NAME) $(DBMIGRATE_IMAGE_LATEST)

publish: package publish-event publish-webhook publish-notification publish-dbmigrate

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

# Publish notification API lambda to S3
publish-notification:
	@echo ""
	@echo "*********************************************"
	@echo "*   Publishing Notification API lambda      *"
	@echo "*********************************************"
	@echo ""
	aws s3 cp $(WORKING_DIR)/lambda/bin/notification/$(NOTIFICATION_PACKAGE_NAME) s3://$(LAMBDA_BUCKET)/$(SERVICE_NAME)/notification_handler/
	rm -rf $(WORKING_DIR)/lambda/bin/notification

# Push DB migration image to ECR
publish-dbmigrate:
	@echo ""
	@echo "*********************************************"
	@echo "*   Publishing Database Migration image     *"
	@echo "*********************************************"
	@echo ""
	docker push $(DBMIGRATE_IMAGE_NAME)

docker-clean:
	docker compose -f docker-compose.test.yml -f docker-compose.build-postgres.yml down --remove-orphans -v

docker-image-clean:
	docker rmi -f $(DBMIGRATE_IMAGE_NAME) $(DBMIGRATE_IMAGE_LATEST)

clean: docker-clean
	rm -rf $(WORKING_DIR)/lambda/bin

clean-ci: clean docker-image-clean

# Alias kept for local developer convenience
down: docker-clean

# Start pennsievedb and run this repo's own dbmigrate image against it, then exit
local-services: docker-clean
	docker compose -f docker-compose.test.yml -f docker-compose.local.override.yml up --build -d pennsievedb
	docker compose -f docker-compose.test.yml -f docker-compose.local.override.yml run --rm dbmigrate

# Run go test locally against a real, migrated Postgres container
test: local-services
	set -a; source ./test.env; set +a; go test -v -count=1 ./...

# Run the full test suite inside the docker network (used by Jenkins)
test-docker: docker-clean
	docker compose -f docker-compose.test.yml up --build --abort-on-container-exit --exit-code-from test test

# Run tests in CI against the pre-seeded pennsievedb-integration image (no dbmigrate step)
test-ci: docker-clean
	@IMAGE_TAG=$(IMAGE_TAG) docker compose -f docker-compose.test.yml up --exit-code-from=ci-tests ci-tests

# Build pennsievedb-integration seed image from this branch's migrations and push to ECR.
# Run this whenever new migration files are added, then update the image tag in
# docker-compose.test.yml (pennsievedb-integration service) to match the new seed tag.
build-postgres: package-dbmigrate
	./build-postgres.sh