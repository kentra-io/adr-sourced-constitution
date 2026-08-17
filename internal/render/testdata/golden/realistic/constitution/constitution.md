<!--
  GENERATED FILE -- projection of the ADR log in constitution/adr/.
  Do not hand-edit; changes will be overwritten by the next "constitution
  regen". Only the rules (## Rules entries) of active ADRs project here; to
  change a rule, add, supersede, or deprecate an ADR instead.
-->

# Constitution

The source of truth for this project's standing technical decisions — how
recurring problems are solved (architecture, mapping, testing, process) — so
that requirements can stay functional and need not re-explain implementation
choices.

## purpose

### confluent-dq-compatibility

Implement the semantics of Confluent Schema Registry Data Quality Rules.
Use CEL (Common Expression Language) as the rule expression syntax. Do not
introduce a proprietary rule syntax or knowingly diverge from Confluent's
semantics; where behaviour is ambiguous, follow Confluent's
implementation.

ADR-0001 · 2026-06-01

## architecture

### library-first-engine

The rule-evaluation engine is a standalone library, consumable
independently of the Docker deployment, the Kafka runtime, and any JVM
framework. Kafka, transport, storage, and framework concerns live in
applications and adapters that depend on the library — never the reverse.

ADR-0002 · 2026-06-02

### two-application-topology

The engine application and the UI application are separate standalone
applications. They communicate only via Kafka topics — the engine
consumes the source topic and publishes violations; the UI consumes the
violations topic. No shared in-process module and no direct calls between
them.

ADR-0003 · 2026-06-03

### hexagonal-per-skill

Every application follows hexagonal architecture per the `java-hexagonal`
skill. Dependencies point inward; the domain is pure Java with no
framework imports and no I/O; non-JDK libraries in the domain require an
explicit ArchUnit whitelist entry. Any framework is confined to the
composition root, or to a thin framework module delegating to
framework-free wiring underneath.

ADR-0004 · 2026-06-04

## testing

### three-tier-test-strategy

Test in the three tiers the `java-hexagonal` skill defines: per-class
unit tests where branching exists and for every mapper; domain tests with
the real object graph and in-memory fakes for outbound ports;
integration tests with Testcontainers and Awaitility. No DI container and
no framework test context in any tier.

ADR-0004 · 2026-06-04

### portable-integration-tests

Integration tests must pass against a standard Docker daemon, not only
inside a proxied-socket environment. Read container ports at runtime and
never hardcode them. The `testcontainers-java` skill is authoritative for
container setup.

ADR-0004 · 2026-06-04

## tooling

### single-container-packaging

Both applications ship inside a single Docker container so the product
deploys as one unit. The container build is the single release artefact;
there is no per-application image and no separate deployment pipeline to
keep in sync.

ADR-0003 · 2026-06-03

### license-compatible-dependencies

Every dependency must be compatible with permissive redistribution. Do
not introduce copyleft (GPL/AGPL/LGPL) or proprietary dependencies.

ADR-0005 · 2026-06-05

## process

### permissive-license

Release this project under a permissive open-source licence (MIT or
Apache-2.0).

ADR-0005 · 2026-06-05

### spec-lifecycle-gates

Take all non-trivial work through spec-lifecycle. Read the relevant
`openspec/changes/<change>/` artefacts before touching related code, and
run `lifecycle status` to see gate state. Approve gates only via
`lifecycle approve` — never by hand-editing `approval-state.json`.

ADR-0006 · 2026-06-06
