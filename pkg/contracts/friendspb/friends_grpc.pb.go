package friendspb

import (
	context "context"

	grpc "google.golang.org/grpc"
	codes "google.golang.org/grpc/codes"
	status "google.golang.org/grpc/status"
)

const (
	FriendsService_SendFriendRequest_FullMethodName         = "/friends.FriendsService/SendFriendRequest"
	FriendsService_AcceptFriendRequest_FullMethodName       = "/friends.FriendsService/AcceptFriendRequest"
	FriendsService_DeclineFriendRequest_FullMethodName      = "/friends.FriendsService/DeclineFriendRequest"
	FriendsService_RemoveFriend_FullMethodName              = "/friends.FriendsService/RemoveFriend"
	FriendsService_ListFriends_FullMethodName               = "/friends.FriendsService/ListFriends"
	FriendsService_ListPendingFriendRequests_FullMethodName = "/friends.FriendsService/ListPendingFriendRequests"
)

type FriendsServiceClient interface {
	SendFriendRequest(ctx context.Context, in *SendFriendRequestRequest, opts ...grpc.CallOption) (*FriendRequestResponse, error)
	AcceptFriendRequest(ctx context.Context, in *AcceptFriendRequestRequest, opts ...grpc.CallOption) (*ActionResponse, error)
	DeclineFriendRequest(ctx context.Context, in *DeclineFriendRequestRequest, opts ...grpc.CallOption) (*ActionResponse, error)
	RemoveFriend(ctx context.Context, in *RemoveFriendRequest, opts ...grpc.CallOption) (*ActionResponse, error)
	ListFriends(ctx context.Context, in *ListFriendsRequest, opts ...grpc.CallOption) (*ListFriendsResponse, error)
	ListPendingFriendRequests(ctx context.Context, in *ListPendingFriendRequestsRequest, opts ...grpc.CallOption) (*ListPendingFriendRequestsResponse, error)
}

type friendsServiceClient struct {
	cc grpc.ClientConnInterface
}

func NewFriendsServiceClient(cc grpc.ClientConnInterface) FriendsServiceClient {
	return &friendsServiceClient{cc}
}

