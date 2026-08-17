---
id: ADR-0004
title: Hexagonal architecture per the java-hexagonal skill
date: 2026-06-04
status: accepted
---

## Context and Problem Statement

Every application in this repository needs a consistent internal layering
and a consistent test discipline; deciding both per application, ad hoc,
would let each one drift toward its own conventions and its own
framework-coupling mistakes.

## Considered Options

- Let each application choose its own architecture and test strategy
- Adopt hexagonal architecture and the three-tier test strategy the
  `java-hexagonal` skill defines, uniformly across every application

## Decision Outcome

Every application in this repository is a hexagonal (ports and adapters)
application as defined by the `java-hexagonal` skill, which is
authoritative for layer responsibilities, adapter-boundary mapping, and
the test strategy. The skill is the specification; this decision makes it
binding, including its three-tier test strategy.

## Rules

### architecture

#### hexagonal-per-skill

Every application follows hexagonal architecture per the `java-hexagonal`
skill. Dependencies point inward; the domain is pure Java with no
framework imports and no I/O; non-JDK libraries in the domain require an
explicit ArchUnit whitelist entry. Any framework is confined to the
composition root, or to a thin framework module delegating to
framework-free wiring underneath.

### testing

#### three-tier-test-strategy

Test in the three tiers the `java-hexagonal` skill defines: per-class
unit tests where branching exists and for every mapper; domain tests with
the real object graph and in-memory fakes for outbound ports;
integration tests with Testcontainers and Awaitility. No DI container and
no framework test context in any tier.

#### portable-integration-tests

Integration tests must pass against a standard Docker daemon, not only
inside a proxied-socket environment. Read container ports at runtime and
never hardcode them. The `testcontainers-java` skill is authoritative for
container setup.
