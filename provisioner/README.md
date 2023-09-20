# Deployment Interface Spec

This repository hosts the gRPC API for the Deployment standard. The interfaces defined in the
[gRPC specification](proto/deployment.proto) are meant to be the common interface for database object provisioning
and management across various database object vendors.

### Build and Test

1. `deployment.proto` is generated from the specification defined in `spec.md`

2. In order to update the API, make changes to `spec.md`. Then, generate `database.proto` using:

```sh
make generate-spec
```

3. Do it all in 1 step:

```
# generates deployment.proto and builds the go bindings
make all
```
