package pipeline

import (
	"errors"
	"testing"
)

// White-box: objectKey is unexported and is the one place a card becomes a
// path inside the bucket. What it may produce is pinned here because the key
// is written into cards.image_object_key and outlives every run that wrote it
// — changing its shape silently orphans every object already stored.
func TestObjectKeyNamesTheCardAndItsType(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name        string
		contentType string
		want        string
	}{
		{name: "jpeg", contentType: "image/jpeg", want: "cards/89631139.jpg"},
		{name: "png", contentType: "image/png", want: "cards/89631139.png"},
		{name: "webp", contentType: "image/webp", want: "cards/89631139.webp"},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			got, err := objectKey(89631139, testCase.contentType)
			if err != nil {
				t.Fatalf("objectKey returned %v", err)
			}
			if got != testCase.want {
				t.Errorf("objectKey = %q, want %q", got, testCase.want)
			}
		})
	}
}

// The guard: a type we cannot name must not become an object whose extension
// lies about its bytes, because nothing downstream could ever tell.
func TestObjectKeyRefusesATypeItCannotName(t *testing.T) {
	t.Parallel()

	got, err := objectKey(89631139, "text/html")

	if !errors.Is(err, errUnsupportedContentType) {
		t.Fatalf("objectKey error = %v, want errUnsupportedContentType", err)
	}
	if got != "" {
		t.Errorf("objectKey returned the key %q alongside an error", got)
	}
}
