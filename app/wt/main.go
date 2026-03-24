package main

import (
	"encoding/binary"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"sync/atomic"

	"github.com/gobwas/ws"
	"github.com/gobwas/ws/wsutil"
)

var (
	//WSAddr  = ":7002"
	//TCPAddr = "host.docker.internal:17001"
	WSAddr  = getEnv("WS_ADDR", ":7001")
	TCPAddr = getEnv("TCP_ADDR", "host.docker.internal:17001")
)

const HeadLen = 12

var connId uint64

func main() {
	http.HandleFunc("/", handleWS)

	log.Println("WS Bridge started at", WSAddr)
	log.Fatal(http.ListenAndServe(WSAddr, nil))
}

func getEnv(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}

func handleWS(w http.ResponseWriter, r *http.Request) {
	id := atomic.AddUint64(&connId, 1)

	conn, _, _, err := ws.UpgradeHTTP(r, w)
	if err != nil {
		log.Println("upgrade err:", err)
		return
	}

	log.Printf("[Conn %d] WS connected\n", id)

	// TCP连接
	tcpConn, err := net.Dial("tcp", TCPAddr)
	if err != nil {
		log.Println("tcp dial err:", err)
		conn.Close()
		return
	}

	tcpConn.(*net.TCPConn).SetNoDelay(true)

	done := make(chan struct{})
	var closeOnce atomic.Bool

	closeAll := func() {
		if closeOnce.CompareAndSwap(false, true) {
			close(done)
			conn.Close()
			tcpConn.Close()
		}
	}

	// ===== WS -> TCP =====
	go func() {
		defer closeAll()
		for {
			msg, op, err := wsutil.ReadClientData(conn)
			if err != nil {
				if err != io.EOF {
					log.Printf("[Conn %d] WS read err: %v\n", id, err)
				}
				return
			}

			if op != ws.OpBinary {
				continue
			}

			_, err = tcpConn.Write(msg)
			if err != nil {
				log.Printf("[Conn %d] TCP write err: %v\n", id, err)
				return
			}
		}
	}()

	// ===== TCP -> WS（带拆包）=====
	go func() {
		defer closeAll()

		buf := make([]byte, 0, 64*1024)
		tmp := make([]byte, 4096)

		for {
			n, err := tcpConn.Read(tmp)
			if err != nil {
				if err != io.EOF {
					log.Printf("[Conn %d] TCP read err: %v\n", id, err)
				}
				return
			}

			buf = append(buf, tmp[:n]...)

			// ===== 拆包 =====
			for {
				if len(buf) < HeadLen {
					break
				}

				bodyLen := binary.BigEndian.Uint16(buf[0:2])
				totalLen := int(HeadLen + bodyLen)

				if len(buf) < totalLen {
					break
				}

				packet := buf[:totalLen]

				// 发WS
				err = wsutil.WriteServerBinary(conn, packet)
				if err != nil {
					log.Printf("[Conn %d] WS write err: %v\n", id, err)
					return
				}

				// 滑动窗口
				buf = buf[totalLen:]
			}
		}
	}()

	// ===== 关闭控制 =====
	select {
	case <-done:
	}

	log.Printf("[Conn %d] closed\n", id)
	conn.Close()
	tcpConn.Close()
}
