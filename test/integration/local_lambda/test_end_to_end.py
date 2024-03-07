# Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
# SPDX-License-Identifier: Apache-2.0

from subprocess import Popen, PIPE
from unittest import TestCase, main
from pathlib import Path
import time

import requests
from parameterized import parameterized

SLEEP_TIME = 2
DEFAULT_1P_ENTRYPOINT = "/lambda-entrypoint.sh"
ARCHS = ["x86_64", "arm64", ""]


class TestEndToEnd(TestCase):
    @classmethod
    def setUpClass(cls):
        testdata_path = Path(__file__).resolve().parents[1].joinpath("testdata")
        dockerfile_path = testdata_path.joinpath("Dockerfile-allinone")
        cls.image_name = "aws-lambda-local:testing"
        cls.path_to_binary = Path().resolve().joinpath("bin")

        # build image
        for arch in ARCHS:
            image_name = cls.image_name if arch == "" else f"{cls.image_name}-{arch}"
            architecture = arch if arch == "arm64" else "amd64"
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
            ]
            Popen(build_cmd).communicate()

    @classmethod
    def tearDownClass(cls):
        images_to_delete = [
            "envvarcheck",
            "twoinvokes",
            "arnexists",
            "customname",
            "timeout",
            "exception",
            "remaining_time_in_three_seconds",
            "remaining_time_in_ten_seconds",
            "remaining_time_in_default_deadline",
            "pre-runtime-api",
            "assert-overwritten",
            "port_override"
        ]

        for image in images_to_delete:
            for arch in ARCHS:
                arch_tag = "" if arch == "" else f"-{arch}"
                cmd = f"docker rm -f {image}{arch_tag}"
                Popen(cmd.split(" ")).communicate()
            
        for arch in ARCHS:
            arch_tag = "" if arch == "" else f"-{arch}"
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
        
    def create_container_and_invoke_function(self, cmd, port):
        self.run_command(cmd)
        
        # sleep 1s to give enough time for the endpoint to be up to curl
        self.sleep_1s()
        
        return self.invoke_function(port)

    @parameterized.expand([("x86_64", "8000"), ("arm64", "9000"), ("", "9050")])
    def test_env_var_with_equal_sign(self, arch, port):
        image, rie, image_name = self.tagged_name("envvarcheck", arch)

        cmd = f"docker run --name {image} -d -v {self.path_to_binary}:/local-lambda-runtime-server -p {port}:8080 --entrypoint /local-lambda-runtime-server/{rie} {image_name} {DEFAULT_1P_ENTRYPOINT} main.check_env_var_handler"
        
        r = self.create_container_and_invoke_function(cmd, port)
        
        self.assertEqual(b'"4=4"', r.content)

    @parameterized.expand([("x86_64", "8001"), ("arm64", "9001"), ("", "9051")])
    def test_two_invokes(self, arch, port):
        image, rie, image_name = self.tagged_name("twoinvokes", arch)

        cmd = f"docker run --name {image} -d -v {self.path_to_binary}:/local-lambda-runtime-server -p {port}:8080 --entrypoint /local-lambda-runtime-server/{rie} {image_name} {DEFAULT_1P_ENTRYPOINT} main.success_handler"

        r = self.create_container_and_invoke_function(cmd, port)
        
        self.assertEqual(b'"My lambda ran succesfully"', r.content)

        # Make sure we can invoke the function twice
        r = self.invoke_function(port)
        
        self.assertEqual(b'"My lambda ran succesfully"', r.content)

    @parameterized.expand([("x86_64", "8002"), ("arm64", "9002"), ("", "9052")])
    def test_lambda_function_arn_exists(self, arch, port):
        image, rie, image_name = self.tagged_name("arnexists", arch)

        cmd = f"docker run --name {image} -d -v {self.path_to_binary}:/local-lambda-runtime-server -p {port}:8080 --entrypoint /local-lambda-runtime-server/{rie} {image_name} {DEFAULT_1P_ENTRYPOINT} main.assert_lambda_arn_in_context"

        r = self.create_container_and_invoke_function(cmd, port)
        
        self.assertEqual(b'"My lambda ran succesfully"', r.content)

    @parameterized.expand([("x86_64", "8003"), ("arm64", "9003"), ("", "9053")])
    def test_lambda_function_arn_exists_with_defining_custom_name(self, arch, port):
        image, rie, image_name = self.tagged_name("customname", arch)

        cmd = f"docker run --name {image} --env AWS_LAMBDA_FUNCTION_NAME=MyCoolName -d -v {self.path_to_binary}:/local-lambda-runtime-server -p {port}:8080 --entrypoint /local-lambda-runtime-server/{rie} {image_name} {DEFAULT_1P_ENTRYPOINT} main.assert_lambda_arn_in_context"
        
        r = self.create_container_and_invoke_function(cmd, port)
        
        self.assertEqual(b'"My lambda ran succesfully"', r.content)

    @parameterized.expand([("x86_64", "8004"), ("arm64", "9004"), ("", "9054")])
    def test_timeout_invoke(self, arch, port):
        image, rie, image_name = self.tagged_name("timeout", arch)

        cmd = f"docker run --name {image} -d --env AWS_LAMBDA_FUNCTION_TIMEOUT=1 -v {self.path_to_binary}:/local-lambda-runtime-server -p {port}:8080 --entrypoint /local-lambda-runtime-server/{rie} {image_name} {DEFAULT_1P_ENTRYPOINT} main.sleep_handler"

        r = self.create_container_and_invoke_function(cmd, port)
        
        self.assertEqual(b"Task timed out after 1.00 seconds", r.content)

    @parameterized.expand([("x86_64", "8005"), ("arm64", "9005"), ("", "9055")])
    def test_exception_returned(self, arch, port):
        image, rie, image_name = self.tagged_name("exception", arch)

        cmd = f"docker run --name {image} -d -v {self.path_to_binary}:/local-lambda-runtime-server -p {port}:8080 --entrypoint /local-lambda-runtime-server/{rie} {image_name} {DEFAULT_1P_ENTRYPOINT} main.exception_handler"

        r = self.create_container_and_invoke_function(cmd, port)
        
        self.assertEqual(
            b'{"errorMessage": "Raising an exception", "errorType": "Exception", "stackTrace": ["  File \\"/var/task/main.py\\", line 13, in exception_handler\\n    raise Exception(\\"Raising an exception\\")\\n"]}',
            r.content,
        )

    @parameterized.expand([("x86_64", "8006"), ("arm64", "9006"), ("", "9056")])
    def test_context_get_remaining_time_in_three_seconds(self, arch, port):
        image, rie, image_name = self.tagged_name("remaining_time_in_three_seconds", arch)

        cmd = f"docker run --name {image} -d --env AWS_LAMBDA_FUNCTION_TIMEOUT=3 -v {self.path_to_binary}:/local-lambda-runtime-server -p {port}:8080 --entrypoint /local-lambda-runtime-server/{rie} {image_name} {DEFAULT_1P_ENTRYPOINT} main.check_remaining_time_handler"

        r = self.create_container_and_invoke_function(cmd, port)
        
        # Execution time is not decided, 1.0s ~ 3.0s is a good estimation
        self.assertLess(int(r.content), 3000)
        self.assertGreater(int(r.content), 1000)

    @parameterized.expand([("x86_64", "8007"), ("arm64", "9007"), ("", "9057")])
    def test_context_get_remaining_time_in_ten_seconds(self, arch, port):
        image, rie, image_name = self.tagged_name("remaining_time_in_ten_seconds", arch)

        cmd = f"docker run --name {image} -d --env AWS_LAMBDA_FUNCTION_TIMEOUT=10 -v {self.path_to_binary}:/local-lambda-runtime-server -p {port}:8080 --entrypoint /local-lambda-runtime-server/{rie} {image_name} {DEFAULT_1P_ENTRYPOINT} main.check_remaining_time_handler"

        r = self.create_container_and_invoke_function(cmd, port)
        
        # Execution time is not decided, 8.0s ~ 10.0s is a good estimation
        self.assertLess(int(r.content), 10000)
        self.assertGreater(int(r.content), 8000)

    @parameterized.expand([("x86_64", "8008"), ("arm64", "9008"), ("", "9058")])
    def test_context_get_remaining_time_in_default_deadline(self, arch, port):
        image, rie, image_name = self.tagged_name("remaining_time_in_default_deadline", arch)

        cmd = f"docker run --name {image} -d -v {self.path_to_binary}:/local-lambda-runtime-server -p {port}:8080 --entrypoint /local-lambda-runtime-server/{rie} {image_name} {DEFAULT_1P_ENTRYPOINT} main.check_remaining_time_handler"

        r = self.create_container_and_invoke_function(cmd, port)

        # Executation time is not decided, 298.0s ~ 300.0s is a good estimation
        self.assertLess(int(r.content), 300000)
        self.assertGreater(int(r.content), 298000)

    @parameterized.expand([("x86_64", "8009"), ("arm64", "9009"), ("", "9059")])
    def test_invoke_with_pre_runtime_api_runtime(self, arch, port):
        image, rie, image_name = self.tagged_name("pre-runtime-api", arch)

        cmd = f"docker run --name {image} -d -v {self.path_to_binary}:/local-lambda-runtime-server -p {port}:8080 --entrypoint /local-lambda-runtime-server/{rie} {image_name} {DEFAULT_1P_ENTRYPOINT} main.success_handler"

        r = self.create_container_and_invoke_function(cmd, port)
        
        self.assertEqual(b'"My lambda ran succesfully"', r.content)

    @parameterized.expand([("x86_64", "8010"), ("arm64", "9010"), ("", "9060")])
    def test_function_name_is_overriden(self, arch, port):
        image, rie, image_name = self.tagged_name("assert-overwritten", arch)

        cmd = f"docker run --name {image} -d --env AWS_LAMBDA_FUNCTION_NAME=MyCoolName -v {self.path_to_binary}:/local-lambda-runtime-server -p {port}:8080 --entrypoint /local-lambda-runtime-server/{rie} {image_name} {DEFAULT_1P_ENTRYPOINT} main.assert_env_var_is_overwritten"

        r = self.create_container_and_invoke_function(cmd, port)
        
        self.assertEqual(b'"My lambda ran succesfully"', r.content)

    @parameterized.expand([("x86_64", "8011"), ("arm64", "9011"), ("", "9061")])
    def test_port_override(self, arch, port):
        image, rie, image_name = self.tagged_name("port_override", arch)

        # Use port 8081 inside the container instead of 8080
        cmd = f"docker run --name {image} -d -v {self.path_to_binary}:/local-lambda-runtime-server -p {port}:8081 --entrypoint /local-lambda-runtime-server/{rie} {image_name} {DEFAULT_1P_ENTRYPOINT} main.success_handler --runtime-interface-emulator-address 0.0.0.0:8081"

        r = self.create_container_and_invoke_function(cmd, port)
        
        self.assertEqual(b'"My lambda ran succesfully"', r.content)


if __name__ == "__main__":
    main()
