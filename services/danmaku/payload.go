package danmaku

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
)

type PayloadInfo struct {
	DanmakuCount int
	ContentHash  string
}

type payloadEnvelope struct {
	Count    int               `json:"count"`
	Comments []json.RawMessage `json:"comments"`
}

func ValidatePayload(payload []byte) (PayloadInfo, error) {
	var envelope payloadEnvelope
	if err := json.Unmarshal(payload, &envelope); err != nil {
		return PayloadInfo{}, err
	}
	if envelope.Comments == nil {
		return PayloadInfo{}, errors.New("danmaku payload missing comments array")
	}

	hash := sha256.Sum256(payload)
	return PayloadInfo{
		DanmakuCount: len(envelope.Comments),
		ContentHash:  hex.EncodeToString(hash[:]),
	}, nil
}

func GzipPayload(payload []byte) ([]byte, error) {
	var out bytes.Buffer
	writer := gzip.NewWriter(&out)
	if _, err := writer.Write(payload); err != nil {
		_ = writer.Close()
		return nil, err
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

func GunzipPayload(payload []byte) ([]byte, error) {
	reader, err := gzip.NewReader(bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	return io.ReadAll(reader)
}
