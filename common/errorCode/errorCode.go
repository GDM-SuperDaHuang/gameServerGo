package errorCode

const (
	ErrorCode_Success = 0 //默认成功

	//异常
	ErrorCode_Panic uint16 = 10 //异常

	// gate
	ErrorCode_ProtoUnarshalFailed uint16 = 101 // 协议解码失败
	ErrorCode_ProtoMarshalFailed  uint16 = 102 // 协议编码失败
	ErrorCode_ProtocolNotFound    uint16 = 103 // 协议未找到
	ErrorCode_RemoteCallFailed    uint16 = 104 // 远程调用失败
	ErrorCode_ServerBusy          uint16 = 105 // 服务繁忙（串行队列满，网关可重试或返回操作频繁）
	ErrorCode_PushFailed          uint16 = 108 // 推送失败
	ErrorCode_GateReqNull         uint16 = 113 // 网关请求参数空
	ErrorCode_GateRespNull        uint16 = 114 // 网关请求响应空
	ErrorCode_LoginFailed         uint16 = 115 //登录失败
	ErrorCode_CreatUserFailed     uint16 = 116 // 创建用户失败
	ErrorCode_DBError             uint16 = 117 // 数据库错误
	ErrorCode_ReqIsNull           uint16 = 118 // 请求参数是null
	ErrorCode_UserStatusBan       uint16 = 119 //被封号

	// secure channel (LR-GAME-V1)
	ErrorCode_KeyExchangeFailed    uint16 = 120 // 密钥交换失败(公钥/随机数非法)
	ErrorCode_KeyVerifyFailed      uint16 = 121 // 密钥确认失败(MAC 校验不通过)
	ErrorCode_SecureChannelMissing uint16 = 122 // 安全通道未建立
	ErrorCode_MustEncrypted        uint16 = 123 // 握手完成后必须加密传输
	ErrorCode_CompressedFailed     uint16 = 125 // 压缩失败
	ErrorCode_UnCompressedFailed   uint16 = 125 // 压缩失败
	ErrorCode_DecryptFailed        uint16 = 126 // 解密失败

	//config
	ErrorCode_GetConfigFailed    uint16 = 200 // 获取配置失败
	ErrorCode_UpdateConfigFailed uint16 = 201 // 更新配置失败

	// room
	ErrorCode_NotJoinRoom    uint16 = 1000 // 没有加入任何房间
	ErrorCode_UseNullItem    uint16 = 1001 // 使用空道具
	ErrorCode_ReusingItem    uint16 = 1002 // 重复使用道具
	ErrorCode_UseItemNumErr  uint16 = 1003 // 重复使用道具数量有错误
	ErrorCode_AlreadyBet     uint16 = 1004 //已经竞拍过了
	ErrorCode_AlreadyAbstain uint16 = 1005 //已经竞拍弃权了
	ErrorCode_BetZero        uint16 = 1006 //出价0

	// item
	ErrorCode_ItemNotEnough         uint16 = 2000 // 道具不足
	ErrorCode_AlreadyReward         uint16 = 2001 //已经领取过奖励
	ErrorCode_RepeatBuyHero         uint16 = 2003 //重复购买人物
	ErrorCode_NotSatisfiedCondition uint16 = 2004 //未满足条件
	ErrorCode_FrequentOperations    uint16 = 2005 //操作频繁
	ErrorCode_ActivityIsOver        uint16 = 2006 //活动结束

)
