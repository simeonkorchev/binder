package oidc_test

import (
	"context"
	"time"

	"github.com/go-jose/go-jose/v4"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/simeonkorchev/binder/pkg/oidc"
)

var _ = Describe("CachedKeys", func() {
	const (
		ttl     = 15 * time.Minute
		refetch = time.Minute
	)

	var (
		published signingKey
		rotated   signingKey
		fetcher   *countingFetcher
		clock     time.Time
		cache     *oidc.CachedKeys
	)

	BeforeEach(func() {
		published = newSigningKey("published-key")
		rotated = newSigningKey("rotated-key")
		fetcher = &countingFetcher{set: keySet(published), err: nil, calls: 0}
		clock = time.Date(2026, time.September, 17, 12, 0, 0, 0, time.UTC)

		var err error
		cache, err = oidc.NewCachedKeys(oidc.CacheConfig{
			Fetcher:            fetcher,
			TTL:                ttl,
			MinRefetchInterval: refetch,
			Now:                func() time.Time { return clock },
		})
		Expect(err).NotTo(HaveOccurred())
	})

	Describe("Key", func() {
		When("the key set has never been fetched", func() {
			It("fetches it and returns the key", func() {
				key, err := cache.Key(context.Background(), published.keyID)
				Expect(err).NotTo(HaveOccurred())
				Expect(key.KeyID).To(Equal(published.keyID))
				Expect(fetcher.calls).To(Equal(1))
			})
		})

		When("the same key is asked for again inside the TTL", func() {
			It("serves it from memory rather than asking the provider again", func() {
				_, err := cache.Key(context.Background(), published.keyID)
				Expect(err).NotTo(HaveOccurred())

				clock = clock.Add(ttl - time.Second)
				key, err := cache.Key(context.Background(), published.keyID)
				Expect(err).NotTo(HaveOccurred())
				Expect(key.KeyID).To(Equal(published.keyID))
				Expect(fetcher.calls).To(Equal(1))
			})
		})

		When("the TTL has run out", func() {
			It("fetches the set again", func() {
				_, err := cache.Key(context.Background(), published.keyID)
				Expect(err).NotTo(HaveOccurred())

				clock = clock.Add(ttl)
				_, err = cache.Key(context.Background(), published.keyID)
				Expect(err).NotTo(HaveOccurred())
				Expect(fetcher.calls).To(Equal(2))
			})
		})

		When("the provider rotated to a key the cached set does not hold", func() {
			It("fetches again once the refetch floor has passed, and finds it", func() {
				_, err := cache.Key(context.Background(), published.keyID)
				Expect(err).NotTo(HaveOccurred())

				fetcher.set = keySet(published, rotated)
				clock = clock.Add(refetch)

				key, err := cache.Key(context.Background(), rotated.keyID)
				Expect(err).NotTo(HaveOccurred())
				Expect(key.KeyID).To(Equal(rotated.keyID))
				Expect(fetcher.calls).To(Equal(2))
			})
		})

		When("an unknown key id arrives inside the refetch floor", func() {
			It("reports it unknown without going to the provider", func() {
				_, err := cache.Key(context.Background(), published.keyID)
				Expect(err).NotTo(HaveOccurred())

				clock = clock.Add(refetch - time.Second)
				_, err = cache.Key(context.Background(), "invented-key-id")
				Expect(err).To(MatchError(oidc.ErrKeyUnknown))
				Expect(fetcher.calls).To(Equal(1),
					"a stream of forged key ids must not become a stream of requests to the provider")
			})
		})

		When("the key id is unknown even after a fresh fetch", func() {
			It("reports it unknown", func() {
				_, err := cache.Key(context.Background(), "invented-key-id")
				Expect(err).To(MatchError(oidc.ErrKeyUnknown))
				Expect(fetcher.calls).To(Equal(1))
			})
		})

		When("the fetch fails", func() {
			BeforeEach(func() {
				fetcher.err = errFetch
			})

			It("reports the failure rather than an unknown key", func() {
				_, err := cache.Key(context.Background(), published.keyID)
				Expect(err).To(MatchError(errFetch))
				Expect(err).NotTo(MatchError(oidc.ErrKeyUnknown))
			})
		})

		When("the provider publishes two keys under one id", func() {
			BeforeEach(func() {
				duplicate := newSigningKey(published.keyID)
				fetcher.set = keySet(published, duplicate)
			})

			It("uses neither: choosing one would be a coin toss", func() {
				_, err := cache.Key(context.Background(), published.keyID)
				Expect(err).To(MatchError(oidc.ErrKeyUnknown))
			})
		})
	})

	Describe("NewCachedKeys", func() {
		It("refuses a cache with no fetcher", func() {
			_, err := oidc.NewCachedKeys(oidc.CacheConfig{Fetcher: nil, TTL: ttl, MinRefetchInterval: refetch, Now: nil})
			Expect(err).To(HaveOccurred())
		})

		It("refuses a refetch floor above the TTL, which would strand a stale set", func() {
			_, err := oidc.NewCachedKeys(oidc.CacheConfig{
				Fetcher:            fetcher,
				TTL:                time.Minute,
				MinRefetchInterval: time.Hour,
				Now:                nil,
			})
			Expect(err).To(HaveOccurred())
		})

		It("defaults both durations, so only the fetcher is required", func() {
			defaulted, err := oidc.NewCachedKeys(oidc.CacheConfig{Fetcher: fetcher, TTL: 0, MinRefetchInterval: 0, Now: nil})
			Expect(err).NotTo(HaveOccurred())

			key, err := defaulted.Key(context.Background(), published.keyID)
			Expect(err).NotTo(HaveOccurred())
			Expect(key.KeyID).To(Equal(published.keyID))
		})
	})
})

var _ = Describe("HTTPFetcher", func() {
	var published signingKey

	BeforeEach(func() {
		published = newSigningKey("served-key")
	})

	When("the endpoint serves a key set", func() {
		It("returns it", func() {
			server := jwksServer(200, keySet(published))
			DeferCleanup(server.Close)

			set, err := oidc.NewHTTPFetcher(server.URL, server.Client()).Fetch(context.Background())
			Expect(err).NotTo(HaveOccurred())
			Expect(set.Key(published.keyID)).To(HaveLen(1))
		})
	})

	When("the endpoint refuses the request", func() {
		It("reports the status rather than an empty key set", func() {
			server := jwksServer(403, nil)
			DeferCleanup(server.Close)

			_, err := oidc.NewHTTPFetcher(server.URL, server.Client()).Fetch(context.Background())
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("403"))
		})
	})

	When("the endpoint serves a key set with no keys in it", func() {
		It("reports it unusable, so an empty set is never cached as the truth", func() {
			server := jwksServer(200, jose.JSONWebKeySet{Keys: nil})
			DeferCleanup(server.Close)

			_, err := oidc.NewHTTPFetcher(server.URL, server.Client()).Fetch(context.Background())
			Expect(err).To(HaveOccurred())
		})
	})

	When("the endpoint serves something that is not a key set", func() {
		It("reports it", func() {
			server := jwksServer(200, map[string]string{"error": "nope"})
			DeferCleanup(server.Close)

			_, err := oidc.NewHTTPFetcher(server.URL, server.Client()).Fetch(context.Background())
			Expect(err).To(HaveOccurred())
		})
	})
})
