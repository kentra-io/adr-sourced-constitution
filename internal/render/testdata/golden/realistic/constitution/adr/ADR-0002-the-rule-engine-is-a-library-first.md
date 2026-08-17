---
id: ADR-0002
title: The rule engine is a library first
date: 2026-06-02
status: accepted
---

## Context and Problem Statement

The rule-evaluation engine must be usable by consumers who want semantic
validation without adopting kafka-dq's deployment, its transport, or its
JVM framework choice. Everything else in this repository is a consumer of
that library, not the other way around.

## Considered Options

- Couple the engine directly to the Kafka runtime and the deployment
  container
- Ship the engine as a standalone library that applications and adapters
  depend on

## Decision Outcome

The rule-evaluation engine is a standalone library, consumable
independently of the Docker deployment, the Kafka runtime, and any JVM
framework. Kafka, transport, storage, and framework concerns live in
applications and adapters that depend on the library — never the reverse.

## Rules

### architecture

#### library-first-engine

The rule-evaluation engine is a standalone library, consumable
independently of the Docker deployment, the Kafka runtime, and any JVM
framework. Kafka, transport, storage, and framework concerns live in
applications and adapters that depend on the library — never the reverse.
