// Copyright 2024 Gravitational, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Code generated by protoc-gen-go-grpc. DO NOT EDIT.
// versions:
// - protoc-gen-go-grpc v1.3.0
// - protoc             (unknown)
// source: teleport/scim/v1/scim_service.proto

package scimv1

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
	SCIMService_ListSCIMResources_FullMethodName  = "/teleport.scim.v1.SCIMService/ListSCIMResources"
	SCIMService_GetSCIMResource_FullMethodName    = "/teleport.scim.v1.SCIMService/GetSCIMResource"
	SCIMService_CreateSCIMResource_FullMethodName = "/teleport.scim.v1.SCIMService/CreateSCIMResource"
	SCIMService_UpdateSCIMResource_FullMethodName = "/teleport.scim.v1.SCIMService/UpdateSCIMResource"
)

// SCIMServiceClient is the client API for SCIMService service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
type SCIMServiceClient interface {
	// List fetches all (or a subset of all) resources resources of a given type
	ListSCIMResources(ctx context.Context, in *ListSCIMResourcesRequest, opts ...grpc.CallOption) (*ResourceList, error)
	// GetSCIMResource fetches a single SCIM resource from the server by name
	GetSCIMResource(ctx context.Context, in *GetSCIMResourceRequest, opts ...grpc.CallOption) (*Resource, error)
	// CreateSCIResource creates a new SCIM resource based on a supplied
	// resource description
	CreateSCIMResource(ctx context.Context, in *CreateSCIMResourceRequest, opts ...grpc.CallOption) (*Resource, error)
	// UpdateResource handles a request to update a resource, returning a
	// representation of the updated resource
	UpdateSCIMResource(ctx context.Context, in *UpdateSCIMResourceRequest, opts ...grpc.CallOption) (*Resource, error)
}

type sCIMServiceClient struct {
	cc grpc.ClientConnInterface
}

func NewSCIMServiceClient(cc grpc.ClientConnInterface) SCIMServiceClient {
	return &sCIMServiceClient{cc}
}

