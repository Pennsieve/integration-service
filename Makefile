.PHONY: help clean compile vet tidy package package-event package-webhook package-notification package-dbmigrate publish publish-event publish-webhook publish-notification publish-dbmigrate build-postgres test test-ci docker-clean

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
	@echo "make build-postgres       - build a seeded Postgres image with this repo's migrations applied"
	@echo "make test                 - run go test locally against the seeded Postgres image"
	@echo "make test-ci              - run go test in CI (no TTY) against the seeded Postgres image"
	@echo "make docker-clean         - tear down build-postgres/test docker compose resources"

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

clean:
	rm -rf $(WORKING_DIR)/lambda/bin

# Build a seeded Postgres image with this repo's migrations already applied,
# tagged from the latest migration file's timestamp (see build-postgres.sh).
build-postgres: package-dbmigrate
	@echo ""
	@echo "*********************************************"
	@echo "*   Building seeded Postgres image          *"
	@echo "*********************************************"
	@echo ""
	./build-postgres.sh

POSTGRES_TAG := $(shell ls -1 internal/dbmigrate/migrations/*/*.up.sql | xargs -n1 basename | sort | tail -1 | cut -d'_' -f1)

# Run go test locally against the seeded Postgres image
test: docker-clean
	POSTGRES_TAG=$(POSTGRES_TAG) docker compose -f docker-compose.test.yml up --build --abort-on-container-exit --exit-code-from test
	$(MAKE) docker-clean

# Run go test in CI against the seeded Postgres image (no interactive TTY)
test-ci: docker-clean
	POSTGRES_TAG=$(POSTGRES_TAG) docker compose -f docker-compose.test.yml up --build --abort-on-container-exit --exit-code-from test
	$(MAKE) docker-clean

# Tear down build-postgres/test docker compose resources
docker-clean:
	docker compose -f docker-compose.build-postgres.yml down -v --remove-orphans || true
	docker compose -f docker-compose.test.yml down -v --remove-orphans || true