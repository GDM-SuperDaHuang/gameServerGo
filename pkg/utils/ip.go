package utils

import (
	"net"
	"os"
	"strings"
)

// LocalIP returns the non-loopback local IP of the host
func LocalIP() string {
	ifaces, err := net.Interfaces()
	if err != nil {
		return ""
	}
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue // 网卡没启用或是 loopback
		}
		addrs, _ := iface.Addrs()
		for _, addr := range addrs {
			if ipnet, ok := addr.(*net.IPNet); ok && ipnet.IP.To4() != nil {
				return ipnet.IP.String()
			}
		}
	}
	return ""
}

func IsDocker() bool {
	data, err := os.ReadFile("/proc/1/cgroup")
	if err != nil {
		return false
	}
	return strings.Contains(string(data), "docker") || strings.Contains(string(data), "kubepods")
}

func Max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// F2 返回 num 对应的 2的n次方
// 2 -> 2, 2^0
// 3 -> 4, 2^2
// 4 -> 4, 2^2
// 5 -> 8, 2^3
func F2(num int) int {
	if num <= 0 {
		return 1
	}

	num = num - 1
	num |= num >> 1
	num |= num >> 2
	num |= num >> 4
	num |= num >> 8
	num |= num >> 16

	return int(num + 1)
}