func (c *sCIMServiceClient) ListSCIMResources(ctx context.Context, in *ListSCIMResourcesRequest, opts ...grpc.CallOption) (*ResourceList, error) {
	out := new(ResourceList)
	err := c.cc.Invoke(ctx, SCIMService_ListSCIMResources_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *sCIMServiceClient) GetSCIMResource(ctx context.Context, in *GetSCIMResourceRequest, opts ...grpc.CallOption) (*Resource, error) {
	out := new(Resource)
	err := c.cc.Invoke(ctx, SCIMService_GetSCIMResource_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *sCIMServiceClient) CreateSCIMResource(ctx context.Context, in *CreateSCIMResourceRequest, opts ...grpc.CallOption) (*Resource, error) {
	out := new(Resource)
	err := c.cc.Invoke(ctx, SCIMService_CreateSCIMResource_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *sCIMServiceClient) UpdateSCIMResource(ctx context.Context, in *UpdateSCIMResourceRequest, opts ...grpc.CallOption) (*Resource, error) {
	out := new(Resource)
	err := c.cc.Invoke(ctx, SCIMService_UpdateSCIMResource_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// SCIMServiceServer is the server API for SCIMService service.
// All implementations must embed UnimplementedSCIMServiceServer
// for forward compatibility
type SCIMServiceServer interface {
	// List fetches all (or a subset of all) resources resources of a given type
	ListSCIMResources(context.Context, *ListSCIMResourcesRequest) (*ResourceList, error)
	// GetSCIMResource fetches a single SCIM resource from the server by name
	GetSCIMResource(context.Context, *GetSCIMResourceRequest) (*Resource, error)
	// CreateSCIResource creates a new SCIM resource based on a supplied
	// resource description
	CreateSCIMResource(context.Context, *CreateSCIMResourceRequest) (*Resource, error)
	// UpdateResource handles a request to update a resource, returning a
	// representation of the updated resource
	UpdateSCIMResource(context.Context, *UpdateSCIMResourceRequest) (*Resource, error)
	mustEmbedUnimplementedSCIMServiceServer()
}

// UnimplementedSCIMServiceServer must be embedded to have forward compatible implementations.
type UnimplementedSCIMServiceServer struct {
}

func (UnimplementedSCIMServiceServer) ListSCIMResources(context.Context, *ListSCIMResourcesRequest) (*ResourceList, error) {
	return nil, status.Errorf(codes.Unimplemented, "method ListSCIMResources not implemented")
}
func (UnimplementedSCIMServiceServer) GetSCIMResource(context.Context, *GetSCIMResourceRequest) (*Resource, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GetSCIMResource not implemented")
}
func (UnimplementedSCIMServiceServer) CreateSCIMResource(context.Context, *CreateSCIMResourceRequest) (*Resource, error) {
	return nil, status.Errorf(codes.Unimplemented, "method CreateSCIMResource not implemented")
}
func (UnimplementedSCIMServiceServer) UpdateSCIMResource(context.Context, *UpdateSCIMResourceRequest) (*Resource, error) {
	return nil, status.Errorf(codes.Unimplemented, "method UpdateSCIMResource not implemented")
}
func (UnimplementedSCIMServiceServer) mustEmbedUnimplementedSCIMServiceServer() {}

// UnsafeSCIMServiceServer may be embedded to opt out of forward compatibility for this service.
// Use of this interface is not recommended, as added methods to SCIMServiceServer will
// result in compilation errors.
type UnsafeSCIMServiceServer interface {
	mustEmbedUnimplementedSCIMServiceServer()
}

func RegisterSCIMServiceServer(s grpc.ServiceRegistrar, srv SCIMServiceServer) {
	s.RegisterService(&SCIMService_ServiceDesc, srv)
}

func _SCIMService_ListSCIMResources_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(ListSCIMResourcesRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(SCIMServiceServer).ListSCIMResources(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: SCIMService_ListSCIMResources_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(SCIMServiceServer).ListSCIMResources(ctx, req.(*ListSCIMResourcesRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _SCIMService_GetSCIMResource_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(GetSCIMResourceRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(SCIMServiceServer).GetSCIMResource(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: SCIMService_GetSCIMResource_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(SCIMServiceServer).GetSCIMResource(ctx, req.(*GetSCIMResourceRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _SCIMService_CreateSCIMResource_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(CreateSCIMResourceRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(SCIMServiceServer).CreateSCIMResource(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: SCIMService_CreateSCIMResource_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(SCIMServiceServer).CreateSCIMResource(ctx, req.(*CreateSCIMResourceRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _SCIMService_UpdateSCIMResource_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(UpdateSCIMResourceRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(SCIMServiceServer).UpdateSCIMResource(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: SCIMService_UpdateSCIMResource_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(SCIMServiceServer).UpdateSCIMResource(ctx, req.(*UpdateSCIMResourceRequest))
	}
	return interceptor(ctx, in, info, handler)
}

// SCIMService_ServiceDesc is the grpc.ServiceDesc for SCIMService service.
// It's only intended for direct use with grpc.RegisterService,
// and not to be introspected or modified (even as a copy)
var SCIMService_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "teleport.scim.v1.SCIMService",
	HandlerType: (*SCIMServiceServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "ListSCIMResources",
			Handler:    _SCIMService_ListSCIMResources_Handler,
		},
		{
			MethodName: "GetSCIMResource",
			Handler:    _SCIMService_GetSCIMResource_Handler,
		},
		{
			MethodName: "CreateSCIMResource",
			Handler:    _SCIMService_CreateSCIMResource_Handler,
		},
		{
			MethodName: "UpdateSCIMResource",
			Handler:    _SCIMService_UpdateSCIMResource_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "teleport/scim/v1/scim_service.proto",
}
