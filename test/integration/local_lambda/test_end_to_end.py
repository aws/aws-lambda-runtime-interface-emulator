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
            "pre-runtime-api",
            "assert-overwritten",
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

    @parameterized.expand([("x86_64", "8000"), ("arm64", "9001"), ("", "9003")])
    def test_env_var_with_equal_sign(self, arch, port):
        image, rie, image_name = self.tagged_name("envvarcheck", arch)

        cmd = f"docker run --name {image} -d -v {self.path_to_binary}:/local-lambda-runtime-server -p {port}:8080 --entrypoint /local-lambda-runtime-server/{rie} {image_name} {DEFAULT_1P_ENTRYPOINT} main.check_env_var_handler"
        Popen(cmd.split(" ")).communicate()

        # sleep 1s to give enough time for the endpoint to be up to curl
        time.sleep(SLEEP_TIME)

        r = requests.post(
            f"http://localhost:{port}/2015-03-31/functions/function/invocations", json={}
        )
        self.assertEqual(b'"4=4"', r.content)

    @parameterized.expand([("x86_64", "8001"), ("arm64", "9002"), ("", "9005")])
    def test_two_invokes(self, arch, port):
        image, rie, image_name = self.tagged_name("twoinvokes", arch)

        cmd = f"docker run --name {image} -d -v {self.path_to_binary}:/local-lambda-runtime-server -p {port}:8080 --entrypoint /local-lambda-runtime-server/{rie} {image_name} {DEFAULT_1P_ENTRYPOINT} main.success_handler"

        Popen(cmd.split(" ")).communicate()

        # sleep 1s to give enough time for the endpoint to be up to curl
        time.sleep(SLEEP_TIME)

        r = requests.post(
            f"http://localhost:{port}/2015-03-31/functions/function/invocations", json={}
        )
        self.assertEqual(b'"My lambda ran succesfully"', r.content)

        # Make sure we can invoke the function twice
        r = requests.post(
            f"http://localhost:{port}/2015-03-31/functions/function/invocations", json={}
        )
        self.assertEqual(b'"My lambda ran succesfully"', r.content)

    @parameterized.expand([("x86_64", "8002"), ("arm64", "9004"), ("", "9007")])
    def test_lambda_function_arn_exists(self, arch, port):
        image, rie, image_name = self.tagged_name("arnexists", arch)
        
        cmd = f"docker run --name {image} -d -v {self.path_to_binary}:/local-lambda-runtime-server -p {port}:8080 --entrypoint /local-lambda-runtime-server/{rie} {image_name} {DEFAULT_1P_ENTRYPOINT} main.assert_lambda_arn_in_context"

        Popen(cmd.split(" ")).communicate()

        # sleep 1s to give enough time for the endpoint to be up to curl
        time.sleep(SLEEP_TIME)

        r = requests.post(
            f"http://localhost:{port}/2015-03-31/functions/function/invocations", json={}
        )
        self.assertEqual(b'"My lambda ran succesfully"', r.content)

    @parameterized.expand([("x86_64", "8003"), ("arm64", "9006"), ("", "9009")])
    def test_lambda_function_arn_exists_with_defining_custom_name(self, arch, port):
        image, rie, image_name = self.tagged_name("customname", arch)
        
        cmd = f"docker run --name {image} --env AWS_LAMBDA_FUNCTION_NAME=MyCoolName -d -v {self.path_to_binary}:/local-lambda-runtime-server -p {port}:8080 --entrypoint /local-lambda-runtime-server/{rie} {image_name} {DEFAULT_1P_ENTRYPOINT} main.assert_lambda_arn_in_context"
        Popen(cmd.split(" ")).communicate()

        # sleep 1s to give enough time for the endpoint to be up to curl
        time.sleep(SLEEP_TIME)

        r = requests.post(
            f"http://localhost:{port}/2015-03-31/functions/function/invocations", json={}
        )
        self.assertEqual(b'"My lambda ran succesfully"', r.content)

    @parameterized.expand([("x86_64", "8004"), ("arm64", "9008"), ("", "9011")])
    def test_timeout_invoke(self, arch, port):
        image, rie, image_name = self.tagged_name("timeout", arch)
        
        cmd = f"docker run --name {image} -d --env AWS_LAMBDA_FUNCTION_TIMEOUT=1 -v {self.path_to_binary}:/local-lambda-runtime-server -p {port}:8080 --entrypoint /local-lambda-runtime-server/{rie} {image_name} {DEFAULT_1P_ENTRYPOINT} main.sleep_handler"

        Popen(cmd.split(" ")).communicate()

        # sleep 1s to give enough time for the endpoint to be up to curl
        time.sleep(SLEEP_TIME)

        r = requests.post(
            f"http://localhost:{port}/2015-03-31/functions/function/invocations", json={}
        )
        self.assertEqual(b"Task timed out after 1.00 seconds", r.content)

    @parameterized.expand([("x86_64", "8005"), ("arm64", "9010"), ("", "9013")])
    def test_exception_returned(self, arch, port):
        image, rie, image_name = self.tagged_name("exception", arch)
        
        cmd = f"docker run --name {image} -d -v {self.path_to_binary}:/local-lambda-runtime-server -p {port}:8080 --entrypoint /local-lambda-runtime-server/{rie} {image_name} {DEFAULT_1P_ENTRYPOINT} main.exception_handler"

        Popen(cmd.split(" ")).communicate()

        # sleep 1s to give enough time for the endpoint to be up to curl
        time.sleep(SLEEP_TIME)

        r = requests.post(
            f"http://localhost:{port}/2015-03-31/functions/function/invocations", json={}
        )
        self.assertEqual(
            b'{"errorMessage": "Raising an exception", "errorType": "Exception", "stackTrace": ["  File \\"/var/task/main.py\\", line 13, in exception_handler\\n    raise Exception(\\"Raising an exception\\")\\n"]}',
            r.content,
        )

    @parameterized.expand([("x86_64", "8006"), ("arm64", "9012"), ("", "9015")])
    def test_invoke_with_pre_runtime_api_runtime(self, arch, port):
        image, rie, image_name = self.tagged_name("pre-runtime-api", arch)
        
        cmd = f"docker run --name {image} -d -v {self.path_to_binary}:/local-lambda-runtime-server -p {port}:8080 --entrypoint /local-lambda-runtime-server/{rie} {image_name} {DEFAULT_1P_ENTRYPOINT} main.success_handler"

        Popen(cmd.split(" ")).communicate()

        # sleep 1s to give enough time for the endpoint to be up to curl
        time.sleep(SLEEP_TIME)

        r = requests.post(
            f"http://localhost:{port}/2015-03-31/functions/function/invocations", json={}
        )
        self.assertEqual(b'"My lambda ran succesfully"', r.content)

    @parameterized.expand([("x86_64", "8007"), ("arm64", "9014"), ("", "9016")])
    def test_function_name_is_overriden(self, arch, port):
        image, rie, image_name = self.tagged_name("assert-overwritten", arch)
        
        cmd = f"docker run --name {image} -d --env AWS_LAMBDA_FUNCTION_NAME=MyCoolName -v {self.path_to_binary}:/local-lambda-runtime-server -p {port}:8080 --entrypoint /local-lambda-runtime-server/{rie} {image_name} {DEFAULT_1P_ENTRYPOINT} main.assert_env_var_is_overwritten"

        Popen(cmd.split(" ")).communicate()

        # sleep 1s to give enough time for the endpoint to be up to curl
        time.sleep(SLEEP_TIME)

        r = requests.post(
            f"http://localhost:{port}/2015-03-31/functions/function/invocations", json={}
        )
        self.assertEqual(b'"My lambda ran succesfully"', r.content)


if __name__ == "__main__":
    main()