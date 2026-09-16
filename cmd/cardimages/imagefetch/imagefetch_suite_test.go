package imagefetch_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestImagefetch(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Image Fetch Suite")
}
