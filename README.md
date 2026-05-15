# Deployment Operator

The Deployment Operator code was moved to the [Console repository](https://github.com/pluralsh/console/tree/master/go/deployment-operator). This repository contains only its Helm chart.

Container images are built from the Console repository, and chart bump pull requests are opened against this repository from the Console CD workflow.

## Testing the Helm Chart

To verify that the Helm chart installs successfully, run:

```sh
./test/helm/test-chart-install.sh
```

This script will:

1. Create a temporary `kind` cluster.
2. Validate the chart using `helm lint`.
3. Verify template rendering with `helm template`.
4. Perform a dry-run installation with `helm install --dry-run`.
5. Automatically clean up the cluster when the test completes.

See [test/helm/README.md](test/helm/README.md) for additional details.
