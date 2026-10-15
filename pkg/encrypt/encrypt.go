package encrypt

// Encrypt 加解密接口
// 注意顺序: 加密时先压缩后加密, 解密时先解密后解压缩
type Encrypt interface {
	// Encrypt 加密
	Encrypt(in []byte) ([]byte, error)

	// Decrypt 解密
	Decrypt(in []byte) ([]byte, error)

	// Name 获取加解密方式名称
	Name() string
}
