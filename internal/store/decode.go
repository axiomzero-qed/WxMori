package store

import (
	"bytes"
	"encoding/xml"
	"io"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/klauspost/compress/zstd"
)

var prefixPattern = regexp.MustCompile(`^([A-Za-z0-9_.@-]{1,128}):\r?\n`)

func DecodeBlob(raw []byte, marker int64) (string, error) {
	if len(raw) > 4<<20 {
		return "", io.ErrShortBuffer
	}
	if marker == 4 || bytes.HasPrefix(raw, []byte{0x28, 0xb5, 0x2f, 0xfd}) {
		dec, err := zstd.NewReader(nil, zstd.WithDecoderMaxMemory(16<<20), zstd.WithDecoderMaxWindow(8<<20), zstd.WithDecoderConcurrency(1))
		if err != nil {
			return "", err
		}
		defer dec.Close()
		out, err := dec.DecodeAll(raw, make([]byte, 0, 1024))
		if err != nil {
			return "", err
		}
		if len(out) > 4<<20 {
			return "", io.ErrShortBuffer
		}
		raw = out
	}
	if !utf8.Valid(raw) {
		return "", io.ErrUnexpectedEOF
	}
	return strings.TrimRight(string(raw), "\x00"), nil
}

func StripPrefix(text, sender string) (string, string, bool) {
	m := prefixPattern.FindStringSubmatchIndex(text)
	if m == nil {
		return text, sender, false
	}
	name := text[m[2]:m[3]]
	if sender == "" && strings.HasPrefix(name, "wxid_") {
		sender = name
	}
	if sender != "" && sender == name {
		return text[m[1]:], sender, false
	}
	return text, sender, sender != ""
}

func xmlFields(raw string) map[string]string {
	out := map[string]string{}
	dec := xml.NewDecoder(strings.NewReader(raw))
	var stack []string
	var text []*strings.Builder
	for {
		tok, err := dec.Token()
		if err != nil {
			break
		}
		switch t := tok.(type) {
		case xml.StartElement:
			if len(stack) >= 128 {
				return out
			}
			stack = append(stack, t.Name.Local)
			text = append(text, &strings.Builder{})
		case xml.CharData:
			if len(text) > 0 {
				for i := range text {
					text[i].Write(t)
				}
			}
		case xml.EndElement:
			if len(stack) > 0 {
				n := len(stack) - 1
				v := strings.TrimSpace(text[n].String())
				if _, ok := out[stack[n]]; !ok && v != "" {
					out[stack[n]] = v
				}
				stack = stack[:n]
				text = text[:n]
			}
		}
	}
	return out
}

func Readable(raw string, localType int64) (string, string) {
	t := uint32(localType)
	switch t {
	case 1:
		return "text", raw
	case 3:
		return "image", "[图片 / Image]"
	case 34:
		return "voice", "[语音 / Voice]"
	case 43, 62:
		return "video", "[视频 / Video]"
	case 47:
		return "emoji", "[表情 / Sticker]"
	case 48:
		return "location", "[位置 / Location]"
	case 50:
		return "call", "[通话 / Call]"
	case 49:
		f := xmlFields(raw)
		if f["title"] != "" {
			return "app", f["title"]
		}
		return "app", "[分享、文件或转账 / Shared item, file or transfer]"
	case 10000, 10001, 10002:
		if strings.HasPrefix(strings.TrimSpace(raw), "<") {
			f := xmlFields(raw)
			if f["content"] != "" {
				return "system", f["content"]
			}
			if f["replacemsg"] != "" {
				return "system", f["replacemsg"]
			}
			return "system", "[系统消息 / System message]"
		}
		return "system", raw
	default:
		return "other", "[暂不支持的消息 / Unsupported message]"
	}
}
