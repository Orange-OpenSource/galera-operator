# Developer guide

This document explains how to setup your development environment.

## How to build

Requirement: Go 1.13+

Clone the project locally, use `git tag` to add a tag used to version the project.

A makefile is provided to build the project, you can just compile it, build the container or also push it to the designated repository.

Do not forget to run `make codegen` is you modify a file in ./pkg/api


## Fetch dependency

[go mod](https://blog.golang.org/using-go-modules) is used to manage dependency.
Install dependency if you have not by building the project.

```bash
$ make build
```

