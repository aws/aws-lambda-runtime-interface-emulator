// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

/*

Package rendering provides stateful event rendering service.

State of the rendering service should be set from the main event dispatch thread
prior to releasing threads that are registered for the event.

Example of INVOKE event:

[thread] // suspended in READY state

[main] // set renderer for INVOKE event
[main] renderingService.SetRenderer(rendering.NewInvokeRenderer())
[main] // release threads registered for INVOKE event

[thread] // receives INVOKE event

*/
package rendering
