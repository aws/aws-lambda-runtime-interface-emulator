# RELEASE_BUILD_LINKER_FLAGS disables DWARF and symbol table generation to reduce binary size
RELEASE_BUILD_LINKER_FLAGS=-s -w

BINARY_NAME=aws-lambda-rie
DESTINATION=bin/${BINARY_NAME}

compile-with-docker:
	docker run --env GOPROXY=direct -v $(shell pwd):/LambdaRuntimeLocal -w /LambdaRuntimeLocal golang:1.14 make compile-lambda-linux

compile-lambda-linux:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags "${RELEASE_BUILD_LINKER_FLAGS}" -o ${DESTINATION} ./cmd/aws-lambda-rie

tests:
	go test ./...

integ-tests-and-compile: compile-lambda-linux
	python3 -m venv .venv
	.venv/bin/pip install --upgrade pip
	.venv/bin/pip install requests
	.venv/bin/python3 test/integration/local_lambda/end-to-end-test.py

integ-tests-with-docker: compile-with-docker
	python3 -m venv .venv
	.venv/bin/pip install --upgrade pip
	.venv/bin/pip install requests
	.venv/bin/python3 test/integration/local_lambda/end-to-end-test.py

integ-tests:
	python3 -m venv .venv
	.venv/bin/pip install --upgrade pip
	.venv/bin/pip install requests
	.venv/bin/python3 test/integration/local_lambda/end-to-end-test.py
