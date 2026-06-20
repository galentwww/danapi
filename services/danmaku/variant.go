package danmaku

import (
	"net/url"
	"strings"
)

type Variant struct {
	WithRelated bool
}

func NormalizeVariant(values url.Values) Variant {
	raw := strings.TrimSpace(strings.ToLower(values.Get("withRelated")))
	return Variant{WithRelated: raw == "true" || raw == "1"}
}

func (v Variant) Key() string {
	if v.WithRelated {
		return "v1|withRelated=1"
	}
	return "v1|withRelated=0"
}

func (v Variant) UpstreamQuery() string {
	values := url.Values{}
	values.Set("chConvert", "1")
	if v.WithRelated {
		values.Set("withRelated", "true")
	} else {
		values.Set("withRelated", "false")
	}
	return values.Encode()
}
