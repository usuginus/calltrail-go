package grpc

import "context"

type CreateOrderRequest struct{}

type CreateOrderResponse struct{}

type debugService struct{}

type userService struct{}

func (s *debugService) CreateOrder(ctx context.Context, req *CreateOrderRequest) (*CreateOrderResponse, error) {
	return &CreateOrderResponse{}, nil
}

func (s *userService) CreateOrder(ctx context.Context, req *CreateOrderRequest) (*CreateOrderResponse, error) {
	return &CreateOrderResponse{}, nil
}
