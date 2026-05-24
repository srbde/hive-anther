package memo

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/binary"
	"strings"
	"testing"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/btcutil/base58"
	"github.com/btcsuite/btcd/chaincfg"
)

// generateWIFForTest creates a new WIF key for testing.
func generateWIFForTest(t *testing.T, seed byte) (string, string) {
	t.Helper()
	privBytes := make([]byte, 32)
	for i := range privBytes {
		privBytes[i] = seed
	}
	privKey, _ := btcec.PrivKeyFromBytes(privBytes)
	wif, err := btcutil.NewWIF(privKey, &chaincfg.MainNetParams, false)
	if err != nil {
		t.Fatalf("failed to create WIF: %v", err)
	}
	pubBytes := privKey.PubKey().SerializeCompressed()
	checksum := btcutil.Hash160(pubBytes)
	payload := append(pubBytes, checksum[:4]...)
	return wif.String(), "STM" + base58.Encode(payload)
}

func TestMemoEncodeDecode(t *testing.T) {
	senderWif, _ := generateWIFForTest(t, 0x11)
	receiverWif, receiverPub := generateWIFForTest(t, 0x22)

	text := "#hello nectar world! 爱"

	t.Run("Encrypt and decrypt by receiver", func(t *testing.T) {
		cypher, err := Encode(senderWif, receiverPub, text)
		if err != nil {
			t.Fatalf("failed to encode: %v", err)
		}

		if !strings.HasPrefix(cypher, "#") {
			t.Fatalf("encrypted memo should start with #: %s", cypher)
		}

		plain, err := Decode(receiverWif, cypher)
		if err != nil {
			t.Fatalf("failed to decode by receiver: %v", err)
		}

		if plain != text {
			t.Fatalf("expected %q, got %q", text, plain)
		}
	})

	t.Run("Encrypt and decrypt by sender", func(t *testing.T) {
		cypher, err := Encode(senderWif, receiverPub, text)
		if err != nil {
			t.Fatalf("failed to encode: %v", err)
		}

		plain, err := Decode(senderWif, cypher)
		if err != nil {
			t.Fatalf("failed to decode by sender: %v", err)
		}

		if plain != text {
			t.Fatalf("expected %q, got %q", text, plain)
		}
	})

	t.Run("Unprefixed memo pass through", func(t *testing.T) {
		plainText := "normal public memo text"
		res1, err := Encode(senderWif, receiverPub, plainText)
		if err != nil {
			t.Fatalf("unexpected encode error: %v", err)
		}
		if res1 != plainText {
			t.Fatalf("expected unprefixed memo to remain identical, got %q", res1)
		}

		res2, err := Decode(receiverWif, plainText)
		if err != nil {
			t.Fatalf("unexpected decode error: %v", err)
		}
		if res2 != plainText {
			t.Fatalf("expected unprefixed memo to remain identical on decode, got %q", res2)
		}
	})

	t.Run("Invalid private key decoding", func(t *testing.T) {
		cypher, err := Encode(senderWif, receiverPub, text)
		if err != nil {
			t.Fatalf("failed to encode: %v", err)
		}

		otherWif, _ := generateWIFForTest(t, 0x33)
		_, err = Decode(otherWif, cypher)
		if err == nil {
			t.Fatalf("expected error decoding with wrong private key")
		}
	})

	t.Run("Legacy memo fallback (no length prefix)", func(t *testing.T) {
		// Construct a legacy memo (just raw text encrypted in CBC, no varint string length prefix inside)
		wifDecoded, err := btcutil.DecodeWIF(senderWif)
		if err != nil {
			t.Fatalf("failed to decode sender wif: %v", err)
		}
		senderPriv := wifDecoded.PrivKey
		senderPubBytes := senderPriv.PubKey().SerializeCompressed()

		recPub, recPubBytes, err := parsePublicKey(receiverPub)
		if err != nil {
			t.Fatalf("failed to parse recipient pubkey: %v", err)
		}

		// Nonce & Secret
		nonce := uint64(987654321)
		sharedX := btcec.GenerateSharedSecret(senderPriv, recPub)
		S := sha512.Sum512(sharedX)

		buf := new(bytes.Buffer)
		binary.Write(buf, binary.LittleEndian, nonce)
		buf.Write(S[:])
		ebuf := buf.Bytes()

		encryptionKey := sha512.Sum512(ebuf)
		tag := encryptionKey[0:32]
		iv := encryptionKey[32:48]

		hash := sha256.Sum256(encryptionKey[:])
		check32 := binary.LittleEndian.Uint32(hash[0:4])

		// Legacy plaintext has NO varint length prefix, just raw memo text bytes
		legacyText := "legacy raw message without prefix"
		plaintext := []byte(legacyText)

		block, err := aes.NewCipher(tag)
		if err != nil {
			t.Fatalf("new cipher: %v", err)
		}
		padded := pkcs7Pad(plaintext, aes.BlockSize)
		ciphertext := make([]byte, len(padded))
		mode := cipher.NewCBCEncrypter(block, iv)
		mode.CryptBlocks(ciphertext, padded)

		// Envelope
		envelopeBuf := new(bytes.Buffer)
		envelopeBuf.Write(senderPubBytes)
		envelopeBuf.Write(recPubBytes)
		binary.Write(envelopeBuf, binary.LittleEndian, nonce)
		binary.Write(envelopeBuf, binary.LittleEndian, check32)

		varintBuf := make([]byte, binary.MaxVarintLen64)
		n := binary.PutUvarint(varintBuf, uint64(len(ciphertext)))
		envelopeBuf.Write(varintBuf[:n])
		envelopeBuf.Write(ciphertext)

		cypher := "#" + base58.Encode(envelopeBuf.Bytes())

		// Decode using receiverWif and verify fallback works
		plain, err := Decode(receiverWif, cypher)
		if err != nil {
			t.Fatalf("failed to decode legacy memo: %v", err)
		}

		expected := "#" + legacyText
		if plain != expected {
			t.Fatalf("expected fallback text %q, got %q", expected, plain)
		}
	})
}
