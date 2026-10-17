//go:build !windows

package keyring

import "github.com/axiomzero-qed/WxMori/internal/fault"

func protect([]byte) (string, error)   { return "", unsupported() }
func unprotect(string) ([]byte, error) { return nil, unsupported() }
func unsupported() error {
	return fault.New("WINDOWS_KEY_FILE", "密钥文件需要原 Windows 用户 / Windows user-bound key file", "Use the Windows user that captured it, or configure a separate local master-key file on this platform.")
}
