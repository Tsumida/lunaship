package client

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/bufbuild/connect-go"
	"github.com/stretchr/testify/assert"
	v1 "github.com/tsumida/lunaship/api/v1"
	svc "github.com/tsumida/lunaship/api/v1/v1connect"
)

func TestResourceClient(t *testing.T) {
	client := svc.NewResourceServiceClient(
		http.DefaultClient,
		"http://localhost:8080",
	)

	resp, err := client.GetServiceMeta(
		context.TODO(),
		&connect.Request[v1.GetServiceMetaRequest]{
			Msg: &v1.GetServiceMetaRequest{},
		},
	)
	assert.NoError(t, err, "expected no err")
	buf, _ := json.MarshalIndent(
		resp, "", "  ",
	)
	t.Log(string(buf))
}

func TestResourceListMachine(t *testing.T) {
	client := svc.NewResourceServiceClient(
		http.DefaultClient,
		"http://localhost:8080",
	)

	resp, err := client.ListMachine(
		context.TODO(),
		&connect.Request[v1.ListMachineRequest]{
			Msg: &v1.ListMachineRequest{},
		},
	)
	assert.NoError(t, err, "expected no err")
	buf, _ := json.MarshalIndent(
		resp, "", "  ",
	)
	t.Log(string(buf))

	machineCnt := resp.Msg.Data.Total
	assert.Equalf(t, uint64(2), machineCnt, "incorrect machine count")
}
