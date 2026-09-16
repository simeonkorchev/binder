package objectstore_test

import (
	"context"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/simeonkorchev/binder/pkg/objectstore"
)

// Dir is the implementation these specs exercise. GCS is deliberately not
// covered: storage.googleapis.com is answered with 403 by the egress proxy in
// this repository's containers and there is no service account here, so its
// first real proof is a deployment. The behaviour the two share — key
// validation — lives in objectstore.go, so asserting it through Dir asserts it
// for both (pkg/objectstore/gcs.go says the same).
var _ = Describe("Dir", func() {
	const (
		jpeg = "image/jpeg"
		key  = "cards/89631139.jpg"
	)

	var (
		ctx     context.Context
		root    string
		subject *objectstore.Dir
		body    []byte
		putKey  string
		err     error
	)

	BeforeEach(func() {
		ctx = context.Background()
		root = GinkgoT().TempDir()
		body = []byte{0xFF, 0xD8, 0xFF, 0xE0}
		putKey = key

		var newErr error
		subject, newErr = objectstore.NewDir(root)
		Expect(newErr).NotTo(HaveOccurred())
	})

	JustBeforeEach(func() {
		err = subject.Put(ctx, putKey, body, jpeg)
	})

	When("the key is a clean relative path", func() {
		It("writes the body at that path under the root", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(os.ReadFile(filepath.Join(root, "cards", "89631139.jpg"))).To(Equal(body))
		})

		It("creates the key's parent directories", func() {
			Expect(err).NotTo(HaveOccurred())
			info, statErr := os.Stat(filepath.Join(root, "cards"))
			Expect(statErr).NotTo(HaveOccurred())
			Expect(info.IsDir()).To(BeTrue())
		})

		It("does not leave the object world-readable", func() {
			Expect(err).NotTo(HaveOccurred())
			info, statErr := os.Stat(filepath.Join(root, putKey))
			Expect(statErr).NotTo(HaveOccurred())
			Expect(info.Mode().Perm() & 0o007).To(BeZero())
		})
	})

	When("the same key is written twice", func() {
		BeforeEach(func() {
			Expect(subject.Put(ctx, key, []byte("stale"), jpeg)).To(Succeed())
		})

		It("replaces the object, so a retried card is not left half-written", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(os.ReadFile(filepath.Join(root, key))).To(Equal(body))
		})
	})

	When("traversal is attempted", func() {
		BeforeEach(func() {
			putKey = "../escaped.jpg"
		})

		It("writes nothing outside the root", func() {
			Expect(err).To(MatchError(objectstore.ErrInvalidKey))
			_, statErr := os.Stat(filepath.Join(filepath.Dir(root), "escaped.jpg"))
			Expect(os.IsNotExist(statErr)).To(BeTrue())
		})
	})

	When("the context is already cancelled", func() {
		BeforeEach(func() {
			cancelled, cancel := context.WithCancel(context.Background())
			cancel()
			ctx = cancelled
		})

		It("does not write the object", func() {
			Expect(err).To(MatchError(context.Canceled))
			_, statErr := os.Stat(filepath.Join(root, key))
			Expect(os.IsNotExist(statErr)).To(BeTrue())
		})
	})

	When("the body is empty", func() {
		BeforeEach(func() {
			body = nil
		})

		It("still writes an object, because an empty body is the caller's business", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(os.ReadFile(filepath.Join(root, key))).To(BeEmpty())
		})
	})
})

// Key validation gets its own container: it must assert that a rejected key
// left the root untouched, which a shared "write a valid object first" setup
// would make impossible to see.
var _ = Describe("Dir key validation", func() {
	DescribeTable("rejecting a key that is not a clean relative path",
		func(rejected string) {
			root := GinkgoT().TempDir()
			subject, err := objectstore.NewDir(root)
			Expect(err).NotTo(HaveOccurred())

			putErr := subject.Put(context.Background(), rejected, []byte("payload"), "image/jpeg")

			Expect(putErr).To(MatchError(objectstore.ErrInvalidKey))
			Expect(os.ReadDir(root)).To(BeEmpty(), "a rejected key must not have written anything")
		},
		Entry("empty", ""),
		Entry("absolute", "/etc/passwd"),
		Entry("climbing out of the root", "../escaped.jpg"),
		Entry("climbing out from inside", "cards/../../escaped.jpg"),
		Entry("a bare dot element", "cards/./89631139.jpg"),
		Entry("a doubled separator", "cards//89631139.jpg"),
		Entry("a trailing separator", "cards/"),
	)
})

var _ = Describe("NewDir", func() {
	var (
		root    string
		subject *objectstore.Dir
		err     error
	)

	BeforeEach(func() {
		root = filepath.Join(GinkgoT().TempDir(), "images")
	})

	JustBeforeEach(func() {
		subject, err = objectstore.NewDir(root)
	})

	When("the root does not exist yet", func() {
		It("creates it, so a caller need not prepare the directory", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(subject).NotTo(BeNil())

			info, statErr := os.Stat(root)
			Expect(statErr).NotTo(HaveOccurred())
			Expect(info.IsDir()).To(BeTrue())
		})
	})

	When("the root is empty", func() {
		BeforeEach(func() {
			root = ""
		})

		It("refuses rather than writing objects into the working directory", func() {
			Expect(err).To(MatchError(objectstore.ErrInvalidKey))
			Expect(subject).To(BeNil())
		})
	})
})
