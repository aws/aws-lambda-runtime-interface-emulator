# RIE Telemetry Package

The RIE (Runtime Interface Emulator) telemetry package provides Telemetry API.

## Architecture Overview

```
┌─────────────────┐    ┌─────────────────┐    ┌──────────────────┐
│   EventsAPI     │    │   LogsEgress    │    │ SubscriptionAPI  │
│                 │    │                 │    │                  │
│ • Platform      │    │ • Runtime logs  │    │ • Subscription   │
│   events        │    │ • Extension     │    │   management     │
│ • Lifecycle     │    │   logs          │    │ • Schema         │
│   events        │    │ • Log capture   │    │   validation     │
└─────────┬───────┘    └─────────┬───────┘    └──────────┬───────┘
          │                      │                       │
          └──────────────┬───────────────────────────────┘
                         │
                    ┌────▼────┐
                    │  Relay  │
                    │         │
                    │ Event   │
                    │ Broker  │
                    └────┬────┘
                         │
          ┌──────────────┼──────────────┐
          │              │              │
    ┌─────▼─────┐  ┌─────▼─────┐  ┌─────▼─────┐
    │Subscriber │  │Subscriber │  │Subscriber │
    │    A      │  │    B      │  │    C      │
    └─────┬─────┘  └─────┬─────┘  └─────┬─────┘
          │              │              │
    ┌─────▼─────┐  ┌─────▼─────┐  ┌─────▼─────┐
    │TCP Client │  │HTTP Client│  │TCP Client │
    └───────────┘  └───────────┘  └───────────┘
```

## Core Components

### 1. EventsAPI (`events_api.go`)
**Responsibility**: Platform event generation and distribution

The EventsAPI serves as the primary interface for generating and broadcasting AWS Lambda platform events. It implements the `EventsAPI` interface and handles various lifecycle events including initialization, invocation, and error reporting.

### 2. LogsEgress (`logs_egress.go`)
**Responsibility**: Log capture and forwarding

The LogsEgress component implements the `StdLogsEgressAPI` interface to capture stdout/stderr from both runtime and extensions, forwarding them to telemetry subscribers while maintaining original console output.

### 3. Relay (`relay.go`)
**Responsibility**: Event broadcasting and subscriber management

The Relay acts as a central event broker, managing subscribers and broadcasting events to all registered telemetry consumers.

### 4. SubscriptionAPI (`subscription_api.go`)
**Responsibility**: Subscription management and validation

The SubscriptionAPI handles telemetry subscription requests, validates them against JSON schemas, and manages the subscription lifecycle.

## Internal Components

### 1. Subscriber (`internal/subscriber.go`)
**Responsibility**: Event batching and delivery

Each subscriber represents a telemetry consumer and manages efficient event delivery through batching and asynchronous processing.

### 2. Client (`internal/client.go`)
**Responsibility**: Protocol-specific event delivery

The client abstraction provides protocol-specific implementations for delivering events to telemetry consumers.

### 3. Batch (`internal/batch.go`)
**Responsibility**: Event collection and timing

The batch component manages collections of events with size and time-based flushing logic.

### 4. Types (`internal/types.go`)
**Responsibility**: Type definitions and constants

Centralized type definitions for protocols, event categories, and configuration structures.

## Event Flow

### 1. Subscription Flow
```
Extension/Agent → SubscriptionAPI → Schema Validation → Subscriber Creation → Relay Registration
```

### 2. Event Flow
```
Event Source → EventsAPI → Relay → Subscribers → Batching → Client
```

### 3. Log Flow
```
Runtime/Extension → LogsEgress → Console Output + Relay → Subscribers → Batching → Client
```
