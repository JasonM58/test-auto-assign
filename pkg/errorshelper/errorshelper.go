package errorshelper

import (
	"net/http"
	"strings"
)

type (
	ExpiredToken struct{}
)

func (e ExpiredToken) Error() string {
	return "token expired"
}

type (
	ValidationErrors struct {
		errorsList []ValidationError
	}

	ValidationError struct {
		Field   string              `json:"field"`
		Message string              `json:"message"`
		Type    ValidationErrorType `type:"error_type" json:"type"`
	}
)

func NewValidationError(errorsList []ValidationError) *ValidationErrors {
	return &ValidationErrors{errorsList: errorsList}
}

func (v *ValidationErrors) Error() string {
	errorsMessagesList := make([]string, 0)
	for _, errorMessage := range v.errorsList {
		errorsMessagesList = append(errorsMessagesList, errorMessage.Message)
	}

	return strings.Join(errorsMessagesList, ", ")
}

func (v *ValidationErrors) Message() []ValidationError {
	if v.ResponseCode() == http.StatusInternalServerError {
		return []ValidationError{
			{
				Field:   Unknown,
				Message: InternalServerMsg,
				Type:    Unknown,
			},
		}
	} else {
		return v.errorsList
	}
}

func (v *ValidationErrors) ResponseCode() int {
	var status int

	switch v.errorsList[0].Type {
	case NoPermission:
		status = http.StatusForbidden
	case NotFound:
		status = http.StatusNotFound
	case Unknown:
		status = http.StatusInternalServerError
	default:
		status = http.StatusUnprocessableEntity
	}

	return status
}
