package encrypt

// LR-GAME-V1 会话密钥派生与双向 AEAD 通道。
//
// 密钥协商流程：
//  1. client: shared = X25519(client_private, server_public)
//     server: shared = X25519(server_private, client_public)
//  2. salt = client_random || server_random
//     client_write_key    = HKDF-SHA256(shared, salt, "LR-GAME-V1 client write key", 32)
//     server_write_key    = HKDF-SHA256(shared, salt, "LR-GAME-V1 server write key", 32)
//     client_nonce_prefix = HKDF-SHA256(shared, salt, "LR-GAME-V1 client nonce prefix", 4)
//     server_nonce_prefix = HKDF-SHA256(shared, salt, "LR-GAME-V1 server nonce prefix", 4)
//     四段拼接共 72 字节: 32+4+32+4。
//  3. 加密: 客户端用 client_write_key, 服务端用 server_write_key (AES-256-GCM)。
//     解密: 客户端用 server_write_key, 服务端用 client_write_key。
//  4. 确认: HMAC-SHA256 双向确认, 见 handshake.go。
//
// 线路格式 (Encrypt 输出):
//  seq_be(8) || AES-GCM-Seal(key, nonce=prefix(4)||seq_be(8), plaintext)
//  其中后 16 字节为 GCM tag。解密时按前 8 字节重建 nonce 再 Open。

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/binary"
	"errors"
	"io"
	"sync/atomic"

	"golang.org/x/crypto/curve25519"
	"golang.org/x/crypto/hkdf"
)

const (
	// KeySize 写密钥长度。
	KeySize = 32
	// NoncePrefixSize nonce 前缀长度。
	NoncePrefixSize = 4
	// NonceSize GCM nonce 长度 = 4 prefix + 8 seq。
	NonceSize = 12
	// SeqSize 线路 seq 字段长度。
	SeqSize = 8
	// TagSize GCM tag 长度。
	TagSize = 16
	// RandomSize 服务端生成的 server_random 长度。
	RandomSize = 32
	// MinRandomSize 客户端 random 最小长度。
	MinRandomSize = 16
	// MaxRandomSize random 最大长度(防 DoS)。
	MaxRandomSize = 64

	InfoClientWriteKey    = "LR-GAME-V1 client write key"
	InfoServerWriteKey    = "LR-GAME-V1 server write key"
	InfoClientNoncePrefix = "LR-GAME-V1 client nonce prefix"
	InfoServerNoncePrefix = "LR-GAME-V1 server nonce prefix"

	labelClientConfirm = "LR-GAME-V1 client confirm"
	labelServerConfirm = "LR-GAME-V1 server confirm"
)

var (
	ErrInvalidPublicKey  = errors.New("encrypt: invalid public key length, want 32")
	ErrInvalidPrivateKey = errors.New("encrypt: invalid private key length, want 32")
	ErrInvalidRandom     = errors.New("encrypt: invalid random length")
	ErrNotVerified       = errors.New("encrypt: secure channel not established")
	ErrCiphertextShort   = errors.New("encrypt: ciphertext too short")
)

// SessionKeys 会话密钥(双方派生结果完全一致)。
type SessionKeys struct {
	ClientWriteKey    [KeySize]byte
	ServerWriteKey    [KeySize]byte
	ClientNoncePrefix [NoncePrefixSize]byte
	ServerNoncePrefix [NoncePrefixSize]byte
}

// GenerateKeyPair 生成 X25519 临时密钥对。
func GenerateKeyPair() (priv, pub [32]byte, err error) {
	if _, err = io.ReadFull(rand.Reader, priv[:]); err != nil {
		return priv, pub, err
	}
	// Clamp 由 curve25519.X25519 内部处理, 直接取私钥即可。
	pubBytes, err := curve25519.X25519(priv[:], curve25519.Basepoint)
	if err != nil {
		return priv, pub, err
	}
	copy(pub[:], pubBytes)
	return priv, pub, nil
}

// ComputeSharedSecret 计算 X25519 共享密钥。
func ComputeSharedSecret(priv, peerPub []byte) ([]byte, error) {
	if len(priv) != 32 {
		return nil, ErrInvalidPrivateKey
	}
	if len(peerPub) != 32 {
		return nil, ErrInvalidPublicKey
	}
	return curve25519.X25519(priv, peerPub)
}

