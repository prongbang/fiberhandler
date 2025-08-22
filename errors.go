package fiberhandler

import "github.com/prongbang/goerror"

type DataInvalidError struct {
	goerror.Body
}

// Error implements error.
func (c *DataInvalidError) Error() string {
	return c.Message
}

func NewDataInvalidError() error {
	return &DataInvalidError{
		Body: goerror.Body{
			Code:    "CLE029",
			Message: "Invalid data provided",
		},
	}
}
