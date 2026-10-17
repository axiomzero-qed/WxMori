package keyring

import (
	"encoding/json"
	"io"
	"os"
)

type CapturedInfo struct {
	AccountHash      string `json:"account_binding"`
	CapturedAt       string `json:"captured_at"`
	DatabaseKeyCount int    `json:"database_key_count"`
	Protected        bool   `json:"protected"`
}

// InspectCaptured exposes only metadata from a validated protected keyring.
func InspectCaptured(path string) (CapturedInfo, error) {
	f, err := os.Open(path)
	if err != nil {
		return CapturedInfo{}, err
	}
	defer f.Close()
	b, err := io.ReadAll(io.LimitReader(f, (1<<20)+1))
	if err != nil {
		return CapturedInfo{}, err
	}
	defer clear(b)
	if len(b) > 1<<20 {
		return CapturedInfo{}, invalid()
	}
	var envelope Envelope
	if json.Unmarshal(b, &envelope) != nil || envelope.Format != "wxmori.dpapi.v1" {
		return CapturedInfo{}, invalid()
	}
	plain, err := unprotect(envelope.Protected)
	if err != nil {
		return CapturedInfo{}, err
	}
	defer clear(plain)
	var file File
	if json.Unmarshal(plain, &file) != nil {
		return CapturedInfo{}, invalid()
	}
	material, err := Decode(plain, file.Account)
	if err != nil {
		return CapturedInfo{}, err
	}
	defer material.Clear()
	return CapturedInfo{AccountHash: file.Account, CapturedAt: file.ObtainedAt, DatabaseKeyCount: len(material.BySalt), Protected: true}, nil
}
