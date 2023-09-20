# Database Interface Spec

This repository hosts the gRPC API for the Database standard. The interfaces defined in the [gRPC specification](database.proto) are meant to be the common interface for database object provisioning and management across various database object vendors.

### Build and Test

1. `database.proto` is generated from the specification defined in `spec.md`

2. In order to update the API, make changes to `spec.md`. Then, generate `database.proto` using:

```sh
# generates cosi.proto
make generate-spec
```

3. Do it all in 1 step:

```
# generates database.proto and builds the go bindings
make all
```
