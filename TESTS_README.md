# Deployment Operator Unit Tests


## Pre Reqs
### Ensure that the cluster in your current kube context is reachable  
Helm tests will run against this cluster  
You can test with:
```sh
kubectl cluster-info
```

### Install dependencies with make
```sh
make tools
```
### Setup Environment
Set the `KUBEBUILDER_ASSETS` directory
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

## Adding Tests
Reference the [Ginkgo Getting Started](https://onsi.github.io/ginkgo/#getting-started) to see the expected structure
### Install the Ginkgo CLI
```sh
go install github.com/onsi/ginkgo/v2/ginkgo
```
### The Test Suites for several Packages are already Generated in the Deployment-Operator Repo
If creating a new package or testing a package that doesn't already have a test suite
```sh
cd pkg/package/that/needs/suite
ginkgo bootstrap
```

### Generate A Basic test
I'm creating a test for  ./pkg/manifests/template/tpl.go 
```sh
cd ./pkg/manifests/template
ginkgo generate book
```
It generates
```sh
# ./pkg/manifests/template/tpl_test.go
package template_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/pluralsh/deployment-operator/pkg/manifests/template"
)

var _ = Describe("Tpl", func() {

})

```
### From here you can begin adding `specs` (test) to your generated file
```sh
var _ = Describe("Tpl", func() {

	Context("Example Test", func() {
		It("Should always Pass", func() {
			Expect(1).To(Equal(1))
		})
	})

    Context("Test Should Fail for example output", func() {
		It("Should always Pass", func() {
			Expect(1).To(Equal(2))
		})
	})

})
```

### Run the Suite with your New Test
```sh
# I'm doing this here just for an example and to check that my tests are bing added to the Suite
make test
# ... other output

Will run 6 of 6 specs
••# Warning: 'bases' is deprecated. Please use 'resources' instead. Run 'kustomize edit fix' to update your Kustomization automatically.
# Warning: 'patchesStrategicMerge' is deprecated. Please use 'patches' instead. Run 'kustomize edit fix' to update your Kustomization automatically.
•2024/05/09 16:47:43 render helm templates: enable dependency update= false dependencies= 0
Found unknown types unknown resource types: apiextensions.k8s.io/v1/CustomResourceDefinition,apiextensions.k8s.io/v1/CustomResourceDefinition, ignoring for now2024/05/09 16:47:44 Server exiting
••
------------------------------
• [FAILED] [0.000 seconds]
Tpl Test Should Fail for example output [It] Should always Pass
/Users/kjj/git/plrl/deployment-operator/pkg/manifests/template/tpl_test.go:17

  [FAILED] Expected
      <int>: 1
  to equal
      <int>: 2
  In [It] at: /Users/kjj/git/plrl/deployment-operator/pkg/manifests/template/tpl_test.go:18 @ 05/09/24 16:47:44.489
------------------------------

Summarizing 1 Failure:
  [FAIL] Tpl Test Should Fail for example output [It] Should always Pass
  /Users/kjj/git/plrl/deployment-operator/pkg/manifests/template/tpl_test.go:18

Ran 6 of 6 Specs in 4.158 seconds
FAIL! -- 5 Passed | 1 Failed | 0 Pending | 0 Skipped
--- FAIL: TestControllers (4.16s)
FAIL
FAIL    github.com/pluralsh/deployment-operator/pkg/manifests/template  4.769s
FAIL
make: *** [test] Error 1

```
