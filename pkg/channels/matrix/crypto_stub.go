//go:build !goolm

package matrix

import (
	"context"
	"fmt"
)

func (c *MatrixChannel) initCrypto(_ context.Context) error {
	return fmt.Errorf("matrix encrypted-room support is not compiled in; rebuild with -tags goolm to enable pure-Go Matrix E2EE")
}
