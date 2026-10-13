package business

import (
	"connectrpc.com/connect/v2"
)

var (
	ErrorUnspecifiedID      = connect.Errorf(connect.CodeInvalidArgument, "no id was supplied")
	ErrorEmptyValueSupplied = connect.Errorf(connect.CodeInvalidArgument, "empty value supplied")
	ErrorItemExist          = connect.Errorf(connect.CodeAlreadyExists, "specified item already exists")
	ErrorItemDoesNotExist   = connect.Errorf(connect.CodeNotFound, "specified item does not exist")
	ErrorInitializationFail = connect.Errorf(connect.CodeInternal, "internal configuration is invalid")
)
