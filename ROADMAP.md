# Roadmap

This document defines a high level roadmap for the galera cluster operator development.

The dates below should not be considered authoritative, but rather indicative of the projected timeline of the project.

### 2020 Q1

- Security
  - Server side TLS support
  - Client side TLS support
  - Migrate to CRD v1 (K8S >= 1.16) and use OpenAPI v3.0 validation for all fields in the schema

- Validating Rolling upgrade
  - Validate if a cluster can be upgraded with the current mariadb config file, will be implemented with the introdutions of new CRDs (upgradeconfigs & upgraderules)

- Backup scheduling:
  - implement the feature

- CI/CD
  - implement CircleCI to share e2e tests on GitHub
  