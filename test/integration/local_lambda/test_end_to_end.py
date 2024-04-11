# Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
# SPDX-License-Identifier: Apache-2.0

from subprocess import Popen, PIPE
from unittest import TestCase, main
from pathlib import Path
import time
import os
import requests
from contextlib import contextmanager
from parameterized import parameterized

SLEEP_TIME = 2
DEFAULT_1P_ENTRYPOINT = "/lambda-entrypoint.sh"
ARCHS = ["x86_64", "arm64", ""]



class TestEndToEnd(TestCase):
    ARCH = os.environ.get('TEST_ARCH', "")
    PORT = os.environ.get('TEST_PORT', 8002)
    @classmethod
    def setUpClass(cls):
        testdata_path = Path(__file__).resolve().parents[1].joinpath("testdata")
        dockerfile_path = testdata_path.joinpath("Dockerfile-allinone")
        cls.image_name = "aws-lambda-local:testing"
        cls.path_to_binary = Path().resolve().joinpath("bin")

        # build image
        image_name = cls.image_name if cls.ARCH == "" else f"{cls.image_name}-{cls.ARCH}"
        architecture = cls.ARCH if cls.ARCH == "arm64" else "amd64"
        docker_arch = cls.ARCH if cls.ARCH == "arm64" else "x86_64"
        
        build_cmd = [
            "docker",
            "build",
            "--platform",
            f"linux/{architecture}",
            "-t",
            image_name,
            "-f",
            str(dockerfile_path),
            str(testdata_path),
            "--build-arg",
            f"IMAGE_ARCH={docker_arch}",
        ]
        print (build_cmd)
        Popen(build_cmd).communicate()

    @classmethod
    def tearDownClass(cls):
        arch_tag = "" if cls.ARCH == "" else f"-{cls.ARCH}"
        Popen(f"docker rmi {cls.image_name}{arch_tag}".split(" ")).communicate()

    def tagged_name(self, name, architecture):
        tag = self.get_tag(architecture)
        return (name + tag, "aws-lambda-rie" + tag, self.image_name + tag)

    def get_tag(self, architecture):
        return "" if architecture == "" else str(f"-{architecture}")
    
    def run_command(self, cmd):
        Popen(cmd.split(" ")).communicate()
    
    def sleep_1s(self):
        time.sleep(SLEEP_TIME)
        
    def invoke_function(self, port):
        return requests.post(
            f"http://localhost:{port}/2015-03-31/functions/function/invocations", json={}
        )
        
    @contextmanager
    def create_container(self, cmd, image):
        try:
            platform = "x86_64" if self.ARCH == "" else self.ARCH
            cmd_full = f"docker run --platform linux/{platform} {cmd}"
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
        

    def test_env_var_with_equal_sign(self, arch=ARCH, port=PORT):
        image, rie, image_name = self.tagged_name("envvarcheck", arch)
        cmd = f"--name {image} -d -v {self.path_to_binary}:/local-lambda-runtime-server -p {port}:8080 --entrypoint /local-lambda-runtime-server/{rie} {image_name} {DEFAULT_1P_ENTRYPOINT} main.check_env_var_handler"
        
        with self.create_container(cmd, image):
            r = self.invoke_function(port)
        
            self.assertEqual(b'"4=4"', r.content)


    def test_two_invokes(self, arch=ARCH, port=PORT):
        image, rie, image_name = self.tagged_name("twoinvokes", arch)

        cmd = f"--name {image} -d -v {self.path_to_binary}:/local-lambda-runtime-server -p {port}:8080 --entrypoint /local-lambda-runtime-server/{rie} {image_name} {DEFAULT_1P_ENTRYPOINT} main.success_handler"

        with self.create_container(cmd, image):
            r = self.invoke_function(port)
        
            self.assertEqual(b'"My lambda ran succesfully"', r.content)

            # Make sure we can invoke the function twice
            r = self.invoke_function(port)
            
            self.assertEqual(b'"My lambda ran succesfully"', r.content)


    def test_lambda_function_arn_exists(self, arch=ARCH, port=PORT):
        image, rie, image_name = self.tagged_name("arnexists", arch)

        cmd = f"--name {image} -d -v {self.path_to_binary}:/local-lambda-runtime-server -p {port}:8080 --entrypoint /local-lambda-runtime-server/{rie} {image_name} {DEFAULT_1P_ENTRYPOINT} main.assert_lambda_arn_in_context"

        with self.create_container(cmd, image):
            r = self.invoke_function(port)
        
            self.assertEqual(b'"My lambda ran succesfully"', r.content)


    def test_lambda_function_arn_exists_with_defining_custom_name(self, arch=ARCH, port=PORT):
        image, rie, image_name = self.tagged_name("customname", arch)

        cmd = f"--name {image} --env AWS_LAMBDA_FUNCTION_NAME=MyCoolName -d -v {self.path_to_binary}:/local-lambda-runtime-server -p {port}:8080 --entrypoint /local-lambda-runtime-server/{rie} {image_name} {DEFAULT_1P_ENTRYPOINT} main.assert_lambda_arn_in_context"
        
        with self.create_container(cmd, image):
            r = self.invoke_function(port)
        
            self.assertEqual(b'"My lambda ran succesfully"', r.content)


    def test_timeout_invoke(self, arch=ARCH, port=PORT):
        image, rie, image_name = self.tagged_name("timeout", arch)

        cmd = f"--name {image} -d --env AWS_LAMBDA_FUNCTION_TIMEOUT=1 -v {self.path_to_binary}:/local-lambda-runtime-server -p {port}:8080 --entrypoint /local-lambda-runtime-server/{rie} {image_name} {DEFAULT_1P_ENTRYPOINT} main.sleep_handler"

        with self.create_container(cmd, image):
            r = self.invoke_function(port)
        
            self.assertEqual(b"Task timed out after 1.00 seconds", r.content)


    def test_exception_returned(self, arch=ARCH, port=PORT):
        image, rie, image_name = self.tagged_name("exception", arch)

        cmd = f"--name {image} -d -v {self.path_to_binary}:/local-lambda-runtime-server -p {port}:8080 --entrypoint /local-lambda-runtime-server/{rie} {image_name} {DEFAULT_1P_ENTRYPOINT} main.exception_handler"

        with self.create_container(cmd, image):
            r = self.invoke_function(port)
            
            # ignore request_id in python3.12 lambda
            result = r.json()
            self.assertEqual(result["errorMessage"],"Raising an exception")
            self.assertEqual(result["errorType"],"Exception")
            self.assertEqual(result["stackTrace"],["  File \"/var/task/main.py\", line 13, in exception_handler\n    raise Exception(\"Raising an exception\")\n"])


    def test_context_get_remaining_time_in_three_seconds(self, arch=ARCH, port=PORT):
        image, rie, image_name = self.tagged_name("remaining_time_in_three_seconds", arch)

        cmd = f"--name {image} -d --env AWS_LAMBDA_FUNCTION_TIMEOUT=3 -v {self.path_to_binary}:/local-lambda-runtime-server -p {port}:8080 --entrypoint /local-lambda-runtime-server/{rie} {image_name} {DEFAULT_1P_ENTRYPOINT} main.check_remaining_time_handler"

        with self.create_container(cmd, image):
            r = self.invoke_function(port)
        
            # Execution time is not decided, 1.0s ~ 3.0s is a good estimation
            self.assertLess(int(r.content), 3000)
            self.assertGreater(int(r.content), 1000)


    def test_context_get_remaining_time_in_ten_seconds(self, arch=ARCH, port=PORT):
        image, rie, image_name = self.tagged_name("remaining_time_in_ten_seconds", arch)

        cmd = f"--name {image} -d --env AWS_LAMBDA_FUNCTION_TIMEOUT=10 -v {self.path_to_binary}:/local-lambda-runtime-server -p {port}:8080 --entrypoint /local-lambda-runtime-server/{rie} {image_name} {DEFAULT_1P_ENTRYPOINT} main.check_remaining_time_handler"

        with self.create_container(cmd, image):
            r = self.invoke_function(port)
        
            # Execution time is not decided, 8.0s ~ 10.0s is a good estimation
            self.assertLess(int(r.content), 10000)
            self.assertGreater(int(r.content), 8000)


    def test_context_get_remaining_time_in_default_deadline(self, arch=ARCH, port=PORT):
        image, rie, image_name = self.tagged_name("remaining_time_in_default_deadline", arch)

        cmd = f"--name {image} -d -v {self.path_to_binary}:/local-lambda-runtime-server -p {port}:8080 --entrypoint /local-lambda-runtime-server/{rie} {image_name} {DEFAULT_1P_ENTRYPOINT} main.check_remaining_time_handler"

        with self.create_container(cmd, image):
            r = self.invoke_function(port)

            # Executation time is not decided, 298.0s ~ 300.0s is a good estimation
            self.assertLess(int(r.content), 300000)
            self.assertGreater(int(r.content), 298000)


    def test_invoke_with_pre_runtime_api_runtime(self, arch=ARCH, port=PORT):
        image, rie, image_name = self.tagged_name("pre-runtime-api", arch)

        cmd = f"--name {image} -d -v {self.path_to_binary}:/local-lambda-runtime-server -p {port}:8080 --entrypoint /local-lambda-runtime-server/{rie} {image_name} {DEFAULT_1P_ENTRYPOINT} main.success_handler"

        with self.create_container(cmd, image):
            r = self.invoke_function(port)
        
            self.assertEqual(b'"My lambda ran succesfully"', r.content)


    def test_function_name_is_overriden(self, arch=ARCH, port=PORT):
        image, rie, image_name = self.tagged_name("assert-overwritten", arch)

        cmd = f"--name {image} -d --env AWS_LAMBDA_FUNCTION_NAME=MyCoolName -v {self.path_to_binary}:/local-lambda-runtime-server -p {port}:8080 --entrypoint /local-lambda-runtime-server/{rie} {image_name} {DEFAULT_1P_ENTRYPOINT} main.assert_env_var_is_overwritten"

        with self.create_container(cmd, image):
            r = self.invoke_function(port)
        
            self.assertEqual(b'"My lambda ran succesfully"', r.content)


    def test_port_override(self, arch=ARCH, port=PORT):
        image, rie, image_name = self.tagged_name("port_override", arch)

        # Use port 8081 inside the container instead of 8080
        cmd = f"--name {image} -d -v {self.path_to_binary}:/local-lambda-runtime-server -p {port}:8081 --entrypoint /local-lambda-runtime-server/{rie} {image_name} {DEFAULT_1P_ENTRYPOINT} main.success_handler --runtime-interface-emulator-address 0.0.0.0:8081"

        with self.create_container(cmd, image):
            r = self.invoke_function(port)
        
            self.assertEqual(b'"My lambda ran succesfully"', r.content)



if __name__ == "__main__":
    main()
