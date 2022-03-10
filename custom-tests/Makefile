build:
	docker build -t rapid-debug .

run:
	make build
	docker run -d -p 9000:8080 --cap-add=SYS_PTRACE rapid-debug:latest

clean:
	rm -f aws-lambda-rie
	docker rm -f rapid-debug

debug-run: clean build
	cd .. && make compile-lambda-linux
	mv ../bin/aws-lambda-rie-x86_64 ./aws-lambda-rie
	docker run --cap-add=SYS_PTRACE -d -v $(shell pwd)/dlv:/dlv -v $(shell pwd)/lambda-entrypoint.sh:/lambda-entrypoint.sh -v $(shell pwd)/aws-lambda-rie:/usr/local/bin/aws-lambda-rie -p 9000:8080 -p 9001:8081 -p 40000:40000 -p 9563:9563 --env LOCALSTACK_RIE_DEBUG_PORT=40000 --name rapid-debug rapid-debug:latest

