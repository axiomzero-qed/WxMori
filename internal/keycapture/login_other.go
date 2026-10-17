//go:build !windows || !amd64

package keycapture

import "context"

func (*Manager) watchLogin(context.Context, []*process, map[[16]byte][32]byte) (bool, error) {
	return false, nil
}
