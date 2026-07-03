---
id: ADR-0012
title: Require 2FA for repository administrators
category: security
date: 2026-06-23
status: accepted
source: FS-0012
---

## Context and Problem Statement

Admin-level repository access without a second factor is a single-point
credential-compromise risk.

## Considered Options

- Password-only admin access
- Require two-factor authentication for all admin roles

## Decision Outcome

All repository administrators must have two-factor authentication
enabled; org settings enforce it.

## Rule

Require two-factor authentication for all repository administrators.
