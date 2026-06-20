package danmaku

import (
	"net/url"
	"testing"
)

func TestNormalizeVariantWithRelatedTrue(t *testing.T) {
	for _, raw := range []string{"withRelated=true", "withRelated=1", "withRelated=TRUE"} {
		values, err := url.ParseQuery(raw)
		if err != nil {
			t.Fatalf("parse query: %v", err)
		}

		variant := NormalizeVariant(values)

		if variant.Key() != "v1|withRelated=1" {
			t.Fatalf("Key() = %q for %q", variant.Key(), raw)
		}
		if variant.UpstreamQuery() != "chConvert=1&withRelated=true" {
			t.Fatalf("UpstreamQuery() = %q for %q", variant.UpstreamQuery(), raw)
		}
	}
}

func TestNormalizeVariantWithRelatedFalse(t *testing.T) {
	for _, raw := range []string{"", "withRelated=false", "withRelated=0", "withRelated=no"} {
		values, err := url.ParseQuery(raw)
		if err != nil {
			t.Fatalf("parse query: %v", err)
		}

		variant := NormalizeVariant(values)

		if variant.Key() != "v1|withRelated=0" {
			t.Fatalf("Key() = %q for %q", variant.Key(), raw)
		}
		if variant.UpstreamQuery() != "chConvert=1&withRelated=false" {
			t.Fatalf("UpstreamQuery() = %q for %q", variant.UpstreamQuery(), raw)
		}
	}
}

func TestUpstreamQueryRequestsSimplifiedChineseDanmaku(t *testing.T) {
	values, err := url.ParseQuery("withRelated=true")
	if err != nil {
		t.Fatalf("parse query: %v", err)
	}

	query := NormalizeVariant(values).UpstreamQuery()

	if query != "chConvert=1&withRelated=true" {
		t.Fatalf("UpstreamQuery() = %q", query)
	}
}

func TestNormalizeVariantIgnoresUnrelatedQuery(t *testing.T) {
	left, err := url.ParseQuery("withRelated=true&_t=123&trace=abc")
	if err != nil {
		t.Fatalf("parse left query: %v", err)
	}
	right, err := url.ParseQuery("withRelated=true")
	if err != nil {
		t.Fatalf("parse right query: %v", err)
	}

	if NormalizeVariant(left).Key() != NormalizeVariant(right).Key() {
		t.Fatalf("unrelated query changed variant: %q != %q", NormalizeVariant(left).Key(), NormalizeVariant(right).Key())
	}
}
