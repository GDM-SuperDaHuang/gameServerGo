package utils

func GetGroupIdByServerId(serverId uint32) uint32 {
	return (serverId / 1000) + 1 // (1~999):1 （1000~1999):2 (2000~2999):3
}
func GetGroupIdByPb(pbId int) int {
	return (pbId / 1000) + 1 // (1~999):1 （1000~1999):2 (2000~2999):3
}
func GetServerIdByServerId(groupId uint32) uint32 {
	return (pbId / 1000) + 1 // (1~999):1 （1000~1999):2 (2000~2999):3
}
