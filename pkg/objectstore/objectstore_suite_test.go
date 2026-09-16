package objectstore_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestObjectstore(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Object Store Suite")
}
