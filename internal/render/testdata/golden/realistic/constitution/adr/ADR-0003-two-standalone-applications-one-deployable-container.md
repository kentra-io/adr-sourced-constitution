---
id: ADR-0003
title: Two standalone applications, one deployable container
date: 2026-06-03
status: accepted
---

## Context and Problem Statement

Validation and visualisation are separate concerns with separate
lifecycles, so they are separate applications. They still ship together
because the product should deploy as a single unit, and operators should
not have to wire two independently versioned deployables together by
hand.

## Considered Options

- One monolithic application handling both validation and visualisation
- Two standalone applications, communicating only over Kafka, packaged
  into a single deployable container

## Decision Outcome

The engine application and the UI application are separate standalone
applications, communicating only via Kafka topics, and they ship together
inside a single Docker container so the product deploys as one unit.

## Rules

### architecture

#### two-application-topology

The engine application and the UI application are separate standalone
applications. They communicate only via Kafka topics — the engine
consumes the source topic and publishes violations; the UI consumes the
violations topic. No shared in-process module and no direct calls between
them.

### tooling

#### single-container-packaging

Both applications ship inside a single Docker container so the product
deploys as one unit. The container build is the single release artefact;
there is no per-application image and no separate deployment pipeline to
keep in sync.
