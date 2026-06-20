package danmaku

import (
	"bytes"
	"testing"
)

func TestValidatePayloadCountsComments(t *testing.T) {
	body := []byte(`{"count":2,"comments":[{"cid":1,"p":"0.00,1,16777215,abc","m":"one"},{"cid":2,"p":"1.00,1,16777215,def","m":"two"}]}`)

	info, err := ValidatePayload(body)
	if err != nil {
		t.Fatalf("ValidatePayload returned error: %v", err)
	}

	if info.CommentCount != 2 {
		t.Fatalf("CommentCount = %d", info.CommentCount)
	}
	if info.ContentHash == "" {
		t.Fatal("ContentHash is empty")
	}
}

func TestValidatePayloadRejectsMissingComments(t *testing.T) {
	_, err := ValidatePayload([]byte(`{"count":0}`))
	if err == nil {
		t.Fatal("ValidatePayload returned nil error")
	}
}

func TestValidatePayloadRejectsInvalidJSON(t *testing.T) {
	_, err := ValidatePayload([]byte(`{`))
	if err == nil {
		t.Fatal("ValidatePayload returned nil error")
	}
}

func TestGzipPayloadRoundTrip(t *testing.T) {
	body := []byte(`{"count":1,"comments":[{"cid":1,"p":"0.00,1,16777215,abc","m":"one"}]}`)

	compressed, err := GzipPayload(body)
	if err != nil {
		t.Fatalf("GzipPayload returned error: %v", err)
	}
	if bytes.Equal(compressed, body) {
		t.Fatal("compressed payload equals original payload")
	}

	restored, err := GunzipPayload(compressed)
	if err != nil {
		t.Fatalf("GunzipPayload returned error: %v", err)
	}
	if !bytes.Equal(restored, body) {
		t.Fatalf("round trip mismatch: %q", restored)
	}
}
