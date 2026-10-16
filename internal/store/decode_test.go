package store

import (
	"strings"
	"testing"
)

func TestStrictSenderPrefix(t *testing.T) {
	for _, tc := range []struct{ text, id, want string }{
		{"wxid_alice:\nfirst\nsecond", "wxid_alice", "first\nsecond"},
		{"wxid_bob:\nbody", "wxid_alice", "wxid_bob:\nbody"},
		{"Address:\nline two", "", "Address:\nline two"},
		{"Meeting time: 10:00\nnext", "wxid_alice", "Meeting time: 10:00\nnext"},
	} {
		got, _, _ := StripPrefix(tc.text, tc.id)
		if got != tc.want {
			t.Fatalf("got %q want %q", got, tc.want)
		}
	}
}
func TestCompoundTypesAndMalformedXML(t *testing.T) {
	kind, text := Readable(`<msg><appmsg><title><![CDATA[A & B]]></title></appmsg></msg>`, (2001<<32)|49)
	if kind != "app" || text != "A & B" {
		t.Fatalf("%s %s", kind, text)
	}
	_, text = Readable(`<msg><appmsg>broken`, 49)
	if strings.Contains(text, "<msg>") {
		t.Fatal("raw XML leaked")
	}
	_, text = Readable(`<sysmsg><revokemsg><content>recalled &amp; hidden</content></revokemsg></sysmsg>`, 10000)
	if text != "recalled & hidden" {
		t.Fatal("system XML not decoded")
	}
}

func TestXMLBuilderGrowth(t *testing.T) {
	raw := `<msg><appmsg><title>hello</title><nested><more><one><two><three>body</three></two></one></more></nested></appmsg></msg>`
	kind, text := Readable(raw, 49)
	if kind != "app" || text != "hello" {
		t.Fatalf("unexpected result: %s %s", kind, text)
	}
	raw = strings.Repeat("<a>", 256) + "body" + strings.Repeat("</a>", 256)
	if _, text = Readable(raw, 49); strings.Contains(text, "<a>") {
		t.Fatal("deep XML should remain a placeholder")
	}
}
