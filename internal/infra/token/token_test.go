package token

import (
	"strings"
	"testing"
)

func TestGenerate(t *testing.T) {
	plain, hash, err := Generate()
	if err != nil {
		t.Fatal(err)
	}

	if len(plain) != TokenByteLength*2 {
		t.Errorf("plaintext length = %d, want %d", len(plain), TokenByteLength*2)
	}
	if len(hash) != 64 {
		t.Errorf("hash length = %d, want 64", len(hash))
	}
	if strings.EqualFold(plain, hash) {
		t.Error("plaintext should differ from hash")
	}

	// 生成のたびに違う値が出ること
	plain2, _, _ := Generate()
	if plain == plain2 {
		t.Error("two generations returned same plaintext")
	}
}

func TestHash_Deterministic(t *testing.T) {
	h1 := Hash("example")
	h2 := Hash("example")
	if h1 != h2 {
		t.Errorf("Hash is non-deterministic: %s vs %s", h1, h2)
	}

	h3 := Hash("Example")
	if h1 == h3 {
		t.Error("Hash should be case-sensitive")
	}
}

func TestGenerate_HashMatches(t *testing.T) {
	plain, hash, _ := Generate()
	if Hash(plain) != hash {
		t.Error("Generated hash does not match Hash(plaintext)")
	}
}
