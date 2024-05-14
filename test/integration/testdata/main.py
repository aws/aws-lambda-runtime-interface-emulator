# Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
# SPDX-License-Identifier: Apache-2.0

import time
import os

def sleep_handler(event, context):
    time.sleep(5)
    return "I was sleeping"


def exception_handler(event, context):
    raise Exception("Raising an exception")


def success_handler(event, context):
    print("Printing data to console")

    return "My lambda ran succesfully"


def check_env_var_handler(event, context):
    return os.environ.get("MyEnv")


def assert_env_var_is_overwritten(event, context):
    print(os.environ.get("AWS_LAMBDA_FUNCTION_NAME"))
    if os.environ.get("AWS_LAMBDA_FUNCTION_NAME") == "test_function":
        raise("Function name was not overwritten")
    else:
        return "My lambda ran succesfully"

def assert_lambda_arn_in_context(event, context):
    if context.invoked_function_arn == f"arn:aws:lambda:us-east-1:012345678912:function:{os.environ.get('AWS_LAMBDA_FUNCTION_NAME', 'test_function')}":
        return "My lambda ran succesfully"
    else:
        raise("Function Arn was not there")


def check_remaining_time_handler(event, context):
    # Wait 1s to see if the remaining time changes
    time.sleep(1)
    return context.get_remaining_time_in_millis()


def custom_client_context_handler(event, context):
    return context.client_context.custom
