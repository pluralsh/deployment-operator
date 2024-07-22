/*
Copyright 2024.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package cache

import (
	"testing"

	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	deploymentsv1alpha1 "github.com/pluralsh/deployment-operator/api/v1alpha1"

	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

// These tests use Ginkgo (BDD-style Go testing framework). Refer to
// http://onsi.github.io/ginkgo/ to learn more about Ginkgo.
var kClient client.Client
var cfg *rest.Config
var err error

func TestControllers(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Resource Cache Suite")
}

var _ = BeforeSuite(func() {
	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))

	By("bootstrapping test environment")
	cfg = ctrl.GetConfigOrDie()

	Expect(cfg).NotTo(BeNil())
	Expect(deploymentsv1alpha1.AddToScheme(scheme.Scheme)).NotTo(HaveOccurred())
	Expect(extv1.AddToScheme(scheme.Scheme)).NotTo(HaveOccurred())

	kClient, err = client.New(cfg, client.Options{
		Scheme: scheme.Scheme,
	})
	Expect(err).NotTo(HaveOccurred())
	Expect(kClient).NotTo(BeNil())
})

var _ = AfterSuite(func() {
	By("tearing down the test environment")

})
