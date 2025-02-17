BIN=./build/bubbly

# env vars for running tests
export BUBBLY_HOST=localhost
export BUBBLY_PORT=8111
export BUBBLY_STORE_PROVIDER=postgres
export POSTGRES_ADDR=postgres:5432
export POSTGRES_USER=postgres
export POSTGRES_DATABASE=bubbly

all: build

.PHONY: build
build:
	go build -o ${BIN}

.PHONY: clean
clean:
	rm ${BIN}

## testing

test: test-unit test-integration

test-unit:
	go test ./...

test-verbose:
	go test -v ./...

test-coverage:
	go test -coverprofile=coverage.txt -covermode=atomic ./...

display-coverage: test-coverage
	go tool cover -html=coverage.txt

test-report:
	go test -coverprofile=coverage.txt -covermode=atomic -json ./... > test_report.json

# The integration tests depend on Bubbly Server and its Store (currently Postgres) being accessible. 
# This is what the env variables in the beginning of this Makefile are for.
# The count flag prevents Go from caching test results as they are dependent on the DB content.
test-integration:
	go test ./integration -tags=integration -count=1

.PHONY: dev
dev:
	docker-compose up --build --abort-on-container-exit --remove-orphans

# Run this target in a separate terminal once `dev` is up to get Postgres console access
psql:
	docker container exec -it postgres psql -U ${POSTGRES_USER}

# Cleanup the docker things: network, volumes, services
cleanup:
	docker-compose down

# Project is CI-enabled with Github Actions. You can run CI locally
# using act (https://github.com/nektos/act). 
# There are some caveats, but the following target should work:
act: 
	act -P ubuntu-latest=golang:latest --env-file act.env -j simple
	
