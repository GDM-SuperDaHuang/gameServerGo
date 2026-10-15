package loginSdk

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sync"
	"time"
)

// 微信登录
const (
	wxAppId  string = "wxd3bb0fe451af6034"
	wxSecret string = "000e65d7846f977d665200773f81cfc9"
)

var (
	wxService     *MiniProgramService
	wxServiceOnce sync.Once
)

// 初始化（程序启动时调用一次）
func InitWxService() {
	wxServiceOnce.Do(func() {
		config := &MiniProgramConfig{
			AppID:  wxAppId,
			Secret: wxSecret,
		}
		wxService = NewMiniProgramService(config)
	})
}

// 微信小程序配置
type MiniProgramConfig struct {
	AppID  string
	Secret string
}

// 微信 jscode2session 响应结构
type WXSessionResponse struct {
	OpenID     string `json:"openid"`      // 用户唯一标识
	SessionKey string `json:"session_key"` // 会话密钥
	UnionID    string `json:"unionid"`     // 用户在开放平台的唯一标识符
	ErrCode    int    `json:"errcode"`     // 错误码
	ErrMsg     string `json:"errmsg"`      // 错误信息
}

// 微信错误码映射
var wxErrorCodes = map[int]string{
	-1:    "系统繁忙，请稍后再试",
	0:     "请求成功",
	40029: "code 无效或过期",
	40163: "code 已被使用",
	45009: "调用频率限制，请稍后再试",
	40226: "高风险等级用户，小程序登录拦截",
	40013: "appid 无效",
}

type MiniProgramService struct {
	config     *MiniProgramConfig
	httpClient *http.Client
}

func NewMiniProgramService(config *MiniProgramConfig) *MiniProgramService {
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{
			MinVersion: tls.VersionTLS12,
		},
	}
	return &MiniProgramService{
		config: config,
		httpClient: &http.Client{
			Transport: tr,
			Timeout:   10 * time.Second,
		},
	}
}

// Code2Session 调用微信接口获取 session_key 和 openid
func (s *MiniProgramService) Code2Session(code string) (*WXSessionResponse, error) {
	// 构建请求 URL
	params := url.Values{}
	params.Set("appid", s.config.AppID)
	params.Set("secret", s.config.Secret)
	params.Set("js_code", code)
	params.Set("grant_type", "authorization_code")

	apiURL := fmt.Sprintf("https://api.weixin.qq.com/sns/jscode2session?%s", params.Encode())

	// 发送 HTTPS GET 请求
	resp, err := s.httpClient.Get(apiURL)
	if err != nil {
		return nil, fmt.Errorf("请求微信接口失败: %w", err)
	}
	defer resp.Body.Close()

	// 读取响应体
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %w", err)
	}

	// 解析 JSON
	var wxResp WXSessionResponse
	if err := json.Unmarshal(body, &wxResp); err != nil {
		return nil, fmt.Errorf("解析响应 JSON 失败: %w", err)
	}

	// 检查微信返回的错误码
	if wxResp.ErrCode != 0 {
		errMsg := wxResp.ErrMsg
		if msg, ok := wxErrorCodes[wxResp.ErrCode]; ok && errMsg == "" {
			errMsg = msg
		}
		return nil, fmt.Errorf("微信接口错误 [%d]: %s", wxResp.ErrCode, errMsg)
	}

	return &wxResp, nil
}

func WxLoginReq(code string) *WXSessionResponse {
	// 初始化配置（实际应从配置文件或环境变量读取）
	if wxService == nil {
		return nil
	}
	// 1. 调用微信接口获取 openid 和 session_key
	wxResp, err := wxService.Code2Session(code)
	if err != nil {
		return nil
	}
	return wxResp
}
