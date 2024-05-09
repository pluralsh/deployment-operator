package template_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Tpl", func() {

	Context("Example Test", func() {
		It("Should always Pass", func() {
			Expect(1).To(Equal(1))
		})
	})

	Context("Test Should Fail for example output", func() {
		It("Should always Fail", func() {
			Expect(1).To(Equal(2))
		})
	})

})
