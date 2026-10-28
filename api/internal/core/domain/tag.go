package domain

import "time"

type Tag struct {
	ID        string    `json:"id"`
	AccountID string    `json:"account_id"`
	Name      string    `json:"name"`
	Color     string    `json:"color"`
	CreatedAt time.Time `json:"created_at"`
}

var ErrTagNotFound = &AppError{Code: "TAG_NOT_FOUND", Message: "tag not found"}
var ErrTagExists = &AppError{Code: "TAG_EXISTS", Message: "tag with this name already exists"}
