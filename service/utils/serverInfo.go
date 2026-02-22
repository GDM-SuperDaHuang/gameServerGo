package utils

/**
* serverId: 1~ 65*1000
* GroupId:1~65
 */
func GetGroupIdByServerId(serverId uint32) uint32 {
	return (serverId / 1000) + 1 // (1~999):1 （1000~1999):2 (2000~2999):3
}

/**
* pdId: 1 ~ 65535
* GroupId:1~65
*(1~999):1 （1000~1999):2 (2000~2999):3
 */
func GetGroupIdByPb(pbId int) uint32 {
	return uint32((pbId / 1000) + 1)
}

func GetServerIdByServerId(groupId uint32, serverIds []uint32) int {
	maxValue := groupId*1000 - 1
	minValue := maxValue - 1000
	for _, target := range serverIds {
		if target >= minValue && target <= maxValue {
			return int(target)
		}
	}
	return 0
}
