package errorshelper

type (
	ValidationErrorType string
)

const (
	DuplicateValue   ValidationErrorType = "DuplicateValue"
	NoAccess         ValidationErrorType = "NoAccess"
	NoPermission     ValidationErrorType = "NoPermission"
	NotFound         ValidationErrorType = "NotFound"
	WrongTokenType   ValidationErrorType = "WrongTokenType"
	AlreadyConfirmed ValidationErrorType = "AlreadyConfirmed"
	InvalidReference ValidationErrorType = "InvalidReference"
	InvalidState     ValidationErrorType = "InvalidState"
	Unknown                              = "unknown"
	InvalidParameter ValidationErrorType = "InvalidParameter"

	InternalServerMsg string = "Sorry, something went wrong on our side. Please try again later."
)
