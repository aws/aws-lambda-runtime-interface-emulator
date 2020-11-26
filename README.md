## AWS Lambda Runtime Interface Emulator

![Apache-2.0](https://img.shields.io/npm/l/aws-sam-local.svg)

AWS Lambda Runtime Interface Emulator emulates the AWS Lambda Runtime API. This can be used to run your Lambda Functions 
locally through a container tooling, such as Docker. When `aws-lambda-rie` is executed, a 
/2015-03-31/functions/function/invocations endpoint will be stood up within the container that you post data to it in 
order to invoke your function for testing.

Installing

Instructions for installing AWS Lambda Runtime Interface Emulator for your platform

| Platform | Command to install |
|---------|---------
| macOS | `mkdir -p ~/.aws-lambda-rie && curl -Lo ~/.aws-lambda-rie/aws-lambda-rie https://github.com/aws/aws-lambda-runtime-interface-emulator/releases/latest/download/aws-lambda-rie && chmod +x ~/.aws-lambda-rie/aws-lambda-rie` |
| Linux | `mkdir -p ~/.aws-lambda-rie && curl -Lo ~/.aws-lambda-rie/aws-lambda-rie https://github.com/aws/aws-lambda-runtime-interface-emulator/releases/latest/download/aws-lambda-rie && chmod +x ~/.aws-lambda-rie/aws-lambda-rie` |
| Windows | `Invoke-WebRequest -OutFile 'C:\Program Files\aws lambda\aws-lambda-rie' https://github.com/aws/aws-lambda-runtime-interface-emulator/releases/latest/download/aws-lambda-rie` |


Getting started

Download the `aws-lambda-rie` binary (see Installing instructions above).
Once downloaded you need to mount the binary into Lambda Image locally and use it as the entrypoint.

Linux and macOS:
docker run -d -v ~/.aws-lambda-rie:/aws-lambda -p 9000:8080 \
    --entrypoint /aws-lambda/aws-lambda-rie myfunction:latest \
    /bootstrap-with-handler app.lambda_handler
    
Windows:
docker run -d -v /c/Program Files/aws lambda:/aws-lambda -p 9000:8080 \
    --entrypoint /aws-lambda/aws-lambda-rie myfunction:latest \
    /bootstrap-with-handler app.lambda_handler

How to configure 

`aws-lambda-rie` can be configured through Environment Variables within the local running Image. 
You can configure your credentials by setting:
* AWS_ACCESS_KEY_ID
* AWS_SECRET_ACCESS_KEY
* AWS_SESSION_TOKEN
* AWS_REGION

You can configure timeout by setting AWS_LAMBDA_FUNCTION_TIMEOUT to the number of seconds you want your function to timeout in.

The rest of these Environment Variables can be set to match AWS Lambda's environment but are not required.
* AWS_LAMBDA_FUNCTION_VERSION
* AWS_LAMBDA_FUNCION_NAME
* AWS_LAMBDA_MEMORY_SIZE

Limitations

* There is no X-Ray support locally
* Only supports linux x84-64 architectures
* The emulator runs in a running container

## Security

See [CONTRIBUTING](CONTRIBUTING.md#security-issue-notifications) for more information.

## License

This project is licensed under the Apache-2.0 License.
