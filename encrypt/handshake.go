package encrypt

// ServerHandshake 服务端握手状态机(每个 WS 连接一份, 存于 Session)。
// 流程: KeyExchangeReq -> KeyExchangeResp -> KeyVerificationReq -> KeyVerificationResp。
type ServerHandshake struct {
	serverPriv    [32]byte
	serverPub     [32]byte
	clientPub     [32]byte
	clientRandom  []byte
	serverRandom  []byte
	sharedSecret  []byte
	keys          *SessionKeys
	serverConfirm []byte // 预计算的服务端确认码
}

// NewServerHandshake 处理客户端 KeyExchangeReq, 生成服务端应答材料。
// clientPub 必须 32 字节, clientRandom 长度 [MinRandomSize, MaxRandomSize]。
func NewServerHandshake(clientPub, clientRandom []byte) (*ServerHandshake, error) {
	if len(clientPub) != 32 {
		return nil, ErrInvalidPublicKey
	}
	if len(clientRandom) < MinRandomSize || len(clientRandom) > MaxRandomSize {
		return nil, ErrInvalidRandom
	}
	priv, pub, err := GenerateKeyPair()
	if err != nil {
		return nil, err
	}
	serverRandom, err := GenerateRandom(RandomSize)
	if err != nil {
		return nil, err
	}
	shared, err := ComputeSharedSecret(priv[:], clientPub)
	if err != nil {
		return nil, err
	}
	keys, err := DeriveSessionKeys(shared, clientRandom, serverRandom)
	if err != nil {
		return nil, err
	}
	h := &ServerHandshake{
		serverPriv:   priv,
		serverPub:    pub,
		serverRandom: serverRandom,
		sharedSecret: shared,
		keys:         keys,
	}
	copy(h.clientPub[:], clientPub)
	h.clientRandom = append([]byte(nil), clientRandom...)
	h.serverConfirm = ComputeServerConfirm(keys, h.clientRandom, serverRandom, h.clientPub[:], h.serverPub[:])
	return h, nil
}

// ServerPublicKey 服务端公钥(回 KeyExchangeResp)。
func (h *ServerHandshake) ServerPublicKey() []byte {
	out := make([]byte, 32)
	copy(out, h.serverPub[:])
	return out
}

// ServerRandom 服务端随机数(回 KeyExchangeResp)。
func (h *ServerHandshake) ServerRandom() []byte {
	return append([]byte(nil), h.serverRandom...)
}

// VerifyClient 确认客户端 KeyVerificationReq。
func (h *ServerHandshake) VerifyClient(confirmMAC []byte) bool {
	expect := ComputeClientConfirm(h.keys, h.clientRandom, h.serverRandom, h.clientPub[:], h.serverPub[:])
	return VerifyConfirm(expect, confirmMAC)
}

// ServerConfirm 服务端确认码(回 KeyVerificationResp)。
func (h *ServerHandshake) ServerConfirm() []byte {
	return append([]byte(nil), h.serverConfirm...)
}

// Keys 会话密钥(验证通过后用于创建 SecureChannel)。
func (h *ServerHandshake) Keys() *SessionKeys { return h.keys }

// BuildServerChannel 验证通过后创建服务端通道。
func (h *ServerHandshake) BuildServerChannel() (*SecureChannel, error) {
	return NewServerChannel(h.keys)
}

// ---- 客户端侧辅助(供联调/SDK/单测使用, 服务端不会调用) ----

// ClientHandshake 客户端握手状态机。
type ClientHandshake struct {
	clientPriv   [32]byte
	clientPub    [32]byte
	clientRandom []byte
	serverPub    []byte
	serverRandom []byte
	sharedSecret []byte
	keys         *SessionKeys
}

// NewClientHandshake 客户端发起: 生成 client keypair + random。
func NewClientHandshake() (*ClientHandshake, error) {
	priv, pub, err := GenerateKeyPair()
	if err != nil {
		return nil, err
	}
	cr, err := GenerateRandom(RandomSize)
	if err != nil {
		return nil, err
	}
	return &ClientHandshake{clientPriv: priv, clientPub: pub, clientRandom: cr}, nil
}

// ClientPublicKey 取客户端公钥。
func (c *ClientHandshake) ClientPublicKey() []byte {
	out := make([]byte, 32)
	copy(out, c.clientPub[:])
	return out
}

// ClientRandom 取客户端随机数。
func (c *ClientHandshake) ClientRandom() []byte {
	return append([]byte(nil), c.clientRandom...)
}

// ProcessServerResp 处理 KeyExchangeResp, 派生会话密钥。
func (c *ClientHandshake) ProcessServerResp(serverPub, serverRandom []byte) error {
	if len(serverPub) != 32 {
		return ErrInvalidPublicKey
	}
	if len(serverRandom) < MinRandomSize || len(serverRandom) > MaxRandomSize {
		return ErrInvalidRandom
	}
	shared, err := ComputeSharedSecret(c.clientPriv[:], serverPub)
	if err != nil {
		return err
	}
	keys, err := DeriveSessionKeys(shared, c.clientRandom, serverRandom)
	if err != nil {
		return err
	}
	c.serverPub = append([]byte(nil), serverPub...)
	c.serverRandom = append([]byte(nil), serverRandom...)
	c.sharedSecret = shared
	c.keys = keys
	return nil
}

// ClientConfirm 生成客户端确认码(发 KeyVerificationReq)。
func (c *ClientHandshake) ClientConfirm() []byte {
	return ComputeClientConfirm(c.keys, c.clientRandom, c.serverRandom, c.clientPub[:], c.serverPub)
}

// VerifyServer 校验服务端确认码。
func (c *ClientHandshake) VerifyServer(confirmMAC []byte) bool {
	expect := ComputeServerConfirm(c.keys, c.clientRandom, c.serverRandom, c.clientPub[:], c.serverPub)
	return VerifyConfirm(expect, confirmMAC)
}

// BuildClientChannel 创建客户端通道。
func (c *ClientHandshake) BuildClientChannel() (*SecureChannel, error) {
	return NewClientChannel(c.keys)
}

// Keys 取会话密钥。
func (c *ClientHandshake) Keys() *SessionKeys { return c.keys }
