// Code generated by protoc-gen-go-grpc. DO NOT EDIT.
// versions:
// - protoc-gen-go-grpc v1.3.0
// - protoc             v4.25.2
// source: felixbackend.proto

package proto

import (
	context "context"

	grpc "google.golang.org/grpc"
	codes "google.golang.org/grpc/codes"
	status "google.golang.org/grpc/status"
)

// This is a compile-time assertion to ensure that this generated file
// is compatible with the grpc package it is being compiled against.
// Requires gRPC-Go v1.32.0 or later.
const _ = grpc.SupportPackageIsVersion7

const (
	PolicySync_Sync_FullMethodName = "/felix.PolicySync/Sync"
)

// PolicySyncClient is the client API for PolicySync service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
type PolicySyncClient interface {
	// On this API, only the following payloads will be sent:
	//   - InSync
	//   - IPSetUpdate
	//   - IPSetDeltaUpdate
	//   - IPSetRemove
	//   - ActiveProfileUpdate
	//   - ActiveProfileRemove
	//   - ActivePolicyUpdate
	//   - ActivePolicyRemove
	//   - WorkloadEndpointUpdate
	//   - WorkloadEndpointRemove
	//   - ServiceAccountUpdate
	//   - ServiceAccountRemove
	//   - NamespaceUpdate
	//   - NamespaceRemove
	//   - RouteUpdate
	//   - RouteRemove
	//   - VXLANTunnelEndpointUpdate
	//   - VXLANTunnelEndpointRemove
	Sync(ctx context.Context, in *SyncRequest, opts ...grpc.CallOption) (PolicySync_SyncClient, error)
}

type policySyncClient struct {
	cc grpc.ClientConnInterface
}

func NewPolicySyncClient(cc grpc.ClientConnInterface) PolicySyncClient {
	return &policySyncClient{cc}
}

func (c *policySyncClient) Sync(ctx context.Context, in *SyncRequest, opts ...grpc.CallOption) (PolicySync_SyncClient, error) {
	stream, err := c.cc.NewStream(ctx, &PolicySync_ServiceDesc.Streams[0], PolicySync_Sync_FullMethodName, opts...)
	if err != nil {
		return nil, err
	}
	x := &policySyncSyncClient{stream}
	if err := x.ClientStream.SendMsg(in); err != nil {
		return nil, err
	}
	if err := x.ClientStream.CloseSend(); err != nil {
		return nil, err
	}
	return x, nil
}

type PolicySync_SyncClient interface {
	Recv() (*ToDataplane, error)
	grpc.ClientStream
}

type policySyncSyncClient struct {
	grpc.ClientStream
}

func (x *policySyncSyncClient) Recv() (*ToDataplane, error) {
	m := new(ToDataplane)
	if err := x.ClientStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

// PolicySyncServer is the server API for PolicySync service.
// All implementations must embed UnimplementedPolicySyncServer
// for forward compatibility
type PolicySyncServer interface {
	// On this API, only the following payloads will be sent:
	//   - InSync
	//   - IPSetUpdate
	//   - IPSetDeltaUpdate
	//   - IPSetRemove
	//   - ActiveProfileUpdate
	//   - ActiveProfileRemove
	//   - ActivePolicyUpdate
	//   - ActivePolicyRemove
	//   - WorkloadEndpointUpdate
	//   - WorkloadEndpointRemove
	//   - ServiceAccountUpdate
	//   - ServiceAccountRemove
	//   - NamespaceUpdate
	//   - NamespaceRemove
	//   - RouteUpdate
	//   - RouteRemove
	//   - VXLANTunnelEndpointUpdate
	//   - VXLANTunnelEndpointRemove
	Sync(*SyncRequest, PolicySync_SyncServer) error
	mustEmbedUnimplementedPolicySyncServer()
}

// UnimplementedPolicySyncServer must be embedded to have forward compatible implementations.
type UnimplementedPolicySyncServer struct {
}

func (UnimplementedPolicySyncServer) Sync(*SyncRequest, PolicySync_SyncServer) error {
	return status.Errorf(codes.Unimplemented, "method Sync not implemented")
}
func (UnimplementedPolicySyncServer) mustEmbedUnimplementedPolicySyncServer() {}

// UnsafePolicySyncServer may be embedded to opt out of forward compatibility for this service.
// Use of this interface is not recommended, as added methods to PolicySyncServer will
// result in compilation errors.
type UnsafePolicySyncServer interface {
	mustEmbedUnimplementedPolicySyncServer()
}

func RegisterPolicySyncServer(s grpc.ServiceRegistrar, srv PolicySyncServer) {
	s.RegisterService(&PolicySync_ServiceDesc, srv)
}

func _PolicySync_Sync_Handler(srv interface{}, stream grpc.ServerStream) error {
	m := new(SyncRequest)
	if err := stream.RecvMsg(m); err != nil {
		return err
	}
	return srv.(PolicySyncServer).Sync(m, &policySyncSyncServer{stream})
}

type PolicySync_SyncServer interface {
	Send(*ToDataplane) error
	grpc.ServerStream
}

type policySyncSyncServer struct {
	grpc.ServerStream
}

func (x *policySyncSyncServer) Send(m *ToDataplane) error {
	return x.ServerStream.SendMsg(m)
}

// PolicySync_ServiceDesc is the grpc.ServiceDesc for PolicySync service.
// It's only intended for direct use with grpc.RegisterService,
// and not to be introspected or modified (even as a copy)
var PolicySync_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "felix.PolicySync",
	HandlerType: (*PolicySyncServer)(nil),
	Methods:     []grpc.MethodDesc{},
	Streams: []grpc.StreamDesc{
		{
			StreamName:    "Sync",
			Handler:       _PolicySync_Sync_Handler,
			ServerStreams: true,
		},
	},
	Metadata: "felixbackend.proto",
}
