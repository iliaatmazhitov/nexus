package httpx

import "context"

type requestLog struct {
	ErrorCode string
	Error     error
}

type requestLogKey struct{}

func SetRequestError(ctx context.Context, code string, err error) {
	state, ok := ctx.Value(requestLogKey{}).(*requestLog)

	if !ok || state == nil {
		return
	}

	state.ErrorCode = code
	state.Error = err
}
