package test

import (
	"gameServer/pkg/loginSdk"
	"testing"
)

const (
	code1 string = "Sb0rK4e63p5gabYswGr8sdeiercTdSaa-HI4GQbeHHhSeYJ_mEglXC_IYcIWIfDsoHPwTuWBfaG-ZhpZzzpLiWRHBC9zo64hWBJz8EImUMt9O5Nt24nm97Jhg9I"
	code2 string = ""
)

func TestRun(t *testing.T) {
	err := loginSdk.InitDouyinClient(loginSdk.AppId, loginSdk.Secret)
	if err != nil {
		panic(err)
	}
	resp1 := loginSdk.DouyinSendReq(loginSdk.AppId, code1, loginSdk.Secret)
	if resp1 == nil {
		t.Errorf("resp is nil")
	}
	resp2 := loginSdk.DouyinSendReq(loginSdk.AppId, code2, loginSdk.Secret)
	if resp2 == nil {
		t.Errorf("resp is nil")
	}
}
