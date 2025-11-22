package handlers

import (
	"github.com/mark47B/be-internship/internal/domain/usecase"
	"github.com/mark47B/be-internship/internal/infra/transport/rest/gen"
)

type Handlers struct {
	gen.Unimplemented
	service usecase.Service
}

func NewHandlers(service usecase.Service) gen.ServerInterface {
	return &Handlers{
		service: service,
	}
}
