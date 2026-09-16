package imagefetch_test

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/simeonkorchev/binder/cmd/cardimages/imagefetch"
	"github.com/simeonkorchev/binder/cmd/cardimages/model"
)

// The host these specs fetch from is an httptest server on localhost. The real
// one, images.ygoprodeck.com, is answered with 403 by the egress proxy in this
// repository's containers and has never been contacted — what the specs pin is
// the *assumed* contract (a card is served at <base>/<id>.jpg), not a verified
// one. specs/001-binder-mvp/VERIFY-YGOPRODECK.md is how a human confirms it.
var _ = Describe("Fetcher", func() {
	const cardID = int64(89631139)

	var (
		server   *httptest.Server
		handler  http.HandlerFunc
		requests []string

		ctx     context.Context
		image   model.Image
		err     error
		baseURL string
	)

	BeforeEach(func() {
		ctx = context.Background()
		requests = nil
		handler = func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "image/jpeg")
			_, _ = w.Write([]byte("jpeg bytes"))
		}

		server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requests = append(requests, r.URL.Path)
			handler(w, r)
		}))
		DeferCleanup(server.Close)

		baseURL = server.URL + "/images/cards"
	})

	JustBeforeEach(func() {
		fetcher, newErr := imagefetch.NewFetcher(server.Client(), baseURL)
		Expect(newErr).NotTo(HaveOccurred())

		image, err = fetcher.Fetch(ctx, cardID)
	})

	When("the host serves the card", func() {
		It("returns the bytes and the media type they were served with", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(image.Body).To(Equal([]byte("jpeg bytes")))
			Expect(image.ContentType).To(Equal("image/jpeg"))
		})

		It("asks for the card under its upstream id", func() {
			Expect(requests).To(ConsistOf("/images/cards/89631139.jpg"))
		})
	})

	When("the media type carries parameters", func() {
		BeforeEach(func() {
			handler = func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "image/jpeg; charset=binary")
				_, _ = w.Write([]byte("jpeg bytes"))
			}
		})

		It("keeps only the media type, which is what the object key is built from", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(image.ContentType).To(Equal("image/jpeg"))
		})
	})

	When("the host has no image for the card", func() {
		BeforeEach(func() {
			handler = func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusNotFound)
			}
		})

		It("reports the one expected absence, which the pipeline demotes below ERROR", func() {
			Expect(err).To(MatchError(model.ErrImageNotFound))
			Expect(image).To(Equal(model.Image{}))
		})
	})

	When("the host fails", func() {
		BeforeEach(func() {
			handler = func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
			}
		})

		It("is an unexpected failure, told apart from a missing image", func() {
			Expect(err).To(HaveOccurred())
			Expect(err).NotTo(MatchError(model.ErrImageNotFound))
			Expect(err).To(MatchError(ContainSubstring("500")))
		})
	})

	When("the body is larger than a card image could be", func() {
		BeforeEach(func() {
			handler = func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "image/jpeg")
				_, _ = w.Write(bytes.Repeat([]byte("x"), imagefetch.MaxImageBytes+1))
			}
		})

		It("refuses it rather than storing a truncated object", func() {
			Expect(err).To(MatchError(ContainSubstring("exceeds the size limit")))
			Expect(image.Body).To(BeEmpty())
		})
	})

	When("the body is exactly at the limit", func() {
		BeforeEach(func() {
			handler = func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "image/jpeg")
				_, _ = w.Write(bytes.Repeat([]byte("x"), imagefetch.MaxImageBytes))
			}
		})

		It("accepts it: the cap is a limit, not a limit minus one", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(image.Body).To(HaveLen(imagefetch.MaxImageBytes))
		})
	})

	When("the host serves no content type", func() {
		BeforeEach(func() {
			handler = func(w http.ResponseWriter, _ *http.Request) {
				w.Header()["Content-Type"] = nil
				w.WriteHeader(http.StatusOK)
			}
		})

		It("reports it rather than storing bytes it cannot label", func() {
			Expect(err).To(MatchError(ContainSubstring("reading content type")))
		})
	})

	When("the host cannot be reached", func() {
		BeforeEach(func() {
			server.Close()
		})

		It("reports the failure, and the card is left for the next run", func() {
			Expect(err).To(MatchError(ContainSubstring("fetching image for card")))
		})
	})

	When("the run is cancelled", func() {
		BeforeEach(func() {
			cancelled, cancel := context.WithCancel(context.Background())
			cancel()
			ctx = cancelled
		})

		It("gives up the request", func() {
			Expect(err).To(MatchError(context.Canceled))
		})
	})
})

var _ = Describe("NewFetcher", func() {
	It("refuses a nil client rather than panicking on the first card", func() {
		_, err := imagefetch.NewFetcher(nil, "https://images.test/cards")
		Expect(err).To(MatchError(ContainSubstring("http client is nil")))
	})

	It("refuses an empty base url, which would make every request relative", func() {
		_, err := imagefetch.NewFetcher(http.DefaultClient, "")
		Expect(err).To(MatchError(ContainSubstring("base url is empty")))
	})

	It("keeps the base path, so a host that serves art under a prefix still works", func() {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			Expect(strings.HasPrefix(r.URL.Path, "/art/pics/")).To(BeTrue())
			w.Header().Set("Content-Type", "image/png")
			_, _ = w.Write([]byte("png bytes"))
		}))
		DeferCleanup(server.Close)

		fetcher, err := imagefetch.NewFetcher(server.Client(), server.URL+"/art/pics")
		Expect(err).NotTo(HaveOccurred())

		image, err := fetcher.Fetch(context.Background(), 1)
		Expect(err).NotTo(HaveOccurred())
		Expect(image.ContentType).To(Equal("image/png"))
	})
})
