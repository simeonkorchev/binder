package pipeline_test

import (
	"log/slog"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/simeonkorchev/binder/cmd/cardimages/pipeline"
)

var _ = Describe("FailureReason", func() {
	// 002-go-conventions.md section 4a: an expected failure is demoted rather
	// than logged at ERROR. Walking ~13k cards past a host that has no art for
	// some of them is expected; a timeout or a refused write is not, and would
	// be buried if the two shared a level.
	DescribeTable("the level one failed card is logged at",
		func(reason pipeline.FailureReason, want slog.Level) {
			Expect(reason.Level()).To(Equal(want))
		},
		Entry("upstream has no image for this card", pipeline.FailureNotFound, slog.LevelWarn),
		Entry("upstream could not be read", pipeline.FailureFetch, slog.LevelError),
		Entry("the bytes could not be labelled", pipeline.FailureUnsupportedType, slog.LevelError),
		Entry("the object store refused it", pipeline.FailureStore, slog.LevelError),
		Entry("the key could not be written back", pipeline.FailureRecord, slog.LevelError),
	)
})

var _ = Describe("Result", func() {
	When("nothing failed", func() {
		It("counts no failures", func() {
			result := pipeline.Result{Stored: 3, Failures: map[pipeline.FailureReason]int{}}

			Expect(result.Failed()).To(BeZero())
			Expect(result.Considered()).To(Equal(3))
		})
	})

	When("cards failed for several reasons", func() {
		It("adds them up across the reasons", func() {
			result := pipeline.Result{Stored: 1, Failures: map[pipeline.FailureReason]int{
				pipeline.FailureNotFound: 2,
				pipeline.FailureFetch:    3,
			}}

			Expect(result.Failed()).To(Equal(5))
			Expect(result.Considered()).To(Equal(6))
		})
	})
})
