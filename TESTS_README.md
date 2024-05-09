# Deployment Operator Unit Tests


## Pre Reqs
### Ensure that the cluster in your current kube context is reachable  
Helm tests will run against this cluster  
You can test with:
```sh
kubectl cluster-info
```
Set the KUBEBUILDER_ASSETS directory
### Install dependencies with make
```sh
make tools
```
### Setup Environment
```sh
# Mac
export KUBEBUILDER_ASSETS=${GOBIN}/k8s/1.28.3-darwin-arm64

# Linux
export KUBEBUILDER_ASSETS=${GOBIN}/k8s/1.28.3-linux-amd64
```



## Running Unit Tests
```sh
make test
```