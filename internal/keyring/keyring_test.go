package keyring

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestOldMasterFileAndAccountBinding(t *testing.T) {
	data := []byte(`{"master_key":"` + strings.Repeat("01", 32) + `"}`)
	m, e := Decode(data, "account")
	if e != nil || len(m.Master) != 32 {
		t.Fatalf("old master file failed %v", e)
	}
	m.Clear()
	file := File{Version: 1, Account: "other", DatabaseKeys: map[string]string{strings.Repeat("02", 16): strings.Repeat("03", 32)}}
	data, _ = json.Marshal(file)
	if _, e = Decode(data, "account"); e == nil || !strings.Contains(e.Error(), "KEY_ACCOUNT_MISMATCH") {
		t.Fatal("cross-account file accepted")
	}
	file.Account = "account"
	data, _ = json.Marshal(file)
	m, e = Decode(data, "account")
	if e != nil || len(m.BySalt) != 1 {
		t.Fatalf("database keys failed %v", e)
	}
	m.Clear()
}

func TestMalformedKeyFiles(t *testing.T) {
	for _, raw := range []string{`{}`, `{"version":99,"master_key":"` + strings.Repeat("01", 32) + `"}`, `{"database_keys":{"00":"abc"}}`, `{"master_key":"wrong"}`} {
		if _, e := Decode([]byte(raw), "account"); e == nil {
			t.Fatal("malformed key file accepted")
		}
	}
}
