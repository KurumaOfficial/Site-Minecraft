// Автор: Kuruma
package domain

type AppError struct {
	Status  int
	Message string
}

func (e *AppError) Error() string {
	return e.Message
}

func NewBadRequest(message string) *AppError {
	return &AppError{Status: 400, Message: message}
}

func NewUnauthorized(message string) *AppError {
	return &AppError{Status: 401, Message: message}
}

func NewForbidden(message string) *AppError {
	return &AppError{Status: 403, Message: message}
}

func NewNotFound(message string) *AppError {
	return &AppError{Status: 404, Message: message}
}

func NewConflict(message string) *AppError {
	return &AppError{Status: 409, Message: message}
}

func NewInternal(message string) *AppError {
	return &AppError{Status: 500, Message: message}
}
