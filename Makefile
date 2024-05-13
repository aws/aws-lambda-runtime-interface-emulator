# RELEASE_BUILD_LINKER_FLAGS disables DWARF and symbol table generation to reduce binary size
RELEASE_BUILD_LINKER_FLAGS=-s -w

BINARY_NAME=aws-lambda-rie
ARCH=x86_64
GO_ARCH_old := amd64
GO_ARCH_x86_64 := amd64
GO_ARCH_arm64 := arm64
DESTINATION_old:= bin/${BINARY_NAME}
DESTINATION_x86_64 := bin/${BINARY_NAME}-x86_64
DESTINATION_arm64 := bin/${BINARY_NAME}-arm64

run_in_docker = docker run --env GOPROXY=direct -v $(shell pwd):/LambdaRuntimeLocal -w /LambdaRuntimeLocal golang:1.22 $(1)

compile-with-docker-all:
	$(call run_in_docker, make compile-lambda-linux-all)

compile-lambda-linux-all:
	make ARCH=x86_64 compile-lambda-linux
	make ARCH=arm64 compile-lambda-linux
	make ARCH=old compile-lambda-linux

compile-with-docker:
	$(call run_in_docker, make ARCH=${ARCH} compile-lambda-linux)

compile-lambda-linux:
	CGO_ENABLED=0 GOOS=linux GOARCH=${GO_ARCH_${ARCH}} go build -buildvcs=false -ldflags "${RELEASE_BUILD_LINKER_FLAGS}" -o ${DESTINATION_${ARCH}} ./cmd/aws-lambda-rie

tests-with-docker:
	$(call run_in_docker, make tests)

tests:
	go test ./...

integ-tests-and-compile: tests 
	make compile-lambda-linux-all
	make integ-tests

integ-tests-with-docker: tests-with-docker
	make compile-with-docker-all
	make integ-tests

prep-python:
	python3 -m venv .venv
	.venv/bin/pip install --upgrade pip
	.venv/bin/pip install requests parameterized

exec-python-e2e-test:
	.venv/bin/python3 test/integration/local_lambda/test_end_to_end.py

integ-tests:
	make prep-python
	docker run --rm --privileged multiarch/qemu-user-static --reset -p yes
	make TEST_ARCH=x86_64 TEST_PORT=8002 exec-python-e2e-test
	make TEST_ARCH=arm64 TEST_PORT=9002 exec-python-e2e-test
	make TEST_ARCH="" TEST_PORT=9052 exec-python-e2e-test

integ-tests-with-docker-x86-64:
	make ARCH=x86_64 compile-with-docker
	make prep-python
	make TEST_ARCH=x86_64 TEST_PORT=8002 exec-python-e2e-test

integ-tests-with-docker-arm64:
	make ARCH=arm64 compile-with-docker
	make prep-python
	docker run --rm --privileged multiarch/qemu-user-static --reset -p yes
	make TEST_ARCH=arm64 TEST_PORT=9002 exec-python-e2e-test

integ-tests-with-docker-old:
	make ARCH=old compile-with-docker
	make prep-python
	make TEST_ARCH="" TEST_PORT=9052 exec-python-e2e-test

check-binaries: prep-python
	.venv/bin/pip install cve-bin-tool
	.venv/bin/python -m cve_bin_tool.cli bin/ -r go -d REDHAT,OSV,GAD,CURL --no-0-cve-report -f csv
