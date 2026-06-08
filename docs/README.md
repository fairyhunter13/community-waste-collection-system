# Documentation

Knowledge base for the Community Waste Collection API. Start here to
navigate to the right document.

---

## Document Index

| Document | Contents |
|---|---|
| [architecture.md](architecture.md) | Layer responsibilities, module dependency graph, DI wiring, observability, wire surface for all 15 endpoints, domain invariants |
| [business-processes.md](business-processes.md) | Pickup + payment lifecycle state machines, BR-01 creation gate, BR-05 atomic transaction, BR-06 proof upload flow, BR-04 worker timeline |
| [data-model.md](data-model.md) | ER diagram, index strategy, partial UNIQUE constraints, migration history |
| [api-reference.md](api-reference.md) | Endpoint map, response envelope contract, error codes, rate-limit config, links to OpenAPI/Postman/Insomnia |
| [deployment.md](deployment.md) | Docker Compose topology, observability data flow (metrics/logs/traces), graceful shutdown sequence |
| [operations.md](operations.md) | Failure-mode decision tree, health endpoints, log correlation, recovery commands |
| [coverage-matrix.md](coverage-matrix.md) | Full coverage matrix: every entity, endpoint, business rule, technical requirement, and deliverable mapped to code + tests + CI job |
| [plans/README.md](plans/README.md) | Index of phase-by-phase engineering records |

---

## Where Do I…?

| Question | Go here |
|---|---|
| Understand how requests flow through the layers | [architecture.md → Request flow](architecture.md#request-flow) |
| See the full pickup state machine | [business-processes.md → Pickup Lifecycle](business-processes.md#pickup-lifecycle) |
| Understand why BR-01 can't be bypassed | [business-processes.md → BR-01 Gate](business-processes.md#pickup-creation--br-01-gate) |
| Trace the complete-pickup → payment creation code path | [business-processes.md → BR-05 Atomic Transaction](business-processes.md#complete-pickup--br-05-atomic-transaction) |
| See what happens when a proof upload fails | [business-processes.md → BR-06 Proof Upload](business-processes.md#payment-confirm--br-06-proof-upload-flow) |
| Understand the DB schema and indexes | [data-model.md](data-model.md) |
| Find the API endpoint for X | [api-reference.md → Endpoint Map](api-reference.md#endpoint-map) |
| Find which status code a validation error returns | [api-reference.md → Response Envelope](api-reference.md#response-envelope) |
| Understand the rate-limit knobs | [api-reference.md → Rate Limiting](api-reference.md#rate-limiting) |
| Trace how logs, metrics, and traces connect | [deployment.md → Observability Data Flow](deployment.md#observability-data-flow) |
| Understand graceful shutdown | [deployment.md → Graceful Shutdown Sequence](deployment.md#graceful-shutdown-sequence) |
| Diagnose a 5xx error | [operations.md](operations.md) |
| Diagnose the worker not canceling pickups | [operations.md → Failure Mode Decision Tree](operations.md#failure-mode-decision-tree) |
| See which test covers endpoint X | [coverage-matrix.md → Endpoints](coverage-matrix.md#b-endpoints) |
| Confirm a business rule is enforced at all three layers | [coverage-matrix.md → Business Rules](coverage-matrix.md#c-business-rules) |
| Set up the full local stack | [../README.md → Quick Start](../README.md#quick-start) |
| Configure environment variables | [../README.md → Environment Variables](../README.md#environment-variables) |

---

## Diagrams Quick Reference

| Diagram | File | Type |
|---|---|---|
| Layered architecture | [architecture.md](architecture.md#layered-architecture) | `graph TD` |
| Module dependency graph | [architecture.md](architecture.md#module-dependency-graph) | `graph LR` |
| DI wiring | [architecture.md](architecture.md#dependency-injection-wiring) | `graph TD` |
| Pickup lifecycle | [business-processes.md](business-processes.md#pickup-lifecycle) | `stateDiagram-v2` |
| Payment lifecycle | [business-processes.md](business-processes.md#payment-lifecycle) | `stateDiagram-v2` |
| BR-01 creation gate | [business-processes.md](business-processes.md#pickup-creation--br-01-gate) | `flowchart TD` |
| BR-05 atomic tx | [business-processes.md](business-processes.md#complete-pickup--br-05-atomic-transaction) | `sequenceDiagram` |
| BR-06 proof upload | [business-processes.md](business-processes.md#payment-confirm--br-06-proof-upload-flow) | `sequenceDiagram` |
| BR-04 worker | [business-processes.md](business-processes.md#br-04-worker--organic-auto-cancel) | `sequenceDiagram` |
| ER diagram | [data-model.md](data-model.md#entity-relationship-diagram) | `erDiagram` |
| Index strategy | [data-model.md](data-model.md#index-strategy) | `graph TD` |
| Endpoint map | [api-reference.md](api-reference.md#endpoint-map) | `graph TD` |
| Docker Compose topology | [deployment.md](deployment.md#docker-compose-topology) | `graph LR` |
| Observability data flow | [deployment.md](deployment.md#observability-data-flow) | `graph LR` |
| Graceful shutdown | [deployment.md](deployment.md#graceful-shutdown-sequence) | `sequenceDiagram` |
| Failure-mode runbook | [operations.md](operations.md#failure-mode-decision-tree) | `flowchart TD` |
