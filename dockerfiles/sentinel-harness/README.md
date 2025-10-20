# Building a Sentinel Harness Test Image with Terratest Suites

This guide explains how to use the `sentinel-harness-base` image to build a new container image that packages your own 
Terratest end-to-end (E2E) tests. This image can then be run as a Kubernetes Job by the Sentinel Harness system to automatically
execute tests against your cluster.

The resulting image is suitable for running as a Kubernetes Job, which automatically:
 - Executes your test suite under the control of the sentinel-harness binary
 - Generates two test report files:
   - /plural/unit-tests.xml (JUnit format)
   - /plural/unit-tests.json (plaintext format)
 - Sends results back to the Plural Console

## Building the Image
To build a working Terratest image, your Dockerfile must include the following key parts:

### Base Image Import
Start with the `sentinel-harness-base` image.
This provides the compiled `sentinel-harness` binary responsible for managing and reporting tests.

```
FROM ghcr.io/pluralsh/sentinel-harness-base:<tag> AS harness
```

### Go Runtime Environment
Next, add the Go runtime environment.
This is required to compile your test suite.

```
FROM golang:1.25-alpine AS final
```

### Install Dependencies
Install any additional dependencies your tests may require.
For example, if your tests interact with AWS services, you may need to install the AWS CLI
The `Terratest` uses kubectl to interact with the cluster.

```
RUN apk add --no-cache curl ca-certificates && \
    KUBECTL_VERSION=$(curl -L -s https://dl.k8s.io/release/stable.txt) && \
    curl -L -o /usr/local/bin/kubectl \
      "https://dl.k8s.io/release/${KUBECTL_VERSION}/bin/${TARGETOS}/${TARGETARCH}/kubectl" && \
    chmod +x /usr/local/bin/kubectl
```

### Copy Test Suite sentinel-harness Binary
Finally, copy your test suite into the container.
```
COPY --from=harness /sentinel-harness /usr/local/bin/sentinel-harness
# Change ownership of the harness binary to UID/GID 65532
RUN chown -R 65532:65532 /usr/local/bin/sentinel-harness
COPY dockerfiles/sentinel-harness/terratest /sentinel
```

### Entrypoint

Define the entrypoint to execute the harness binary with the required arguments.
```
ENTRYPOINT ["sentinel-harness", "--test-dir=/sentinel", "--output-dir=/plural"]
```
This command:
 - Executes all tests in `/sentinel`
 - Writes `unit-tests.xml` and `unit-tests.json` to /plural
 - Returns compatible output to Plural Console