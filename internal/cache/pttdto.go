package cache

import (
	"transbroker/internal/domain"
)

type ICache interface {
	SetTrans(key TransIdent, value domain.PreparedData)
	GetTrans(key TransIdent) (domain.PreparedData, bool)
	RemoveTrans(key TransIdent)
}

type TransIdent struct {
	Language string
	TextHash string
}
