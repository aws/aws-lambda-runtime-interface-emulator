# Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
# SPDX-License-Identifier: Apache-2.0

from subprocess import Popen, PIPE
from unittest import TestCase, main
from pathlib import Path
import time

import requests

SLEEP_TIME = 2
DEFAULT_1P_ENTRYPOINT = "/lambda-entrypoint.sh"

class TestEndToEnd(TestCase):

    @classmethod
    def setUpClass(cls):
        testdata_path = Path(__file__).resolve().parents[1].joinpath("testdata")
        dockerfile_path = testdata_path.joinpath("Dockerfile-allinone")
        cls.image_name = "aws-lambda-local:testing"
        cls.path_to_binary = Path().resolve().joinpath("bin")


        # build image
        build_cmd = ["docker", "build", "-t", cls.image_name, "-f", str(dockerfile_path), str(testdata_path)]
        Popen(build_cmd).communicate()

    @classmethod
    def tearDownClass(cls):
        cmds_to_delete_images = ["docker rm -f envvarcheck", "docker rm -f testing", "docker rm -f timeout", "docker rm -f exception", "docker rm -f remainingtime"]

        for cmd in cmds_to_delete_images:
            Popen(cmd.split(' ')).communicate()

        Popen(f"docker rmi {cls.image_name}".split(' ')).communicate()


    def test_env_var_with_eqaul_sign(self):
        cmd = f"docker run --name envvarcheck -d -v {self.path_to_binary}:/local-lambda-runtime-server -p 9003:8080 --entrypoint /local-lambda-runtime-server/aws-lambda-rie {self.image_name} {DEFAULT_1P_ENTRYPOINT} main.check_env_var_handler"

        Popen(cmd.split(' ')).communicate()

        # sleep 1s to give enough time for the endpoint to be up to curl
        time.sleep(SLEEP_TIME)

        r = requests.post("http://localhost:9003/2015-03-31/functions/function/invocations", json={})
        self.assertEqual(b'"4=4"', r.content)

    def test_two_invokes(self):
        cmd = f"docker run --name testing -d -v {self.path_to_binary}:/local-lambda-runtime-server -p 9000:8080 --entrypoint /local-lambda-runtime-server/aws-lambda-rie {self.image_name} {DEFAULT_1P_ENTRYPOINT} main.success_handler"

        Popen(cmd.split(' ')).communicate()

        # sleep 1s to give enough time for the endpoint to be up to curl
        time.sleep(SLEEP_TIME)

        r = requests.post("http://localhost:9000/2015-03-31/functions/function/invocations", json={})
        self.assertEqual(b'"My lambda ran succesfully"', r.content)

        # Make sure we can invoke the function twice
        r = requests.post("http://localhost:9000/2015-03-31/functions/function/invocations", json={})
        self.assertEqual(b'"My lambda ran succesfully"', r.content) 

    def test_lambda_function_arn_exists(self):
        cmd = f"docker run --name testing -d -v {self.path_to_binary}:/local-lambda-runtime-server -p 9000:8080 --entrypoint /local-lambda-runtime-server/aws-lambda-rie {self.image_name} {DEFAULT_1P_ENTRYPOINT} main.assert_lambda_arn_in_context"

        Popen(cmd.split(' ')).communicate()

        # sleep 1s to give enough time for the endpoint to be up to curl
        time.sleep(SLEEP_TIME)

        r = requests.post("http://localhost:9000/2015-03-31/functions/function/invocations", json={})
        self.assertEqual(b'"My lambda ran succesfully"', r.content)

    def test_lambda_function_arn_exists_with_defining_custom_name(self):
        cmd = f"docker run --name testing --env AWS_LAMBDA_FUNCTION_NAME=MyCoolName -d -v {self.path_to_binary}:/local-lambda-runtime-server -p 9000:8080 --entrypoint /local-lambda-runtime-server/aws-lambda-rie {self.image_name} {DEFAULT_1P_ENTRYPOINT} main.assert_lambda_arn_in_context"

        Popen(cmd.split(' ')).communicate()

        # sleep 1s to give enough time for the endpoint to be up to curl
        time.sleep(SLEEP_TIME)

        r = requests.post("http://localhost:9000/2015-03-31/functions/function/invocations", json={})
        self.assertEqual(b'"My lambda ran succesfully"', r.content)


    def test_timeout_invoke(self):
        cmd = f"docker run --name timeout -d --env AWS_LAMBDA_FUNCTION_TIMEOUT=1 -v {self.path_to_binary}:/local-lambda-runtime-server -p 9001:8080 --entrypoint /local-lambda-runtime-server/aws-lambda-rie {self.image_name} {DEFAULT_1P_ENTRYPOINT} main.sleep_handler"

        Popen(cmd.split(' ')).communicate()

        # sleep 1s to give enough time for the endpoint to be up to curl
        time.sleep(SLEEP_TIME)

        r = requests.post("http://localhost:9001/2015-03-31/functions/function/invocations", json={})
        self.assertEqual(b"Task timed out after 1.00 seconds", r.content)

    def test_exception_returned(self):
        cmd = f"docker run --name exception -d -v {self.path_to_binary}:/local-lambda-runtime-server -p 9002:8080 --entrypoint /local-lambda-runtime-server/aws-lambda-rie {self.image_name} {DEFAULT_1P_ENTRYPOINT} main.exception_handler"

        Popen(cmd.split(' ')).communicate()

        # sleep 1s to give enough time for the endpoint to be up to curl
        time.sleep(SLEEP_TIME)

        r = requests.post("http://localhost:9002/2015-03-31/functions/function/invocations", json={})
        self.assertEqual(b'{"errorMessage": "Raising an exception", "errorType": "Exception", "stackTrace": ["  File \\"/var/task/main.py\\", line 13, in exception_handler\\n    raise Exception(\\"Raising an exception\\")\\n"]}', r.content)

    def test_context_get_remaining_time_in_three_seconds(self):
        cmd = f"docker run --name remainingtime -d --env AWS_LAMBDA_FUNCTION_TIMEOUT=3 -v {self.path_to_binary}:/local-lambda-runtime-server -p 9004:8080 --entrypoint /local-lambda-runtime-server/aws-lambda-rie {self.image_name} {DEFAULT_1P_ENTRYPOINT} main.check_remaining_time_handler"

        Popen(cmd.split(' ')).communicate()

        r = requests.post("http://localhost:9004/2015-03-31/functions/function/invocations", json={})

        # sleep 1s to give enough time for the endpoint to be up to curl
        time.sleep(SLEEP_TIME)
        self.assertLess(int(r.content), 3000)

