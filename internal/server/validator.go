package server

import (
	"github.com/go-playground/validator/v10"
)

// Validator は Echo の echo.Validator インタフェース実装。
// go-playground/validator を使って struct タグベースの検証を行う。
type Validator struct {
	v *validator.Validate
}

func NewValidator() *Validator {
	return &Validator{v: validator.New()}
}

func (val *Validator) Validate(i interface{}) error {
	return val.v.Struct(i)
}
