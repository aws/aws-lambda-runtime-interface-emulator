import time

invoke_counter = 0


def handler(event, context):
    global invoke_counter
    invoke_counter += 1
    wait_time = event.get("wait", "0.1")
    print(f"Starting to sleep...")
    time.sleep(float(wait_time))
    print(f"Ending sleep...")
    counter = event["counter"]

    if event.get("fail"):
        raise Exception("oh no")

    return {
        "counter": counter + 1,
        "invoke_counter": invoke_counter
    }
