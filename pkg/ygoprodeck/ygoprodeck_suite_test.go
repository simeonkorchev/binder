package ygoprodeck_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestYgoprodeck(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "YGOPRODeck Suite")
}