class TestPython36Runtime(TestCase):

    @classmethod
    def setUpClass(cls):
        testdata_path = Path(__file__).resolve().parents[1].joinpath("testdata")
        dockerfile_path = testdata_path.joinpath("Dockerfile-python36")
        cls.image_name = "aws-lambda-local:testing-py36"
        cls.path_to_binary = Path().resolve().joinpath("bin")


        # build image
        build_cmd = ["docker", "build", "-t", cls.image_name, "-f", str(dockerfile_path), str(testdata_path)]
        Popen(build_cmd).communicate()

    @classmethod
    def tearDownClass(cls):
        cmds_to_delete_images = ["docker rm -f testing", "docker rm -f assert-overwritten"]

        for cmd in cmds_to_delete_images:
            Popen(cmd.split(' ')).communicate()

        Popen(f"docker rmi {cls.image_name}".split(' ')).communicate()

    def test_invoke_with_pre_runtime_api_runtime(self):
        cmd = f"docker run --name testing -d -v {self.path_to_binary}:/local-lambda-runtime-server -p 9000:8080 --entrypoint /local-lambda-runtime-server/aws-lambda-rie {self.image_name} {DEFAULT_1P_ENTRYPOINT} main.success_handler"

        Popen(cmd.split(' ')).communicate()

        # sleep 1s to give enough time for the endpoint to be up to curl
        time.sleep(SLEEP_TIME)

        r = requests.post("http://localhost:9000/2015-03-31/functions/function/invocations", json={})
        self.assertEqual(b'"My lambda ran succesfully"', r.content)

    def test_function_name_is_overriden(self):
        cmd = f"docker run --name assert-overwritten -d --env AWS_LAMBDA_FUNCTION_NAME=MyCoolName -v {self.path_to_binary}:/local-lambda-runtime-server -p 9009:8080 --entrypoint /local-lambda-runtime-server/aws-lambda-rie {self.image_name} {DEFAULT_1P_ENTRYPOINT} main.assert_env_var_is_overwritten"

        Popen(cmd.split(' ')).communicate()

        # sleep 1s to give enough time for the endpoint to be up to curl
        time.sleep(SLEEP_TIME)

        r = requests.post("http://localhost:9009/2015-03-31/functions/function/invocations", json={})
        self.assertEqual(b'"My lambda ran succesfully"', r.content)


if __name__ == "__main__":
    main()