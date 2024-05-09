# Deployment Operator Unit Tests


## Pre Reqs
Ensure that the cluster in your current kube context is reachable  
Helm tests will run against this cluster  
You can test with:
```sh
kubectl cluster-info
```

## Running Unit Tests
```sh
make test
```