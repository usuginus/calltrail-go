package grpc

import (
	"context"
	stdstrings "strings"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type Server struct {
	fooUsecase FooUsecase
}

type FooUsecase interface {
	GetFoo(context.Context, string) (*Foo, error)
}

type FooRepository interface {
	FindFoo(context.Context, string) (*Foo, error)
}

type fooUsecase struct {
	repos *Repositories
}

type Repositories struct {
	Foo *fooRepository
}

var _ FooUsecase = (*fooUsecase)(nil)

type fooRepository struct{}

type Foo struct{}

type pb struct{}

func (pb) GetFooRequest() {}

func (pb) GetFooResponse() {}

func (s *Server) GetFoo(ctx context.Context, req *pb.GetFooRequest) (*pb.GetFooResponse, error) {
	foo, err := s.fooUsecase.GetFoo(ctx, req.ID)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return convertFoo(foo), nil
}

func convertFoo(*Foo) *pb.GetFooResponse {
	return nil
}

func (f fooUsecase) GetFoo(ctx context.Context, id string) (*Foo, error) {
	id = stdstrings.TrimSpace(id)
	return f.repos.Foo.FindFoo(ctx, id)
}

func (f *fooRepository) FindFoo(context.Context, string) (*Foo, error) {
	return &Foo{}, nil
}
