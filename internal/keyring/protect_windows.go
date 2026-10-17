//go:build windows

package keyring

import (
	"encoding/base64"
	"github.com/axiomzero-qed/WxMori/internal/fault"
	"golang.org/x/sys/windows"
	"unsafe"
)

func protect(data []byte) (string, error) {
	if len(data) == 0 {
		return "", invalid()
	}
	in := windows.DataBlob{Size: uint32(len(data)), Data: &data[0]}
	var out windows.DataBlob
	if err := windows.CryptProtectData(&in, nil, nil, 0, nil, windows.CRYPTPROTECT_UI_FORBIDDEN, &out); err != nil {
		return "", fault.New("KEY_PROTECTION_FAILED", "无法保护密钥文件 / Could not protect key file", "Windows DPAPI must be available under your interactive user account.")
	}
	defer windows.LocalFree(windows.Handle(unsafe.Pointer(out.Data)))
	bytes := unsafe.Slice(out.Data, int(out.Size))
	defer clear(bytes)
	return base64.StdEncoding.EncodeToString(bytes), nil
}
func unprotect(text string) ([]byte, error) {
	data, err := base64.StdEncoding.DecodeString(text)
	if err != nil || len(data) == 0 || len(data) > 1<<20 {
		return nil, invalid()
	}
	defer clear(data)
	in := windows.DataBlob{Size: uint32(len(data)), Data: &data[0]}
	var out windows.DataBlob
	if err = windows.CryptUnprotectData(&in, nil, nil, 0, nil, windows.CRYPTPROTECT_UI_FORBIDDEN, &out); err != nil {
		return nil, fault.New("KEY_UNPROTECT_FAILED", "无法解开本机密钥文件 / Could not unprotect local keys", "Use the Windows user and computer that captured these keys, or capture them again.")
	}
	defer windows.LocalFree(windows.Handle(unsafe.Pointer(out.Data)))
	bytes := unsafe.Slice(out.Data, int(out.Size))
	defer clear(bytes)
	return append([]byte(nil), bytes...), nil
}
