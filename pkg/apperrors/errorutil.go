package apperrors

import (
	"errors"

	"connectrpc.com/connect/v2"
	"github.com/pitabwire/frame/v3/data"
)

func CleanErr(err error) *connect.Error {
	if err == nil {
		return nil
	}

	var cerr *connect.Error
	ok := errors.As(err, &cerr)
	if ok {
		return cerr
	}

	return data.ErrorConvertToAPI(err)
}
