// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

/*
RAPID emits or proxies the following sources of logging:

 1. Internal logs: RAPID's own application logs into stderr for operational use, visible only internally
 2. Function stream-based logs: Runtime's stdout and stderr, read as newline separated lines
 3. Function message-based logs: Stock runtimes communicate using a custom TLV protocol over a Unix pipe
 4. Extension stream-based logs: Extension's stdout and stderr, read as newline separated lines
 5. Platform logs: Logs that RAPID generates, but is visible either in customer's logs or via Logs API
    (e.g. EXTENSION, RUNTIME, RUNTIMEDONE, IMAGE)
*/
package logging