func (c *friendsServiceClient) SendFriendRequest(ctx context.Context, in *SendFriendRequestRequest, opts ...grpc.CallOption) (*FriendRequestResponse, error) {
	out := new(FriendRequestResponse)
	err := c.cc.Invoke(ctx, FriendsService_SendFriendRequest_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *friendsServiceClient) AcceptFriendRequest(ctx context.Context, in *AcceptFriendRequestRequest, opts ...grpc.CallOption) (*ActionResponse, error) {
	out := new(ActionResponse)
	err := c.cc.Invoke(ctx, FriendsService_AcceptFriendRequest_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *friendsServiceClient) DeclineFriendRequest(ctx context.Context, in *DeclineFriendRequestRequest, opts ...grpc.CallOption) (*ActionResponse, error) {
	out := new(ActionResponse)
	err := c.cc.Invoke(ctx, FriendsService_DeclineFriendRequest_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *friendsServiceClient) RemoveFriend(ctx context.Context, in *RemoveFriendRequest, opts ...grpc.CallOption) (*ActionResponse, error) {
	out := new(ActionResponse)
	err := c.cc.Invoke(ctx, FriendsService_RemoveFriend_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *friendsServiceClient) ListFriends(ctx context.Context, in *ListFriendsRequest, opts ...grpc.CallOption) (*ListFriendsResponse, error) {
	out := new(ListFriendsResponse)
	err := c.cc.Invoke(ctx, FriendsService_ListFriends_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *friendsServiceClient) ListPendingFriendRequests(ctx context.Context, in *ListPendingFriendRequestsRequest, opts ...grpc.CallOption) (*ListPendingFriendRequestsResponse, error) {
	out := new(ListPendingFriendRequestsResponse)
	err := c.cc.Invoke(ctx, FriendsService_ListPendingFriendRequests_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

type FriendsServiceServer interface {
	SendFriendRequest(context.Context, *SendFriendRequestRequest) (*FriendRequestResponse, error)
	AcceptFriendRequest(context.Context, *AcceptFriendRequestRequest) (*ActionResponse, error)
	DeclineFriendRequest(context.Context, *DeclineFriendRequestRequest) (*ActionResponse, error)
	RemoveFriend(context.Context, *RemoveFriendRequest) (*ActionResponse, error)
	ListFriends(context.Context, *ListFriendsRequest) (*ListFriendsResponse, error)
	ListPendingFriendRequests(context.Context, *ListPendingFriendRequestsRequest) (*ListPendingFriendRequestsResponse, error)
	mustEmbedUnimplementedFriendsServiceServer()
}

type UnimplementedFriendsServiceServer struct{}

func (UnimplementedFriendsServiceServer) SendFriendRequest(context.Context, *SendFriendRequestRequest) (*FriendRequestResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method SendFriendRequest not implemented")
}

func (UnimplementedFriendsServiceServer) AcceptFriendRequest(context.Context, *AcceptFriendRequestRequest) (*ActionResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method AcceptFriendRequest not implemented")
}

func (UnimplementedFriendsServiceServer) DeclineFriendRequest(context.Context, *DeclineFriendRequestRequest) (*ActionResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method DeclineFriendRequest not implemented")
}

func (UnimplementedFriendsServiceServer) RemoveFriend(context.Context, *RemoveFriendRequest) (*ActionResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method RemoveFriend not implemented")
}

func (UnimplementedFriendsServiceServer) ListFriends(context.Context, *ListFriendsRequest) (*ListFriendsResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method ListFriends not implemented")
}

func (UnimplementedFriendsServiceServer) ListPendingFriendRequests(context.Context, *ListPendingFriendRequestsRequest) (*ListPendingFriendRequestsResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method ListPendingFriendRequests not implemented")
}

func (UnimplementedFriendsServiceServer) mustEmbedUnimplementedFriendsServiceServer() {}

type UnsafeFriendsServiceServer interface {
	mustEmbedUnimplementedFriendsServiceServer()
}

func RegisterFriendsServiceServer(s grpc.ServiceRegistrar, srv FriendsServiceServer) {
	s.RegisterService(&FriendsService_ServiceDesc, srv)
}

func _FriendsService_SendFriendRequest_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(SendFriendRequestRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(FriendsServiceServer).SendFriendRequest(ctx, in)
	}
	info := &grpc.UnaryServerInfo{Server: srv, FullMethod: FriendsService_SendFriendRequest_FullMethodName}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(FriendsServiceServer).SendFriendRequest(ctx, req.(*SendFriendRequestRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _FriendsService_AcceptFriendRequest_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(AcceptFriendRequestRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(FriendsServiceServer).AcceptFriendRequest(ctx, in)
	}
	info := &grpc.UnaryServerInfo{Server: srv, FullMethod: FriendsService_AcceptFriendRequest_FullMethodName}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(FriendsServiceServer).AcceptFriendRequest(ctx, req.(*AcceptFriendRequestRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _FriendsService_DeclineFriendRequest_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(DeclineFriendRequestRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(FriendsServiceServer).DeclineFriendRequest(ctx, in)
	}
	info := &grpc.UnaryServerInfo{Server: srv, FullMethod: FriendsService_DeclineFriendRequest_FullMethodName}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(FriendsServiceServer).DeclineFriendRequest(ctx, req.(*DeclineFriendRequestRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _FriendsService_RemoveFriend_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(RemoveFriendRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(FriendsServiceServer).RemoveFriend(ctx, in)
	}
	info := &grpc.UnaryServerInfo{Server: srv, FullMethod: FriendsService_RemoveFriend_FullMethodName}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(FriendsServiceServer).RemoveFriend(ctx, req.(*RemoveFriendRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _FriendsService_ListFriends_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(ListFriendsRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(FriendsServiceServer).ListFriends(ctx, in)
	}
	info := &grpc.UnaryServerInfo{Server: srv, FullMethod: FriendsService_ListFriends_FullMethodName}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(FriendsServiceServer).ListFriends(ctx, req.(*ListFriendsRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _FriendsService_ListPendingFriendRequests_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(ListPendingFriendRequestsRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(FriendsServiceServer).ListPendingFriendRequests(ctx, in)
	}
	info := &grpc.UnaryServerInfo{Server: srv, FullMethod: FriendsService_ListPendingFriendRequests_FullMethodName}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(FriendsServiceServer).ListPendingFriendRequests(ctx, req.(*ListPendingFriendRequestsRequest))
	}
	return interceptor(ctx, in, info, handler)
}

var FriendsService_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "friends.FriendsService",
	HandlerType: (*FriendsServiceServer)(nil),
	Methods: []grpc.MethodDesc{
		{MethodName: "SendFriendRequest", Handler: _FriendsService_SendFriendRequest_Handler},
		{MethodName: "AcceptFriendRequest", Handler: _FriendsService_AcceptFriendRequest_Handler},
		{MethodName: "DeclineFriendRequest", Handler: _FriendsService_DeclineFriendRequest_Handler},
		{MethodName: "RemoveFriend", Handler: _FriendsService_RemoveFriend_Handler},
		{MethodName: "ListFriends", Handler: _FriendsService_ListFriends_Handler},
		{MethodName: "ListPendingFriendRequests", Handler: _FriendsService_ListPendingFriendRequests_Handler},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "proto/friends/friends.proto",
}
