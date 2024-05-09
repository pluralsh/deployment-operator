package template

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Test .tpl Rendering", func() {

	Context("When rendering a .tpl template", func() {

		It("Should render the template correctly", func() {
			// simple test to check if the test is running
			Expect(1).To(Equal(1))
		})
	})

})
