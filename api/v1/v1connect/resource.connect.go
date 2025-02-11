// Code generated by protoc-gen-connect-go. DO NOT EDIT.
//
// Source: resource.proto

package v1connect

import (
	context "context"
	errors "errors"
	connect_go "github.com/bufbuild/connect-go"
	v1 "github.com/tsumida/lunaship/api/v1"
	http "net/http"
	strings "strings"
)

// This is a compile-time assertion to ensure that this generated file and the connect package are
// compatible. If you get a compiler error that this constant is not defined, this code was
// generated with a version of connect newer than the one compiled into your binary. You can fix the
// problem by either regenerating this code with an older version of connect or updating the connect
// version compiled into your binary.
const _ = connect_go.IsAtLeastVersion0_1_0

const (
	// ResourceServiceName is the fully-qualified name of the ResourceService service.
	ResourceServiceName = "meta.ResourceService"
)

// These constants are the fully-qualified names of the RPCs defined in this package. They're
// exposed at runtime as Spec.Procedure and as the final two segments of the HTTP route.
//
// Note that these are different from the fully-qualified method names used by
// google.golang.org/protobuf/reflect/protoreflect. To convert from these constants to
// reflection-formatted method names, remove the leading slash and convert the remaining slash to a
// period.
const (
	// ResourceServiceGetServiceMetaProcedure is the fully-qualified name of the ResourceService's
	// GetServiceMeta RPC.
	ResourceServiceGetServiceMetaProcedure = "/meta.ResourceService/GetServiceMeta"
	// ResourceServiceListMachineProcedure is the fully-qualified name of the ResourceService's
	// ListMachine RPC.
	ResourceServiceListMachineProcedure = "/meta.ResourceService/ListMachine"
)

// ResourceServiceClient is a client for the meta.ResourceService service.
type ResourceServiceClient interface {
	GetServiceMeta(context.Context, *connect_go.Request[v1.GetServiceMetaRequest]) (*connect_go.Response[v1.GetServiceMetaResponse], error)
	ListMachine(context.Context, *connect_go.Request[v1.ListMachineRequest]) (*connect_go.Response[v1.ListMachineResponse], error)
}

// NewResourceServiceClient constructs a client for the meta.ResourceService service. By default, it
// uses the Connect protocol with the binary Protobuf Codec, asks for gzipped responses, and sends
// uncompressed requests. To use the gRPC or gRPC-Web protocols, supply the connect.WithGRPC() or
// connect.WithGRPCWeb() options.
//
// The URL supplied here should be the base URL for the Connect or gRPC server (for example,
// http://api.acme.com or https://acme.com/grpc).
func NewResourceServiceClient(httpClient connect_go.HTTPClient, baseURL string, opts ...connect_go.ClientOption) ResourceServiceClient {
	baseURL = strings.TrimRight(baseURL, "/")
	return &resourceServiceClient{
		getServiceMeta: connect_go.NewClient[v1.GetServiceMetaRequest, v1.GetServiceMetaResponse](
			httpClient,
			baseURL+ResourceServiceGetServiceMetaProcedure,
			opts...,
		),
		listMachine: connect_go.NewClient[v1.ListMachineRequest, v1.ListMachineResponse](
			httpClient,
			baseURL+ResourceServiceListMachineProcedure,
			opts...,
		),
	}
}

// resourceServiceClient implements ResourceServiceClient.
type resourceServiceClient struct {
	getServiceMeta *connect_go.Client[v1.GetServiceMetaRequest, v1.GetServiceMetaResponse]
	listMachine    *connect_go.Client[v1.ListMachineRequest, v1.ListMachineResponse]
}

// GetServiceMeta calls meta.ResourceService.GetServiceMeta.
func (c *resourceServiceClient) GetServiceMeta(ctx context.Context, req *connect_go.Request[v1.GetServiceMetaRequest]) (*connect_go.Response[v1.GetServiceMetaResponse], error) {
	return c.getServiceMeta.CallUnary(ctx, req)
}

// ListMachine calls meta.ResourceService.ListMachine.
func (c *resourceServiceClient) ListMachine(ctx context.Context, req *connect_go.Request[v1.ListMachineRequest]) (*connect_go.Response[v1.ListMachineResponse], error) {
	return c.listMachine.CallUnary(ctx, req)
}

// ResourceServiceHandler is an implementation of the meta.ResourceService service.
type ResourceServiceHandler interface {
	GetServiceMeta(context.Context, *connect_go.Request[v1.GetServiceMetaRequest]) (*connect_go.Response[v1.GetServiceMetaResponse], error)
	ListMachine(context.Context, *connect_go.Request[v1.ListMachineRequest]) (*connect_go.Response[v1.ListMachineResponse], error)
}

// NewResourceServiceHandler builds an HTTP handler from the service implementation. It returns the
// path on which to mount the handler and the handler itself.
//
// By default, handlers support the Connect, gRPC, and gRPC-Web protocols with the binary Protobuf
// and JSON codecs. They also support gzip compression.
func NewResourceServiceHandler(svc ResourceServiceHandler, opts ...connect_go.HandlerOption) (string, http.Handler) {
	resourceServiceGetServiceMetaHandler := connect_go.NewUnaryHandler(
		ResourceServiceGetServiceMetaProcedure,
		svc.GetServiceMeta,
		opts...,
	)
	resourceServiceListMachineHandler := connect_go.NewUnaryHandler(
		ResourceServiceListMachineProcedure,
		svc.ListMachine,
		opts...,
	)
	return "/meta.ResourceService/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case ResourceServiceGetServiceMetaProcedure:
			resourceServiceGetServiceMetaHandler.ServeHTTP(w, r)
		case ResourceServiceListMachineProcedure:
			resourceServiceListMachineHandler.ServeHTTP(w, r)
		default:
			http.NotFound(w, r)
		}
	})
}

// UnimplementedResourceServiceHandler returns CodeUnimplemented from all methods.
type UnimplementedResourceServiceHandler struct{}

func (UnimplementedResourceServiceHandler) GetServiceMeta(context.Context, *connect_go.Request[v1.GetServiceMetaRequest]) (*connect_go.Response[v1.GetServiceMetaResponse], error) {
	return nil, connect_go.NewError(connect_go.CodeUnimplemented, errors.New("meta.ResourceService.GetServiceMeta is not implemented"))
}

func (UnimplementedResourceServiceHandler) ListMachine(context.Context, *connect_go.Request[v1.ListMachineRequest]) (*connect_go.Response[v1.ListMachineResponse], error) {
	return nil, connect_go.NewError(connect_go.CodeUnimplemented, errors.New("meta.ResourceService.ListMachine is not implemented"))
}
