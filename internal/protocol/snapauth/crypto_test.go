package snapauth

import (
	"bytes"
	"encoding/json"
	"os"
	"testing"
)

func TestExistingPythonBootstrapVectors(t *testing.T) {
	raw, err := os.ReadFile("testdata/python-reference.json")
	if err != nil {
		t.Fatal(err)
	}
	var v struct {
		Seed, Secret                      string
		Magic, Challenge, Answer, Success []byte
		Port                              uint16
	}
	if err := json.Unmarshal(raw, &v); err != nil {
		t.Fatal(err)
	}
	challenge, err := Challenge(v.Magic, v.Seed, v.Secret, v.Port)
	if err != nil || !bytes.Equal(challenge, v.Challenge) {
		t.Fatal("challenge differs from existing bootstrap", err)
	}
	if !VerifyAnswer(v.Answer, v.Secret) || VerifyAnswer(v.Answer, "different-proof") {
		t.Fatal("native response proof mismatch")
	}
	success, err := Success("FixtureName", "203.0.113.7", 9092)
	if err != nil || !bytes.Equal(success, v.Success) {
		t.Fatal("destination differs from existing bootstrap", err)
	}
	if _, err := Decrypt(BootstrapKey, []byte{1, 2}); err == nil {
		t.Fatal("partial encrypted block accepted")
	}
	if _, err := Challenge(v.Magic[:19], v.Seed, v.Secret, v.Port); err == nil {
		t.Fatal("invalid account key accepted")
	}
	if _, err := Success("FixtureName", "0.0.0.0", 0); err == nil {
		t.Fatal("invalid game destination accepted")
	}
}
