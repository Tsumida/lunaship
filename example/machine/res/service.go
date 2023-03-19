package res

import (
	"context"
	"os"

	"github.com/bufbuild/connect-go"
	v1 "github.com/tsumida/lunaship/api/v1"
	svc "github.com/tsumida/lunaship/api/v1/v1connect"
	"github.com/tsumida/lunaship/infra/utils"
)

type ResourceService struct {
	svc.ResourceServiceHandler
}

func (s *ResourceService) ListMachine(
	context.Context, *connect.Request[v1.ListMachineRequest],
) (*connect.Response[v1.ListMachineResponse], error) {
	var (
		resp = &connect.Response[v1.ListMachineResponse]{
			Msg: &v1.ListMachineResponse{},
		}
	)

	resp.Msg.Code = 200
	resp.Msg.TsSec = utils.NowInSec()
	resp.Msg.Data = EXAMPLE_LIST_MACHINES
	return resp, nil
}

func (s *ResourceService) GetServiceMeta(
	context.Context,
	*connect.Request[v1.GetServiceMetaRequest],
) (*connect.Response[v1.GetServiceMetaResponse], error) {
	var (
		resp = &connect.Response[v1.GetServiceMetaResponse]{
			Msg: &v1.GetServiceMetaResponse{},
		}
	)

	resp.Msg.Code = 200
	resp.Msg.TsSec = utils.NowInSec()
	resp.Msg.Data = &v1.ServiceMeta{
		ServiceId:         utils.StrOrDefault(os.Getenv("SERVICE_ID"), "resource"),
		ServiceVersion:    utils.StrOrDefault(os.Getenv("SERVICE_VERSION"), "unknown"),
		ServiceOpenapiDoc: "",
	}
	return resp, nil
}

var _ (svc.ResourceServiceHandler) = (*ResourceService)(nil)
