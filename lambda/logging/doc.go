// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

/*

RAPID emits or proxies the following sources of logging:

1. Internal logs: RAPID's own application logs into stderr for operational use, visible only internally
2. Function stream-based logs: Runtime's stdout and stderr, read as newline separated lines
3. Function message-based logs: Stock runtimes communicate using a custom TLV protocol over a Unix pipe
4. Extension stream-based logs: Extension's stdout and stderr, read as newline separated lines
5. Platform logs: Logs that RAPID generates, but is visible in customer's logs.


It has the following log sinks, which may further be egressed to other sinks (e.g. CloudWatch) by external telemetry agents:

1. Internal Log File (stderr): stderr is redirected to a file specified by Sandbox Factory via env-vars, and accessible via StreamQuery
2. Stdout: stream-based function logs are output to RAPID's stdout process, and read by a telemetry agent
3. Telemetry API MSG-verb events: function messages-based logs are written using GirP protocol into the console socket specified by Sandbox Factory env-vars
4. Telemetry API LOGX-verb events: extension stream-based logs are written using GirP protocol into the console socket specified by Sandbox Factory env-vars
5. Telemetry API LOGP-verb events: platform logs are written using GirP protocol into the console socket specified by Sandbox Factory env-vars
6. Tail logs: a truncated version of function stream-based and message-based logs are written along with the invocation response to the frontend when 'debug logging' is enabled

*/
package logging
