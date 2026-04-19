// Package token はデバイスAPI用 Bearer トークンの生成・ハッシュ化ユーティリティ。
//
// セキュリティ方針:
//   - 平文トークンは発行時にのみ表示し、DB には SHA-256 ハッシュのみ保存する
//   - 平文トークンが漏洩しても DB からは逆算できない
//   - 照合時もリクエストの平文を SHA-256 してから DB と突合する
package token

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
)

// TokenByteLength は生成する乱数トークンのバイト長 (32B = 256bit)。
// hex エンコード後は 64 文字になり device_tokens.token_hash の VARCHAR(64) に収まる。
const TokenByteLength = 32

// Generate は暗号学的に安全な乱数トークンを生成する。
// 戻り値: 平文トークン (hex 文字列), ハッシュ (SHA-256 hex), error
func Generate() (plaintext, hash string, err error) {
	buf := make([]byte, TokenByteLength)
	if _, err := rand.Read(buf); err != nil {
		return "", "", fmt.Errorf("rand.Read: %w", err)
	}
	plaintext = hex.EncodeToString(buf)
	hash = Hash(plaintext)
	return plaintext, hash, nil
}

// Hash は平文トークンの SHA-256 ハッシュを hex 文字列で返す。
// 照合時に受信した Bearer トークンをこの関数にかけ、DB 保存値と比較する。
func Hash(plaintext string) string {
	sum := sha256.Sum256([]byte(plaintext))
	return hex.EncodeToString(sum[:])
}
