package objectstore_test

// The mem:// driver is registered by this file rather than by the package
// under test, because nothing in production opens one: the suites use it, and
// a blank import in production code would ship a backend no deployment wants.
import (
	"context"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/simeonkorchev/binder/pkg/objectstore"
	"gocloud.dev/blob"
	_ "gocloud.dev/blob/memblob"
)

var _ = Describe("Bucket", func() {
	var (
		ctx    context.Context
		bucket *objectstore.Bucket
		err    error
	)

	BeforeEach(func() {
		ctx = context.Background()
	})

	Describe("Open", func() {
		When("the url is empty", func() {
			It("says the store is unconfigured rather than returning a driver error", func() {
				_, err = objectstore.Open(ctx, "")
				Expect(err).To(MatchError(ContainSubstring("bucket url is empty")))
			})
		})

		When("the scheme has no registered driver", func() {
			It("fails at open, naming the url", func() {
				_, err = objectstore.Open(ctx, "s3://not-registered")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("s3://not-registered"))
			})
		})
	})

	Describe("Put", func() {
		JustBeforeEach(func() {
			bucket, err = objectstore.Open(ctx, "mem://")
			Expect(err).NotTo(HaveOccurred())
			DeferCleanup(func() { Expect(bucket.Close()).To(Succeed()) })
		})

		DescribeTable("rejects a key that is not a clean relative path",
			func(key string) {
				Expect(bucket.Put(ctx, key, []byte("x"), "image/jpeg")).
					To(MatchError(objectstore.ErrInvalidKey))
			},
			Entry("empty", ""),
			Entry("absolute", "/cards/1.jpg"),
			Entry("parent traversal", "cards/../../etc/passwd"),
			Entry("bare parent", ".."),
			Entry("current directory element", "cards/./1.jpg"),
			Entry("empty element", "cards//1.jpg"),
		)
	})

	Describe("the file:// backend, which is what local development runs", func() {
		var root string

		JustBeforeEach(func() {
			root = GinkgoT().TempDir()
			bucket, err = objectstore.Open(ctx, "file://"+filepath.ToSlash(root)+"?create_dir=true")
			Expect(err).NotTo(HaveOccurred())
			DeferCleanup(func() { Expect(bucket.Close()).To(Succeed()) })
		})

		It("writes the object to a real file under the key's path", func() {
			Expect(bucket.Put(ctx, "cards/46986414.jpg", []byte("art"), "image/jpeg")).To(Succeed())

			body, readErr := os.ReadFile(filepath.Join(root, "cards", "46986414.jpg"))
			Expect(readErr).NotTo(HaveOccurred())
			Expect(string(body)).To(Equal("art"))
		})

		It("overwrites an existing object, so a retried write replaces a partial one", func() {
			Expect(bucket.Put(ctx, "cards/1.jpg", []byte("first"), "image/jpeg")).To(Succeed())
			Expect(bucket.Put(ctx, "cards/1.jpg", []byte("second"), "image/jpeg")).To(Succeed())

			body, readErr := os.ReadFile(filepath.Join(root, "cards", "1.jpg"))
			Expect(readErr).NotTo(HaveOccurred())
			Expect(string(body)).To(Equal("second"))
		})

		It("records the content type, which the old directory store discarded", func() {
			Expect(bucket.Put(ctx, "cards/1.jpg", []byte("art"), "image/jpeg")).To(Succeed())

			attrs, attrErr := blobAttributes(ctx, root, "cards/1.jpg")
			Expect(attrErr).NotTo(HaveOccurred())
			Expect(attrs.ContentType).To(Equal("image/jpeg"))
		})

		It("keeps a traversing key inside the root", func() {
			Expect(bucket.Put(ctx, "../escaped.jpg", []byte("x"), "image/jpeg")).
				To(MatchError(objectstore.ErrInvalidKey))

			_, statErr := os.Stat(filepath.Join(filepath.Dir(root), "escaped.jpg"))
			Expect(os.IsNotExist(statErr)).To(BeTrue(), "nothing may be written outside the root")
		})
	})
})

// blobAttributes reads an object's stored metadata back through the same
// driver, which is how the content type is proven to survive a write.
func blobAttributes(ctx context.Context, root, key string) (*blob.Attributes, error) {
	b, err := blob.OpenBucket(ctx, "file://"+filepath.ToSlash(root))
	if err != nil {
		return nil, err
	}
	defer func() { _ = b.Close() }()
	return b.Attributes(ctx, key)
}
