package loginSdk

// 引入包 go get github.com/bytedance/douyin-openapi-sdk-go
import (
	"gameServer/pkg/logger/log2"

	credential "github.com/bytedance/douyin-openapi-credential-go/client"
	openApiSdkClient "github.com/bytedance/douyin-openapi-sdk-go/client"
	"go.uber.org/zap"
)

var (
	AppId        string = "ttc840befd562334c207"
	Secret       string = "de858c6bddc6118d5dee540b5ed2fdacbdbd327a"
	douyinClient *openApiSdkClient.Client
)

type DouyinResponse struct {
	Openid          *string //抖音应用唯一
	Message         *string
	Error           *int64
	Unionid         *string //抖音唯一
	AnonymousOpenid *string
	Errcode         *int64 // 错误码，成功：0
	SessionKey      *string
	Errmsg          *string
}

func DouyinSendReq(appId, code, secret string) *DouyinResponse {
	sdkRequest := &openApiSdkClient.AppsJscode2sessionRequest{}
	//sdkRequest.SetAnonymousCode("hheVSHLKwS") //匿名登录，
	sdkRequest.SetAppid(appId)
	sdkRequest.SetCode(code) //前端获取
	sdkRequest.SetSecret(secret)
	// sdk调用
	sdkResponse, err := douyinClient.AppsJscode2session(sdkRequest)
	if err != nil {
		log2.Get().Warn("douyin sdkResponse false ", zap.Any("err", err))
		return nil
	}
	d := &DouyinResponse{}
	d.Openid = sdkResponse.Unionid        //抖音应用唯一
	d.Unionid = sdkResponse.Unionid       //抖音唯一
	d.SessionKey = sdkResponse.SessionKey //会话
	d.Errcode = sdkResponse.Error         // 错误码，成功：0
	return d
}

func InitDouyinClient(appId, secret string) error {
	// 初始化SDK client
	opt := new(credential.Config).
		SetClientKey(appId).    // 改成自己的app_id
		SetClientSecret(secret) // 改成自己的secret
	sdkClient, err := openApiSdkClient.NewClient(opt)
	if err != nil {
		log2.Get().Warn("douyin sdk init err:", zap.Any("err", err))
		return err
	}
	douyinClient = sdkClient
	return nil
}
func GetDouyinClient() *openApiSdkClient.Client {
	return douyinClient
}