// hkdfExpand 单次 HKDF-SHA256 派生。
func hkdfExpand(secret, salt []byte, info string, n int) ([]byte, error) {
	r := hkdf.New(sha256.New, secret, salt, []byte(info))
	out := make([]byte, n)
	if _, err := io.ReadFull(r, out); err != nil {
		return nil, err
	}
	return out, nil
}

// DeriveSessionKeys 由 sharedSecret + clientRandom + serverRandom 派生 72 字节会话材料。
func DeriveSessionKeys(sharedSecret, clientRandom, serverRandom []byte) (*SessionKeys, error) {
	if len(clientRandom) < MinRandomSize || len(clientRandom) > MaxRandomSize {
		return nil, ErrInvalidRandom
	}
	if len(serverRandom) < MinRandomSize || len(serverRandom) > MaxRandomSize {
		return nil, ErrInvalidRandom
	}
	salt := make([]byte, 0, len(clientRandom)+len(serverRandom))
	salt = append(salt, clientRandom...)
	salt = append(salt, serverRandom...)

	keys := &SessionKeys{}
	buf, err := hkdfExpand(sharedSecret, salt, InfoClientWriteKey, KeySize)
	if err != nil {
		return nil, err
	}
	copy(keys.ClientWriteKey[:], buf)

	if buf, err = hkdfExpand(sharedSecret, salt, InfoServerWriteKey, KeySize); err != nil {
		return nil, err
	}
	copy(keys.ServerWriteKey[:], buf)

	if buf, err = hkdfExpand(sharedSecret, salt, InfoClientNoncePrefix, NoncePrefixSize); err != nil {
		return nil, err
	}
	copy(keys.ClientNoncePrefix[:], buf)

	if buf, err = hkdfExpand(sharedSecret, salt, InfoServerNoncePrefix, NoncePrefixSize); err != nil {
		return nil, err
	}
	copy(keys.ServerNoncePrefix[:], buf)
	return keys, nil
}

// Derive72 一次性派生 72 字节拼接材料(兼容“一次返回72字节”描述):
// layout = client_write_key(32) || client_nonce_prefix(4) || server_write_key(32) || server_nonce_prefix(4)。
// 实现上等价于 4 次独立 HKDF(不同 info), 顺序拼接返回, 便于与其它语言对接验证。
func Derive72(sharedSecret, clientRandom, serverRandom []byte) ([]byte, error) {
	keys, err := DeriveSessionKeys(sharedSecret, clientRandom, serverRandom)
	if err != nil {
		return nil, err
	}
	out := make([]byte, 0, 72)
	out = append(out, keys.ClientWriteKey[:]...)
	out = append(out, keys.ClientNoncePrefix[:]...)
	out = append(out, keys.ServerWriteKey[:]...)
	out = append(out, keys.ServerNoncePrefix[:]...)
	return out, nil
}

// transcript = client_random || server_random || client_pub || server_pub。
func buildTranscript(clientRandom, serverRandom, clientPub, serverPub []byte) []byte {
	t := make([]byte, 0, len(clientRandom)+len(serverRandom)+len(clientPub)+len(serverPub)+32)
	t = append(t, clientRandom...)
	t = append(t, serverRandom...)
	t = append(t, clientPub...)
	t = append(t, serverPub...)
	return t
}

func computeConfirm(key []byte, label string, transcript []byte) []byte {
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(label))
	mac.Write(transcript)
	return mac.Sum(nil) // 32 bytes
}

// ComputeClientConfirm 客户端确认码: HMAC-SHA256(client_write_key, "LR-GAME-V1 client confirm" || transcript)。
func ComputeClientConfirm(keys *SessionKeys, clientRandom, serverRandom, clientPub, serverPub []byte) []byte {
	return computeConfirm(keys.ClientWriteKey[:], labelClientConfirm, buildTranscript(clientRandom, serverRandom, clientPub, serverPub))
}

// ComputeServerConfirm 服务端确认码: HMAC-SHA256(server_write_key, "LR-GAME-V1 server confirm" || transcript)。
func ComputeServerConfirm(keys *SessionKeys, clientRandom, serverRandom, clientPub, serverPub []byte) []byte {
	return computeConfirm(keys.ServerWriteKey[:], labelServerConfirm, buildTranscript(clientRandom, serverRandom, clientPub, serverPub))
}

// VerifyConfirm 常量时间比较。
func VerifyConfirm(expect, got []byte) bool {
	if len(expect) != 32 || len(got) != 32 {
		return false
	}
	return subtle.ConstantTimeCompare(expect, got) == 1
}

