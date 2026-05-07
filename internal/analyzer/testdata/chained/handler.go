package grpc

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type Server struct{}

type pb struct{}

func (pb) GetFooRequest() {}

func (pb) GetFooResponse() {}

type Foo struct{}

type fooUsecase struct {
	service *fooService
}

type fooService struct{}

func NewFooUsecase() *fooUsecase {
	return &fooUsecase{service: NewFooService()}
}

func NewFooService() *fooService {
	return &fooService{}
}

func (s *Server) GetFoo(ctx context.Context, req *pb.GetFooRequest) (*pb.GetFooResponse, error) {
	uc := NewFooUsecase()
	if _, err := uc.GetFoo(ctx); err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	if _, err := NewFooUsecase().GetFoo(ctx); err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &pb.GetFooResponse{}, nil
}

func (u *fooUsecase) GetFoo(ctx context.Context) (*Foo, error) {
	return u.load(ctx)
}

func (u *fooUsecase) load(ctx context.Context) (*Foo, error) {
	return u.service.FetchFoo(ctx)
}

func (s *fooService) FetchFoo(context.Context) (*Foo, error) {
	return &Foo{}, nil
}
