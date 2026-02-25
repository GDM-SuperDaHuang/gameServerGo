package utils

const baseValue = 1000

/**
* serverId: GroupId
* (0~999):1 （1000~1999):2 (2000~2999):3
 */
func GetGroupIdByServerId(serverId uint32) uint32 {
	return (serverId / baseValue) + 1
}

/**
* serverId: GroupId
* (0~999):1 （1000~1999):2 (2000~2999):3
 */
func GetServerId(groupId uint32, serverIds []uint32) int {
	maxValue := groupId*baseValue - 1
	minValue := maxValue - baseValue + 1
	for _, target := range serverIds {
		if target >= minValue && target <= maxValue {
			return int(target)
		}
	}
	return 0
}

/**
* pdId: 1 ~ 65535
* pdId:GroupId
*(1~999):1 （1000~1999):2 (2000~2999):3
 */
func GetGroupIdByPb(pbId int) uint32 {
	return uint32((pbId / baseValue) + 1)
}
