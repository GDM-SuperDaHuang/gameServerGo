package errorCode

const (
	//MessageID_Heart             uint16 = 1
	//MessageID_SecretShareTest   uint16 = 2
	//MessageID_SecretSharePubKey uint16 = 3
	//MessageID_Login             uint16 = 4

	ErrorCode_Success = 0 //默认成功
	// gate
	ErrorCode_ProtoUnarshalFailed uint16 = 101 // 协议解码失败
	ErrorCode_ProtoMarshalFailed  uint16 = 102 // 协议编码失败
	ErrorCode_ProtocolNotFound    uint16 = 103 // 协议未找到
	ErrorCode_RemoteCallFailed    uint16 = 104 // 远程调用失败
	ErrorCode_PushFailed          uint16 = 108 // 推送失败
	ErrorCode_GateReqNull         uint16 = 113 // 网关请求参数空
	ErrorCode_GateRespNull        uint16 = 114 // 网关请求响应空
	ErrorCode_LoginFailed         uint16 = 115 //登录失败
	ErrorCode_CreatUserFailed     uint16 = 116 // 创建用户失败
	ErrorCode_DBError             uint16 = 117 // 数据库错误
	ErrorCode_ReqIsNull           uint16 = 118 // 请求参数是null

	//config
	ErrorCode_GetConfigFailed uint16 = 200 // 获取配置失败
	ErrorCode_ConfigError     uint16 = 201 // 获取配置有误

	// room
	ErrorCode_NotJoinRoom    uint16 = 1000 // 没有加入任何房间
	ErrorCode_UseNullItem    uint16 = 1001 // 使用空道具
	ErrorCode_ReusingItem    uint16 = 1002 // 重复使用道具
	ErrorCode_UseItemNumErr  uint16 = 1003 // 重复使用道具数量有错误
	ErrorCode_AlreadyBet     uint16 = 1004 //已经竞拍过了
	ErrorCode_AlreadyAbstain uint16 = 1005 //已经竞拍弃权了

	// item
	ErrorCode_ItemNotEnough uint16 = 2000 // 道具不足
	ErrorCode_AlreadyReward uint16 = 2001 //已经领取过奖励
	ErrorCode_RepeatBuyHero uint16 = 2003 //重复购买人物
)
