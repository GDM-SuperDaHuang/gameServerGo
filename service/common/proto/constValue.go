package proto

const (
	MessageID_Heart             uint16 = 1
	MessageID_SecretShareTest   uint16 = 2
	MessageID_SecretSharePubKey uint16 = 3
	MessageID_Login             uint16 = 4

	ErrorCode_Success                    = 0
	ErrorCode_ProtoUnarshalFailed uint16 = 10001 // 协议解码失败
	ErrorCode_ProtoMarshalFailed  uint16 = 10002 // 协议编码失败
	ErrorCode_ProtocolNotFound    uint16 = 10003 // 协议未找到
	ErrorCode_RemoteCallFailed    uint16 = 10004 // 远程调用失败
	ErrorCode_ConfigNotFound      uint16 = 10005 // 配置未找到
	ErrorCode_IDZero              uint16 = 10006 // id为0
	ErrorCode_DevelopOnly         uint16 = 10007 // 仅在开发环境下可用
	ErrorCode_PushFailed          uint16 = 10008 // 推送失败
	ErrorCode_ParamError          uint16 = 10011 // 参数错误
	ErrorCode_ParamError2         uint16 = 10012 // 参数错误2
	ErrorCode_ParamError3         uint16 = 10013 // 参数错误3
	ErrorCode_ParamError4         uint16 = 10014 // 参数错误4
	ErrorCode_ParamError5         uint16 = 10015 // 参数错误5
	ErrorCode_ParamError6         uint16 = 10016 // 参数错误6
	ErrorCode_ParamError7         uint16 = 10017 // 参数错误7
	ErrorCode_ParamError8         uint16 = 10018 // 参数错误8
	ErrorCode_ParamError9         uint16 = 10019 // 参数错误9
	ErrorCode_ParamError10        uint16 = 10020 // 参数错误10
)
