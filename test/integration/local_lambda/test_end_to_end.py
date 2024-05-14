# Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
# SPDX-License-Identifier: Apache-2.0

from subprocess import Popen, PIPE
from unittest import TestCase, main
from pathlib import Path
import base64
import json
import time
import os
import requests
from contextlib import contextmanager
from parameterized import parameterized

SLEEP_TIME = 1.5
DEFAULT_1P_ENTRYPOINT = "/lambda-entrypoint.sh"
ARCHS = ["x86_64", "arm64", ""]



class TestEndToEnd(TestCase):
    ARCH = os.environ.get('TEST_ARCH', "")
    PORT = os.environ.get('TEST_PORT', 8002)
    @classmethod
    def setUpClass(cls):
        testdata_path = Path(__file__).resolve().parents[1].joinpath("testdata")
        dockerfile_path = testdata_path.joinpath("Dockerfile-allinone")
        cls.path_to_binary = Path().resolve().joinpath("bin")

        # build image
        image_name_base = "aws-lambda-local:testing"
        cls.image_name = image_name_base if cls.ARCH == "" else f"{image_name_base}-{cls.ARCH}"
        architecture = cls.ARCH if cls.ARCH == "arm64" else "amd64"
        docker_arch = cls.ARCH if cls.ARCH == "arm64" else "x86_64"
        
        build_cmd = [
            "docker",
            "build",
            "--platform",
            f"linux/{architecture}",
            "-t",
            cls.image_name,
            "-f",
            str(dockerfile_path),
            str(testdata_path),
            "--build-arg",
            f"IMAGE_ARCH={docker_arch}",
        ]
        Popen(build_cmd).communicate()

    @classmethod
    def tearDownClass(cls):
        Popen(f"docker rmi {cls.image_name}".split(" ")).communicate()

    def tagged_name(self, name):
        tag = self.get_tag()
        return (name + tag, "aws-lambda-rie" + tag, self.image_name)

    def get_tag(self):
        return "" if self.ARCH == "" else str(f"-{self.ARCH}")
    
    def run_command(self, cmd):
        Popen(cmd.split(" ")).communicate()
    
    def sleep_1s(self):
        time.sleep(SLEEP_TIME)

    def invoke_function(self, json={}, headers={}):
        return requests.post(
            f"http://localhost:{self.PORT}/2015-03-31/functions/function/invocations",
            json=json,
            headers=headers,
        )

    @contextmanager
    def create_container(self, param, image):
        try:
            platform = "x86_64" if self.ARCH == "" else self.ARCH
            cmd_full = f"docker run --platform linux/{platform} {param}"
            self.run_command(cmd_full)
        
            # sleep 1s to give enough time for the endpoint to be up to curl
            self.sleep_1s()
            yield
        except Exception as e:
            print(f"An error occurred while executing cmd: {cmd_full}. error: {e}")
            raise e
        finally:
            self.run_command(f"docker stop {image}")
            self.run_command(f"docker rm -f {image}")
        

    def test_env_var_with_equal_sign(self):
        image, rie, image_name = self.tagged_name("envvarcheck")
        params = f"--name {image} -d -v {self.path_to_binary}:/local-lambda-runtime-server -p {self.PORT}:8080 --entrypoint /local-lambda-runtime-server/{rie} {image_name} {DEFAULT_1P_ENTRYPOINT} main.check_env_var_handler"
        
        with self.create_container(params, image):
            r = self.invoke_function()
        
            self.assertEqual(b'"4=4"', r.content)


    def test_two_invokes(self):
        image, rie, image_name = self.tagged_name("twoinvokes")

        params = f"--name {image} -d -v {self.path_to_binary}:/local-lambda-runtime-server -p {self.PORT}:8080 --entrypoint /local-lambda-runtime-server/{rie} {image_name} {DEFAULT_1P_ENTRYPOINT} main.success_handler"

        with self.create_container(params, image):
            r = self.invoke_function()
        
            self.assertEqual(b'"My lambda ran succesfully"', r.content)

            # Make sure we can invoke the function twice
            r = self.invoke_function()
            
            self.assertEqual(b'"My lambda ran succesfully"', r.content)


    def test_lambda_function_arn_exists(self):
        image, rie, image_name = self.tagged_name("arnexists")

        params = f"--name {image} -d -v {self.path_to_binary}:/local-lambda-runtime-server -p {self.PORT}:8080 --entrypoint /local-lambda-runtime-server/{rie} {image_name} {DEFAULT_1P_ENTRYPOINT} main.assert_lambda_arn_in_context"

        with self.create_container(params, image):
            r = self.invoke_function()
        
            self.assertEqual(b'"My lambda ran succesfully"', r.content)


    def test_lambda_function_arn_exists_with_defining_custom_name(self):
        image, rie, image_name = self.tagged_name("customname")

        params = f"--name {image} --env AWS_LAMBDA_FUNCTION_NAME=MyCoolName -d -v {self.path_to_binary}:/local-lambda-runtime-server -p {self.PORT}:8080 --entrypoint /local-lambda-runtime-server/{rie} {image_name} {DEFAULT_1P_ENTRYPOINT} main.assert_lambda_arn_in_context"
        
        with self.create_container(params, image):
            r = self.invoke_function()
        
            self.assertEqual(b'"My lambda ran succesfully"', r.content)


    def test_timeout_invoke(self):
        image, rie, image_name = self.tagged_name("timeout")

        params = f"--name {image} -d --env AWS_LAMBDA_FUNCTION_TIMEOUT=1 -v {self.path_to_binary}:/local-lambda-runtime-server -p {self.PORT}:8080 --entrypoint /local-lambda-runtime-server/{rie} {image_name} {DEFAULT_1P_ENTRYPOINT} main.sleep_handler"

        with self.create_container(params, image):
            r = self.invoke_function()
        
            self.assertEqual(b"Task timed out after 1.00 seconds", r.content)


    def test_exception_returned(self):
        image, rie, image_name = self.tagged_name("exception")

        params = f"--name {image} -d -v {self.path_to_binary}:/local-lambda-runtime-server -p {self.PORT}:8080 --entrypoint /local-lambda-runtime-server/{rie} {image_name} {DEFAULT_1P_ENTRYPOINT} main.exception_handler"

        with self.create_container(params, image):
            r = self.invoke_function()
            
            # Except the 3 fields assrted below, there's another field `request_id` included start from python3.12 runtime.
            # We should ignore asserting the field `request_id` for it is in a UUID like format and changes everytime
            result = r.json()
            self.assertEqual(result["errorMessage"], "Raising an exception")
            self.assertEqual(result["errorType"], "Exception")
            self.assertEqual(result["stackTrace"], ["  File \"/var/task/main.py\", line 13, in exception_handler\n    raise Exception(\"Raising an exception\")\n"])


    def test_context_get_remaining_time_in_three_seconds(self):
        image, rie, image_name = self.tagged_name("remaining_time_in_three_seconds")

        params = f"--name {image} -d --env AWS_LAMBDA_FUNCTION_TIMEOUT=3 -v {self.path_to_binary}:/local-lambda-runtime-server -p {self.PORT}:8080 --entrypoint /local-lambda-runtime-server/{rie} {image_name} {DEFAULT_1P_ENTRYPOINT} main.check_remaining_time_handler"

        with self.create_container(params, image):
            r = self.invoke_function()
        
            # Execution time is not decided, 1.0s ~ 3.0s is a good estimation
            self.assertLess(int(r.content), 3000)
            self.assertGreater(int(r.content), 1000)


    def test_context_get_remaining_time_in_ten_seconds(self):
        image, rie, image_name = self.tagged_name("remaining_time_in_ten_seconds")

        params = f"--name {image} -d --env AWS_LAMBDA_FUNCTION_TIMEOUT=10 -v {self.path_to_binary}:/local-lambda-runtime-server -p {self.PORT}:8080 --entrypoint /local-lambda-runtime-server/{rie} {image_name} {DEFAULT_1P_ENTRYPOINT} main.check_remaining_time_handler"

        with self.create_container(params, image):
            r = self.invoke_function()
        
            # Execution time is not decided, 8.0s ~ 10.0s is a good estimation
            self.assertLess(int(r.content), 10000)
            self.assertGreater(int(r.content), 8000)


    def test_context_get_remaining_time_in_default_deadline(self):
        image, rie, image_name = self.tagged_name("remaining_time_in_default_deadline")

        params = f"--name {image} -d -v {self.path_to_binary}:/local-lambda-runtime-server -p {self.PORT}:8080 --entrypoint /local-lambda-runtime-server/{rie} {image_name} {DEFAULT_1P_ENTRYPOINT} main.check_remaining_time_handler"

        with self.create_container(params, image):
            r = self.invoke_function()

            # Executation time is not decided, 298.0s ~ 300.0s is a good estimation
            self.assertLess(int(r.content), 300000)
            self.assertGreater(int(r.content), 298000)


    def test_invoke_with_pre_runtime_api_runtime(self):
        image, rie, image_name = self.tagged_name("pre-runtime-api")

        params = f"--name {image} -d -v {self.path_to_binary}:/local-lambda-runtime-server -p {self.PORT}:8080 --entrypoint /local-lambda-runtime-server/{rie} {image_name} {DEFAULT_1P_ENTRYPOINT} main.success_handler"

        with self.create_container(params, image):
            r = self.invoke_function()
        
            self.assertEqual(b'"My lambda ran succesfully"', r.content)


    def test_function_name_is_overriden(self):
        image, rie, image_name = self.tagged_name("assert-overwritten")

        params = f"--name {image} -d --env AWS_LAMBDA_FUNCTION_NAME=MyCoolName -v {self.path_to_binary}:/local-lambda-runtime-server -p {self.PORT}:8080 --entrypoint /local-lambda-runtime-server/{rie} {image_name} {DEFAULT_1P_ENTRYPOINT} main.assert_env_var_is_overwritten"

        with self.create_container(params, image):
            r = self.invoke_function()
        
            self.assertEqual(b'"My lambda ran succesfully"', r.content)


    def test_port_override(self):
        image, rie, image_name = self.tagged_name("port_override")

        # Use port 8081 inside the container instead of 8080
        params = f"--name {image} -d -v {self.path_to_binary}:/local-lambda-runtime-server -p {self.PORT}:8081 --entrypoint /local-lambda-runtime-server/{rie} {image_name} {DEFAULT_1P_ENTRYPOINT} main.success_handler --runtime-interface-emulator-address 0.0.0.0:8081"

        with self.create_container(params, image):
            r = self.invoke_function()
        
            self.assertEqual(b'"My lambda ran succesfully"', r.content)


    def test_custom_client_context(self):
        image, rie, image_name = self.tagged_name("custom_client_context")

        params = f"--name {image} -d -v {self.path_to_binary}:/local-lambda-runtime-server -p {self.PORT}:8080 --entrypoint /local-lambda-runtime-server/{rie} {image_name} {DEFAULT_1P_ENTRYPOINT} main.custom_client_context_handler"

        with self.create_container(params, image):
            r = self.invoke_function(headers={
                "X-Amz-Client-Context": base64.b64encode(json.dumps({
                    "custom": {
                        "foo": "bar",
                        "baz": 123,
                    }
                }).encode('utf8')).decode('utf8'),
            })
            content = json.loads(r.content)
            self.assertEqual("bar", content["foo"])
            self.assertEqual(123, content["baz"])


if __name__ == "__main__":
    main()
