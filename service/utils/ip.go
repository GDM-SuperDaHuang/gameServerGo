package utils

import (
	"net"
	"os"
	"strings"
)

// LocalIP returns the non-loopback local IP of the host
func LocalIP() string {
	//addrs, err := net.InterfaceAddrs()
	//if err != nil {
	//}
	//inDocker := IsDocker()
	//
	//for _, address := range addrs {
	//	ipnet, ok := address.(*net.IPNet)
	//	if !ok || ipnet.IP.IsLoopback() || ipnet.IP.To4() == nil {
	//		continue
	//	}
	//	ip := ipnet.IP.String()
	//
	//	if inDocker {
	//		// 容器里，返回 Docker 网络（10.x.x.x）IP
	//		if strings.HasPrefix(ip, "10.") || strings.HasPrefix(ip, "172.") || strings.HasPrefix(ip, "192.168.") {
	//			return ip
	//		}
	//	} else {
	//		// 宿主机，排除 Docker 虚拟网卡
	//		if strings.HasPrefix(ip, "192.168.") && !strings.HasPrefix(ip, "10.") && !strings.HasPrefix(ip, "172.") && !strings.HasPrefix(ip, "docker") {
	//			return ip
	//		}
	//	}
	//
	//	//if ipnet, ok := address.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
	//	//	if ipnet.IP.To4() != nil {
	//	//		return ipnet.IP.String()
	//	//	}
	//	//}
	//}
	//
	//return "127.0.0.1"

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
