package rpc

// BuildForwardReq 构建转发请求
//
//   - body: google protobuf marshaled bytes
func BuildForwardReq(realServerID, serverID uint32, roleID uint64, protocol pb_protocol.MessageID, body []byte) *pb_forward.ForwardReq {
	req := forwardReqPool.Get()
	req.RealServerId = realServerID
	req.ServerId = serverID
	req.RoleId = roleID
	req.Protocol = protocol
	req.Param = body
	return req
}
