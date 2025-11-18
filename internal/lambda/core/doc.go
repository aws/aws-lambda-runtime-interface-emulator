// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

/*
Package core provides state objects and synchronization primitives for
managing data flow in the system.

# States

Runtime and Agent implement state object design pattern.

Runtime state interface:

	type RuntimeState interface {
		InitError() error
		Ready() error
		InvocationResponse() error
		InvocationErrorResponse() error
	}

# Gates

Gates provide synchornization primitives for managing data flow in the system.

Gate is a synchronization aid that allows one or more threads to wait until a
set of operations being performed in other threads completes.

To better understand gates, consider two examples below:

Example 1: main thread is awaiting registered threads to walk through the gate,

	and after the last registered thread walked through the gate, gate
	condition will be satisfied and main thread will proceed:

[main] // register threads with the gate and start threads ...
[main] g.AwaitGateCondition()
[main] // blocked until gate condition is satisfied

[thread] g.WalkThrough()
[thread] // not blocked

Example 2: main thread is awaiting registered threads to arrive at the gate,

	and after the last registered thread arrives at the gate, gate
	condition will be satisfied and main thread, along with registered
	threads will proceed:

[main] // register threads with the gate and start threads ...
[main] g.AwaitGateCondition()
[main] // blocked until gate condition is satisfied

# Flow

Flow wraps a set of specific gates required to implement specific data flow in the system.

Example flows would be INIT, INVOKE and RESET.

# Registrations

Registration service manages registrations, it maintains the mapping between registered
parties are events they are registered. Parties not registered in the system will not
be issued events.
*/
package core
