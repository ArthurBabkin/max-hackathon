package api

import (
	"errors"
	"net/http"

	"github.com/ArthurBabkin/max-hackathon/packages/db/store"
)

// Error — ответ об ошибке по контракту: {"error": {"code", "message"}}.
// message пишется по-русски и показывается пользователю как есть.
type Error struct {
	Status  int
	Code    string
	Message string
}

func (e *Error) Error() string { return e.Code + ": " + e.Message }

func badRequest(msg string) *Error { return &Error{http.StatusBadRequest, "BAD_REQUEST", msg} }
func forbidden(msg string) *Error  { return &Error{http.StatusForbidden, "FORBIDDEN", msg} }
func notFound(msg string) *Error   { return &Error{http.StatusNotFound, "NOT_FOUND", msg} }
func conflict(msg string) *Error   { return &Error{http.StatusConflict, "CONFLICT", msg} }

var (
	errUnauthorized = &Error{http.StatusUnauthorized, "UNAUTHORIZED",
		"Сессия истекла. Откройте приложение заново."}
	errInternal = &Error{http.StatusInternalServerError, "INTERNAL",
		"Что-то пошло не так. Попробуйте ещё раз."}
	errNotImplemented = &Error{http.StatusNotImplemented, "NOT_IMPLEMENTED",
		"Эта функция пока не готова."}
)

// asError приводит ошибку сценария к ответу. Неизвестные ошибки — 500 без
// подробностей: текст ошибки базы пользователю не показывается.
func asError(err error) (*Error, bool) {
	var e *Error
	if errors.As(err, &e) {
		return e, true
	}
	if errors.Is(err, store.ErrNotFound) {
		return notFound("Не найдено."), true
	}
	return errInternal, false
}

type errorBody struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

func writeError(w http.ResponseWriter, e *Error) {
	var body errorBody
	body.Error.Code, body.Error.Message = e.Code, e.Message
	writeJSON(w, e.Status, body)
}
