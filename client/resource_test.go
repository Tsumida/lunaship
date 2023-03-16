package client

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/bufbuild/connect-go"
	v1 "github.com/tsumida/lunaship/api/v1"
	svc "github.com/tsumida/lunaship/api/v1/v1connect"
)

func TestResourceClient(t *testing.T) {
	client := svc.NewMetaServiceClient(
		http.DefaultClient,
		"http://localhost:8080",
	)

	resp, err := client.GetServiceMeta(
		context.TODO(),
		&connect.Request[v1.GetServiceMetaRequest]{
			Msg: &v1.GetServiceMetaRequest{},
		},
	)
	FailIf(err, t)
	buf, _ := json.MarshalIndent(
		resp, "", "  ",
	)
	t.Log(string(buf))
}
