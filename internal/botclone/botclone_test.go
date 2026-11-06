package botclone

import (
	"context"
	"errors"
	"strings"
	"testing"
)

const goodToken = "123456789:AAH-abcdefghijklmnopqrstuvwxyz012345"

func TestCipherRoundTrip(t *testing.T) {
	c, err := NewCipher("mother-secret")
	if err != nil {
		t.Fatal(err)
	}
	enc, err := c.Encrypt(goodToken)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(enc, goodToken) || strings.Contains(enc, "AAH-") {
		t.Fatalf("ciphertext leaks token: %s", enc)
	}
	dec, err := c.Decrypt(enc)
	if err != nil || dec != goodToken {
		t.Fatalf("round trip: %q %v", dec, err)
	}
	enc2, _ := c.Encrypt(goodToken)
	if enc2 == enc {
		t.Fatal("nonce must be random")
	}
	other, _ := NewCipher("other-secret")
	if _, err := other.Decrypt(enc); err == nil {
		t.Fatal("wrong key must fail")
	}
	if _, err := NewCipher(" "); err == nil {
		t.Fatal("empty key must fail")
	}
}

type fakeSaver struct {
	rows  map[string]Clone
	saves int
}

func (f *fakeSaver) Get(id string) (Clone, bool, error) { c, ok := f.rows[id]; return c, ok, nil }
func (f *fakeSaver) Save(c Clone, token string) (Clone, error) {
	f.saves++
	if f.rows == nil {
		f.rows = map[string]Clone{}
	}
	c.EncryptedToken = "enc"
	f.rows[c.BotID] = c
	return c, nil
}

func TestRegisterInvalidTokenNotStored(t *testing.T) {
	s := &fakeSaver{}
	failing := func(context.Context, string, string) (BotIdentity, error) {
		return BotIdentity{}, errors.New("401 Unauthorized")
	}
	for _, tok := range []string{goodToken, "not-a-token", ""} {
		_, err := Register(context.Background(), s, failing, Request{Token: tok, RequestedBy: "1"})
		if !errors.Is(err, ErrBadToken) {
			t.Fatalf("token %q: want ErrBadToken, got %v", tok, err)
		}
		if strings.Contains(err.Error(), goodToken) {
			t.Fatal("error echoes token")
		}
	}
	if s.saves != 0 {
		t.Fatalf("invalid token stored %d times", s.saves)
	}
}

func TestRegisterRules(t *testing.T) {
	ok := func(_ context.Context, base, _ string) (BotIdentity, error) {
		if base != DefaultAPIBase {
			return BotIdentity{}, errors.New("base " + base)
		}
		return BotIdentity{ID: "123456789", Username: "my_clone_bot"}, nil
	}
	s := &fakeSaver{}
	if _, err := Register(context.Background(), s, ok, Request{Token: goodToken, MotherBotID: "123456789"}); !errors.Is(err, ErrIsMother) {
		t.Fatalf("mother: %v", err)
	}
	if _, err := Register(context.Background(), s, ok, Request{Token: goodToken, IsReserved: func(string) bool { return true }}); !errors.Is(err, ErrAlreadyActive) {
		t.Fatalf("running: %v", err)
	}
	c, err := Register(context.Background(), s, ok, Request{Token: goodToken, ParentBotID: "999", MotherBotID: "999", RequestedBy: "42"})
	if err != nil || c.Status != StatusActive || c.APIBase != DefaultAPIBase || c.ParentBotID != "999" || c.RequestedBy != "42" {
		t.Fatalf("register: %+v %v", c, err)
	}
	if _, err := Register(context.Background(), s, ok, Request{Token: goodToken}); !errors.Is(err, ErrAlreadyActive) {
		t.Fatalf("duplicate active: %v", err)
	}
}
