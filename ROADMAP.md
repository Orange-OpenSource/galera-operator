# Roadmap

This document defines a high level roadmap for the galera cluster operator development.

The dates below should not be considered authoritative, but rather indicative of the projected timeline of the project.

### 2020 S1

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

### 2020 S2

- Machine Learning
  - build on top of this operator a system to manage galera clusters, users will not have to provide any resource requirements, sizing will be operated by the system based on observation and  datas collected from managed cluster. Upgrading will be automated and scheduled when databases are less used. Upgrade will be also validated by checking the client calls (version checking)
  - multi-operators running on different Kubernetes clusters

- API for galera cluster operations

  