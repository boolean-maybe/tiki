---
id: "5YWWFF"
---
# Operations

Manufacturing, testing, and maintenance procedures for the Celestia drone fleet.

## Contents

| Document | Description |
| --- | --- |
| [Manufacturing Process](manufacturing.md) | Each Celestia drone goes through a six-stage manufacturing pipeline from bare PC... |
| [Field Testing Procedures](field-testing.md) | Field testing validates drone behaviour in real-world conditions before fleet de... |
| [Incident Response](incident-response.md) | The incident response plan defines how the Celestia team handles drone malfuncti... |
| [Maintenance Schedule](maintenance.md) | Celestia drones follow a preventive maintenance schedule based on flight hours a... |

## Section Overview

```mermaid
graph LR
    A[Manufacturing] --> B[Field Testing]
    B --> C[Deployment]
    C --> D[Maintenance]
    D --> E[Incident Response]
    E --> B
```
