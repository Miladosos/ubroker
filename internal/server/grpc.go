package server

import (
	"context"
	"io"

	"github.com/golang/protobuf/ptypes/empty"
	"github.com/pkg/errors"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/miladosos/ubroker/pkg/ubroker"
)

type grpcServicer struct {
	broker   ubroker.Broker
	delivery <-chan *ubroker.Delivery
}

func NewGRPC(broker ubroker.Broker) ubroker.BrokerServer {
	return &grpcServicer{
		broker: broker,
	}
}

func (s *grpcServicer) Fetch(stream ubroker.Broker_FetchServer) error {
	var err error
	ctx := stream.Context()

	select {
	case <-ctx.Done():
		return s.handleError(ctx.Err())
	default:
		s.delivery, err = s.broker.Delivery(ctx)
		if err != nil {
			return s.handleError(err)
		}
	}

	for {
		_, err := stream.Recv()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			continue
		}

		select {
		case <-ctx.Done():
			return s.handleError(err)
		case msg := <- s.delivery:
			if msg == nil {
				return status.Error(codes.Unavailable, "Empty Queue")
			}
			if err := stream.Send(msg); err != nil {
				return s.handleError(err)
			}
		}
	}
}

func (s *grpcServicer) Acknowledge(ctx context.Context, request *ubroker.AcknowledgeRequest) (*empty.Empty, error) {
	err := s.broker.Acknowledge(ctx, request.Id)
	return &empty.Empty{}, s.handleError(err)
}

func (s *grpcServicer) ReQueue(ctx context.Context, request *ubroker.ReQueueRequest) (*empty.Empty, error) {
	err := s.broker.ReQueue(ctx, request.Id)
	return &empty.Empty{}, s.handleError(err)
}

func (s *grpcServicer) Publish(ctx context.Context, request *ubroker.Message) (*empty.Empty, error) {
	err := s.broker.Publish(ctx, request)
	return &empty.Empty{}, s.handleError(err)
}

func (s *grpcServicer) handleError(err error) error {
	return status.Error(s.errorToStatusCode(err), s.errorToMessage(err))
}

func (s *grpcServicer) errorToStatusCode(err error) codes.Code {
	if err == nil {
		return codes.OK
	}
	for {
		result, ok := s.tryErrorToStatusCode(err)
		if ok {
			return result
		}

		cause := errors.Cause(err)
		if cause == err {
			return result
		}

		err = cause
	}
}

func (s *grpcServicer) tryErrorToStatusCode(err error) (codes.Code, bool) {
	switch err {
	case ubroker.ErrClosed:
		return codes.Unavailable, true

	case ubroker.ErrUnimplemented:
		return codes.Unimplemented, true

	case ubroker.ErrInvalidID, errInvalidArgument:
		return codes.InvalidArgument, true

	case context.Canceled, context.DeadlineExceeded:
		return codes.DeadlineExceeded, true

	default:
		return codes.Internal, false
	}
}

func (s *grpcServicer) errorToMessage(err error) string {
	if err == nil {
		return "OK"
	}
	for {
		result, ok := s.tryErrorToMessage(err)
		if ok {
			return result
		}

		cause := errors.Cause(err)
		if cause == err {
			return result
		}

		err = cause
	}
}

func (s *grpcServicer) tryErrorToMessage(err error) (string, bool) {
	switch err {
	case ubroker.ErrClosed:
		return "Closed", true

	case ubroker.ErrUnimplemented:
		return "Unimplemented", true

	case ubroker.ErrInvalidID:
		return "InvalidID", true

	case context.Canceled, context.DeadlineExceeded:
		return "DeadlineExceeded", true

	default:
		return "InternalError", false
	}
}