// newGCM 由 32 字节 key 构造 AES-256-GCM。
func newGCM(key []byte) (cipher.AEAD, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}

func buildNonce(prefix [NoncePrefixSize]byte, seq uint64) []byte {
	var nonce [NonceSize]byte
	copy(nonce[:4], prefix[:])
	binary.BigEndian.PutUint64(nonce[4:], seq)
	return nonce[:]
}

// seal 一次 AEAD 加密, 输出 seq(8) || ct||tag。
func seal(gcm cipher.AEAD, prefix [NoncePrefixSize]byte, seq uint64, plain []byte) []byte {
	nonce := buildNonce(prefix, seq)
	out := make([]byte, SeqSize, SeqSize+len(plain)+TagSize)
	binary.BigEndian.PutUint64(out, seq)
	return gcm.Seal(out, nonce, plain, nil)
}

// open 一次 AEAD 解密。
func open(gcm cipher.AEAD, prefix [NoncePrefixSize]byte, wire []byte) ([]byte, error) {
	if len(wire) < SeqSize+TagSize {
		return nil, ErrCiphertextShort
	}
	seq := binary.BigEndian.Uint64(wire[:SeqSize])
	nonce := buildNonce(prefix, seq)
	return gcm.Open(nil, nonce, wire[SeqSize:], nil)
}

// SecureChannel 双向通道。服务端视角: 写=server_write, 读=client_write;
// 客户端视角相反, 通过 NewServerChannel / NewClientChannel 区分。
// 满足 Encrypt 接口(Encrypt=本端写, Decrypt=对端读), 可直接作为会话级加解密器。
// 并发安全: 写序号原子递增保证 nonce 不重用; GCM Seal/Open 无共享可变状态。
type SecureChannel struct {
	writeGCM    cipher.AEAD
	readGCM     cipher.AEAD
	writePrefix [NoncePrefixSize]byte
	readPrefix  [NoncePrefixSize]byte
	writeSeq    atomic.Uint64
	isServer    bool
}

// NewServerChannel 服务端通道: 加密用 server_write_key, 解密用 client_write_key。
func NewServerChannel(keys *SessionKeys) (*SecureChannel, error) {
	wgcm, err := newGCM(keys.ServerWriteKey[:])
	if err != nil {
		return nil, err
	}
	rgcm, err := newGCM(keys.ClientWriteKey[:])
	if err != nil {
		return nil, err
	}
	return &SecureChannel{
		writeGCM: wgcm, readGCM: rgcm,
		writePrefix: keys.ServerNoncePrefix, readPrefix: keys.ClientNoncePrefix,
		isServer: true,
	}, nil
}

// NewClientChannel 客户端通道: 加密用 client_write_key, 解密用 server_write_key。
func NewClientChannel(keys *SessionKeys) (*SecureChannel, error) {
	wgcm, err := newGCM(keys.ClientWriteKey[:])
	if err != nil {
		return nil, err
	}
	rgcm, err := newGCM(keys.ServerWriteKey[:])
	if err != nil {
		return nil, err
	}
	return &SecureChannel{
		writeGCM: wgcm, readGCM: rgcm,
		writePrefix: keys.ClientNoncePrefix, readPrefix: keys.ServerNoncePrefix,
		isServer: false,
	}, nil
}

// Encrypt 本端加密(服务端->客户端 或 客户端->服务端)。
func (c *SecureChannel) Encrypt(plain []byte) ([]byte, error) {
	if c == nil || c.writeGCM == nil {
		return nil, ErrNotVerified
	}
	seq := c.writeSeq.Add(1) - 1
	return seal(c.writeGCM, c.writePrefix, seq, plain), nil
}

// Decrypt 对端解密。
func (c *SecureChannel) Decrypt(wire []byte) ([]byte, error) {
	if c == nil || c.readGCM == nil {
		return nil, ErrNotVerified
	}
	return open(c.readGCM, c.readPrefix, wire)
}

// Name 实现 Encrypt 接口。
func (c *SecureChannel) Name() string { return "lr-game-v1-aes256gcm" }

// IsServer 是否服务端视角。
func (c *SecureChannel) IsServer() bool { return c != nil && c.isServer }

// GenerateRandom 生成 n 字节随机数。
func GenerateRandom(n int) ([]byte, error) {
	b := make([]byte, n)
	if _, err := io.ReadFull(rand.Reader, b); err != nil {
		return nil, err
	}
	return b, nil
}
