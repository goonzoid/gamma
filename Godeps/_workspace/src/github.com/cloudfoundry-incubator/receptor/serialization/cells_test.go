package serialization_test

import (
	"github.com/cloudfoundry-incubator/receptor"
	"github.com/cloudfoundry-incubator/receptor/serialization"
	"github.com/cloudfoundry-incubator/runtime-schema/models"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("CellPresence Serialization", func() {
	Describe("CellPresenceToCellResponse", func() {
		var cellPresence models.CellPresence

		BeforeEach(func() {
			cellPresence = models.CellPresence{
				CellID: "cell-id-0",
				Stack:  "stack-0",
			}
		})

		It("serializes all the fields", func() {
			expectedResponse := receptor.CellResponse{
				CellID: "cell-id-0",
				Stack:  "stack-0",
			}

			actualResponse := serialization.CellPresenceToCellResponse(cellPresence)
			Ω(actualResponse).Should(Equal(expectedResponse))
		})
	})
})
