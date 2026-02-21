package hander

import (
	"context"
	"fmt"
	"gameServer/protobuf/pbGo"
)

type HanderTest struct {
}

func (h *HanderTest) TestHandler(_ context.Context, _ uint64, req *pbGo.TestRpcRep, resp *pbGo.TestRpcResp) {
	fmt.Println("%d===%s", req.Id, req.Name)
	resp.Id = 130
	resp.Name = "回包测试"
}
