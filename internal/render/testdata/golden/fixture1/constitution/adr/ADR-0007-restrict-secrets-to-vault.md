---
id: ADR-0007
title: Restrict secrets to the managed vault
date: 2026-06-12
status: accepted
source: FS-0007
---

## Context and Problem Statement

Secrets committed to source or passed as plain environment variables leak
through logs, shell history, and CI artifacts.

## Considered Options

- Plain environment variables
- A managed secrets vault with scoped, audited access

## Decision Outcome

All secrets must be stored in the managed vault and injected at runtime;
none may be committed or passed as plain environment variables in CI
configuration.

## Rules

### security

#### vault-secrets

Store all secrets in the managed vault; never commit them or pass them as
plain CI environment variables.
