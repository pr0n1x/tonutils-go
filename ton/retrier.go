package ton

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/xssnick/tonutils-go/liteclient"
	"github.com/xssnick/tonutils-go/tl"
)

type retryClient struct {
	maxRetries int
	original   LiteClient
}

func (w *retryClient) QueryLiteserver(ctx context.Context, payload tl.Serializable, result tl.Serializable) (err error) {
	tries := 0
	for {
		if err != nil && w.maxRetries > 0 && tries >= w.maxRetries {
			return err
		}
		err = w.original.QueryLiteserver(ctx, payload, result)
		tries++

		if err != nil {
			if errors.Is(err, liteclient.ErrADNLReqTimeout) {
				// try next node
				ctx, err = w.original.StickyContextNextNode(ctx)
				if err != nil {
					return fmt.Errorf("timeout error received, but failed to try with next node, "+
						"looks like all active nodes was already tried, original error: %w", err)
				}
			}
			continue
		}

		if tmp, ok := result.(*tl.Serializable); ok && tmp != nil {
			// list of maybe all errors: https://github.com/ton-blockchain/ton/blob/master/common/errorcode.h#L23
			if lsErr, ok := (*tmp).(LSError); ok && (lsErr.Code == 651 ||
				lsErr.Code == 652 || // ErrorCode::timeout
				lsErr.Code == -400 || // what is -400 code
				lsErr.Code == -503 || // -503 is also timeout (found out practically)
				// what about
				// - ErrorCode::notready(651, "block is not applied")
				// - ErrorCode::cancelled(653)
				(lsErr.Code == 0 && strings.Contains(lsErr.Text, "Failed to get account state"))) {
				// try next node
				if ctx, err = w.original.StickyContextNextNode(ctx); err != nil {
					// no more nodes left, return as it is
					return nil
				}
				err = lsErr
				continue
			}
		}
		return nil
	}
}

func (w *retryClient) StickyContext(ctx context.Context) context.Context {
	return w.original.StickyContext(ctx)
}

func (w *retryClient) StickyNodeID(ctx context.Context) uint32 {
	return w.original.StickyNodeID(ctx)
}

func (w *retryClient) StickyContextNextNode(ctx context.Context) (context.Context, error) {
	return w.original.StickyContextNextNode(ctx)
}
