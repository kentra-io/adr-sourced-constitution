---
id: ADR-0008
title: Store all timestamps in UTC
category: data
date: 2026-06-13
status: accepted
source: FS-0008
---

## Context and Problem Statement

Mixed local-time and UTC timestamps across services make cross-service
correlation and debugging unreliable.

## Considered Options

- Store timestamps in the producing service's local time zone
- Store all timestamps in UTC, convert for display only

## Decision Outcome

All persisted and logged timestamps must be UTC; local-time conversion
happens only at the presentation layer.
