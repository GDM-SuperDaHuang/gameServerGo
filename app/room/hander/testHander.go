package hander

import (
	"context"
	"fmt"
	"gameServer/protobuf/pbGo"
	"gameServer/service/common"
	"gameServer/service/services/node"
)

type HanderTest struct { //必须大写,必须使用指针
}

func (h *HanderTest) TestHandler(_ context.Context, player *common.Player, req *pbGo.TestRpcRep, resp *pbGo.TestRpcResp) *common.ErrorInfo {
	fmt.Println("%d===%s", req.Id, req.Name)
	resp.Id = 130
	resp.Name = "回包测试"
	if 4 == 5 {
		return common.Error(4)
	}
	// 主动推送
	node.Push(player, 1000, resp)
	return nil
}

// 早
func (h *HanderTest) TestMaxHandler(_ context.Context, player *common.Player, req *pbGo.TestRoundInfoReq, resp *pbGo.TestRoundInfoPush) *common.ErrorInfo {
	hint := &pbGo.Hint{}
	grid := &pbGo.Grid{
		IndexId:     12,
		ShowQuality: 15,
		QualityType: 152,
	}
	goods := &pbGo.Goods{
		ItemId:      1,
		Index:       21,
		ShowAll:     1,
		ShowContour: 100,
	}
	HintList := make([]*pbGo.Hint, 0, 30)
	hint.Id = 100
	hint.HintType = 2
	hint.Value = 10000
	hint.ValueType = 1241
	changeScreenInfo := &pbGo.ScreenInfo{}
	changeScreenInfo.GridList = make([]*pbGo.Grid, 0)
	changeScreenInfo.AllGoods = make([]*pbGo.Goods, 0)

	for i := 0; i < 20; i++ {
		HintList = append(HintList, hint)
	}
	for i := 0; i < 20; i++ {
		changeScreenInfo.GridList = append(changeScreenInfo.GridList, grid)
	}
	for i := 0; i < 20; i++ {
		changeScreenInfo.AllGoods = append(changeScreenInfo.AllGoods, goods)
	}
	resp.IsFinish = true
	resp.HintList = HintList
	resp.ChangeScreenInfo = changeScreenInfo
	resp.RoundIndex = 15
	resp.EndTimeOut = 2115841431521

	for i := 0; i < 10000; i++ {
		// 主动推送
		node.Push(player, 1099, resp)
	}
	return nil
}
