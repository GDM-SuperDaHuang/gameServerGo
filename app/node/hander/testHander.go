package hander

import (
	"context"
	"fmt"
	"gameServer/protobuf/pbGo"
	"gameServer/service/common"
)

type HanderTest struct { //必须大写,必须使用指针
}

func (h *HanderTest) TestHandler(_ context.Context, _ uint64, req *pbGo.TestRpcRep, resp *pbGo.TestRpcResp) *common.ErrorInfo {
	fmt.Println("%d===%s", req.Id, req.Name)
	resp.Id = 130
	resp.Name = "回包测试"
	if 4 == 5 {
		return common.Error(4)
	}
	return nil
}
