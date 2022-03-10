
## Debugging

Debugging with delve can be enabled by setting the entrypoint to the delve binary instead:

```bash
/dlv --listen=:"${LOCALSTACK_RIE_DEBUG_PORT}" --headless=true --api-version=2 --accept-multiclient exec /usr/local/bin/aws-lambda-rie "${RUNTIME_ENTRYPOINT}"
```