package httpdelivery

import (
	"encoding/json"
	"errors"
	"net/http"

	"boilerplate-skeletoncode/internal/domain"
	"boilerplate-skeletoncode/internal/usecase"
)

type userHandler struct {
	userUsecase UserUsecase
}

type createUserRequest struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

func newUserHandler(userUsecase UserUsecase) userHandler {
	return userHandler{
		userUsecase: userUsecase,
	}
}

func (h userHandler) create(w http.ResponseWriter, r *http.Request) {
	var request createUserRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	user, err := h.userUsecase.CreateUser(r.Context(), usecase.CreateUserInput{
		Name:  request.Name,
		Email: request.Email,
	})
	if err != nil {
		switch {
		case errors.Is(err, usecase.ErrInvalidUserName), errors.Is(err, usecase.ErrInvalidUserEmail):
			writeError(w, http.StatusBadRequest, err.Error())
		case errors.Is(err, domain.ErrUserAlreadyExists):
			writeError(w, http.StatusConflict, err.Error())
		default:
			writeError(w, http.StatusInternalServerError, "internal server error")
		}

		return
	}

	writeJSON(w, http.StatusCreated, user)
}

func (h userHandler) list(w http.ResponseWriter, r *http.Request) {
	users, err := h.userUsecase.ListUsers(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	writeJSON(w, http.StatusOK, users)
}

func (h userHandler) getByID(w http.ResponseWriter, r *http.Request) {
	user, err := h.userUsecase.GetUser(r.Context(), r.PathValue("id"))
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrUserNotFound):
			writeError(w, http.StatusNotFound, err.Error())
		default:
			writeError(w, http.StatusInternalServerError, "internal server error")
		}

		return
	}

	writeJSON(w, http.StatusOK, user)
}
