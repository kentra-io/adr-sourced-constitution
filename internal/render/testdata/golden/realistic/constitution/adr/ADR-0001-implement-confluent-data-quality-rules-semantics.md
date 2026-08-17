---
id: ADR-0001
title: Implement Confluent Data Quality Rules semantics
date: 2026-06-01
status: accepted
---

## Context and Problem Statement

kafka-dq exists to provide an open-source implementation of Confluent
Schema Registry Data Quality Rules, evaluated against live Kafka topics to
track semantic correctness. Conforming to an existing standard — rather
than designing a competing one — is the point of the project: rules
authored for Confluent must evaluate here with the same meaning.

## Considered Options

- Design a proprietary rule syntax and evaluation semantics
- Implement Confluent's Data Quality Rules semantics exactly, using CEL
  as the rule expression language

## Decision Outcome

Implement the semantics of Confluent Schema Registry Data Quality Rules,
using CEL (Common Expression Language) as the rule expression syntax. Do
not introduce a proprietary rule syntax or knowingly diverge from
Confluent's semantics; where behaviour is ambiguous, follow Confluent's
implementation.

## Rules

### purpose

#### confluent-dq-compatibility

Implement the semantics of Confluent Schema Registry Data Quality Rules.
Use CEL (Common Expression Language) as the rule expression syntax. Do not
introduce a proprietary rule syntax or knowingly diverge from Confluent's
semantics; where behaviour is ambiguous, follow Confluent's
implementation.
