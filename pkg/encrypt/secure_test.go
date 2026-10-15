package encrypt

import (
	"bytes"
	"testing"
)

func TestHandshakeEndToEnd(t *testing.T) {
	// 客户端发起
	cli, err := NewClientHandshake()
	if err != nil {
		t.Fatal(err)
	}
	// 服务端处理 KeyExchangeReq
	srv, err := NewServerHandshake(cli.ClientPublicKey(), cli.ClientRandom())
	if err != nil {
		t.Fatal(err)
	}
	// 客户端处理 KeyExchangeResp
	if err := cli.ProcessServerResp(srv.ServerPublicKey(), srv.ServerRandom()); err != nil {
		t.Fatal(err)
	}
	// 双方密钥一致
	if !bytes.Equal(cli.Keys().ClientWriteKey[:], srv.Keys().ClientWriteKey[:]) {
		t.Fatal("client_write_key mismatch")
	}
	if !bytes.Equal(cli.Keys().ServerWriteKey[:], srv.Keys().ServerWriteKey[:]) {
		t.Fatal("server_write_key mismatch")
	}
	if cli.Keys().ClientNoncePrefix != srv.Keys().ClientNoncePrefix {
		t.Fatal("client nonce prefix mismatch")
	}
	if cli.Keys().ServerNoncePrefix != srv.Keys().ServerNoncePrefix {
		t.Fatal("server nonce prefix mismatch")
	}
	// 72 字节拼接
	b72, err := Derive72(cli.sharedSecret, cli.clientRandom, cli.serverRandom)
	if err != nil {
		t.Fatal(err)
	}
	if len(b72) != 72 {
		t.Fatalf("want 72 bytes, got %d", len(b72))
	}
	// 双向确认
	cliMac := cli.ClientConfirm()
	if !srv.VerifyClient(cliMac) {
		t.Fatal("server verify client confirm failed")
	}
	srvMac := srv.ServerConfirm()
	if !cli.VerifyServer(srvMac) {
		t.Fatal("client verify server confirm failed")
	}
	// 双向加解密
	cliCh, _ := cli.BuildClientChannel()
	srvCh, _ := srv.BuildServerChannel()

	// 客户端->服务端
	wire, err := cliCh.Encrypt([]byte("hello server"))
	if err != nil {
		t.Fatal(err)
	}
	plain, err := srvCh.Decrypt(wire)
	if err != nil {
		t.Fatal(err)
	}
	if string(plain) != "hello server" {
		t.Fatalf("c2s mismatch: %q", plain)
	}
	// 服务端->客户端
	wire2, err := srvCh.Encrypt([]byte("hello client"))
	if err != nil {
		t.Fatal(err)
	}
	plain2, err := cliCh.Decrypt(wire2)
	if err != nil {
		t.Fatal(err)
	}
	if string(plain2) != "hello client" {
		t.Fatalf("s2c mismatch: %q", plain2)
	}
	// nonce 不重用: 同一明文两次加密线路不同
	w1, _ := cliCh.Encrypt([]byte("same"))
	w2, _ := cliCh.Encrypt([]byte("same"))
	if bytes.Equal(w1, w2) {
		t.Fatal("nonce reuse: two encrypts identical")
	}
	// 篡改检测
	wire2[len(wire2)-1] ^= 1
	if _, err := cliCh.Decrypt(wire2); err == nil {
		t.Fatal("tampered ciphertext should fail")
	}
}
