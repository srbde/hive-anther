// Package memo provides support for private message (memo) encryption and decryption using ECIES (Elliptic Curve Integrated Encryption Scheme) on the secp256k1 curve.
package memo

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/decred/dcrd/dcrec/secp256k1/v4"
	"github.com/srbde/hive-anther/crypto"
)

// pkcs7Pad appends PKCS#7 padding to data.
func pkcs7Pad(data []byte, blockSize int) []byte {
	padding := blockSize - (len(data) % blockSize)
	padText := bytes.Repeat([]byte{byte(padding)}, padding)
	return append(data, padText...)
}

// pkcs7Unpad removes PKCS#7 padding from data.
func pkcs7Unpad(data []byte, blockSize int) ([]byte, error) {
	length := len(data)
	if length == 0 {
		return nil, errors.New("empty data")
	}
	if length%blockSize != 0 {
		return nil, errors.New("data is not a multiple of block size")
	}
	padding := int(data[length-1])
	if padding < 1 || padding > blockSize {
		return nil, errors.New("invalid padding size")
	}
	for i := length - padding; i < length; i++ {
		if int(data[i]) != padding {
			return nil, errors.New("invalid padding bytes")
		}
	}
	return data[:length-padding], nil
}

// parsePublicKey decodes a Hive public key string (STM... or TST...) into secp256k1.PublicKey.
func parsePublicKey(pubKeyStr string) (*secp256k1.PublicKey, []byte, error) {
	trimmed := pubKeyStr
	if len(pubKeyStr) > 3 && (pubKeyStr[:3] == "STM" || pubKeyStr[:3] == "TST") {
		trimmed = pubKeyStr[3:]
	}
	decoded := crypto.Base58Decode(trimmed)
	if len(decoded) < 33 {
		return nil, nil, fmt.Errorf("invalid public key length: %d", len(decoded))
	}
	rawPub := decoded[:33]
	pubKey, err := secp256k1.ParsePubKey(rawPub)
	if err != nil {
		return nil, nil, err
	}
	return pubKey, rawPub, nil
}

// generateNonce generates a random uint64 nonce.
func generateNonce() (uint64, error) {
	var n uint64
	err := binary.Read(rand.Reader, binary.LittleEndian, &n)
	return n, err
}

// Encode encrypts a memo if it starts with "#".
func Encode(senderWif string, recipientPubKeyStr string, memo string) (string, error) {
	if !strings.HasPrefix(memo, "#") {
		return memo, nil
	}
	memoText := memo[1:]

	// Decode WIF
	privKeyBytes, err := crypto.DecodeWIF(senderWif)
	if err != nil {
		return "", fmt.Errorf("failed to decode WIF: %w", err)
	}
	senderPrivKey := secp256k1.PrivKeyFromBytes(privKeyBytes)
	senderPubBytes := senderPrivKey.PubKey().SerializeCompressed()

	// Decode recipient public key
	recipientPubKey, recipientPubBytes, err := parsePublicKey(recipientPubKeyStr)
	if err != nil {
		return "", fmt.Errorf("failed to parse recipient public key: %w", err)
	}

	// Generate Nonce
	nonce, err := generateNonce()
	if err != nil {
		return "", fmt.Errorf("failed to generate nonce: %w", err)
	}

	// Derive shared secret S
	sharedX := secp256k1.GenerateSharedSecret(senderPrivKey, recipientPubKey)
	S := sha512.Sum512(sharedX)

	// Derive encryption key materials
	buf := new(bytes.Buffer)
	if err := binary.Write(buf, binary.LittleEndian, nonce); err != nil {
		return "", err
	}
	buf.Write(S[:])
	ebuf := buf.Bytes()

	encryptionKey := sha512.Sum512(ebuf)
	tag := encryptionKey[0:32]
	iv := encryptionKey[32:48]

	// Compute checksum
	hash := sha256.Sum256(encryptionKey[:])
	check32 := binary.LittleEndian.Uint32(hash[0:4])

	// Prepare plaintext with varint length prefix
	plainBuf := new(bytes.Buffer)
	varintBuf := make([]byte, binary.MaxVarintLen64)
	n := binary.PutUvarint(varintBuf, uint64(len(memoText)))
	plainBuf.Write(varintBuf[:n])
	plainBuf.Write([]byte(memoText))
	plaintext := plainBuf.Bytes()

	// Encrypt
	block, err := aes.NewCipher(tag)
	if err != nil {
		return "", err
	}
	padded := pkcs7Pad(plaintext, aes.BlockSize)
	ciphertext := make([]byte, len(padded))
	mode := cipher.NewCBCEncrypter(block, iv)
	mode.CryptBlocks(ciphertext, padded)

	// Build envelope
	envelopeBuf := new(bytes.Buffer)
	envelopeBuf.Write(senderPubBytes)
	envelopeBuf.Write(recipientPubBytes)
	if err := binary.Write(envelopeBuf, binary.LittleEndian, nonce); err != nil {
		return "", err
	}
	if err := binary.Write(envelopeBuf, binary.LittleEndian, check32); err != nil {
		return "", err
	}

	n = binary.PutUvarint(varintBuf, uint64(len(ciphertext)))
	envelopeBuf.Write(varintBuf[:n])
	envelopeBuf.Write(ciphertext)

	return "#" + crypto.Base58Encode(envelopeBuf.Bytes()), nil
}

// Decode decrypts a memo if it starts with "#".
func Decode(wif string, memo string) (string, error) {
	if !strings.HasPrefix(memo, "#") {
		return memo, nil
	}
	memoBase58 := memo[1:]

	decoded := crypto.Base58Decode(memoBase58)
	if len(decoded) < 33+33+8+4 {
		return "", errors.New("invalid encrypted memo payload length")
	}

	// Parse fields
	reader := bytes.NewReader(decoded)
	fromBytes := make([]byte, 33)
	if _, err := io.ReadFull(reader, fromBytes); err != nil {
		return "", fmt.Errorf("failed to read fromBytes: %w", err)
	}
	toBytes := make([]byte, 33)
	if _, err := io.ReadFull(reader, toBytes); err != nil {
		return "", fmt.Errorf("failed to read toBytes: %w", err)
	}
	var nonce uint64
	if err := binary.Read(reader, binary.LittleEndian, &nonce); err != nil {
		return "", fmt.Errorf("failed to read nonce: %w", err)
	}
	var check32 uint32
	if err := binary.Read(reader, binary.LittleEndian, &check32); err != nil {
		return "", fmt.Errorf("failed to read check32: %w", err)
	}

	cipherLen, err := binary.ReadUvarint(reader)
	if err != nil {
		return "", fmt.Errorf("failed to read ciphertext length: %w", err)
	}
	ciphertext := make([]byte, cipherLen)
	if _, err := io.ReadFull(reader, ciphertext); err != nil {
		return "", fmt.Errorf("failed to read ciphertext: %w", err)
	}

	// Decode WIF
	privKeyBytes, err := crypto.DecodeWIF(wif)
	if err != nil {
		return "", fmt.Errorf("failed to decode WIF: %w", err)
	}
	myPrivKey := secp256k1.PrivKeyFromBytes(privKeyBytes)
	myPubBytes := myPrivKey.PubKey().SerializeCompressed()

	// Find the other public key
	var otherPubBytes []byte
	if bytes.Equal(myPubBytes, fromBytes) {
		otherPubBytes = toBytes
	} else {
		otherPubBytes = fromBytes
	}

	otherPubKey, err := secp256k1.ParsePubKey(otherPubBytes)
	if err != nil {
		return "", fmt.Errorf("failed to parse counterparty public key: %w", err)
	}

	// Derive shared secret
	sharedX := secp256k1.GenerateSharedSecret(myPrivKey, otherPubKey)
	S := sha512.Sum512(sharedX)

	// Derive key materials
	buf := new(bytes.Buffer)
	if err := binary.Write(buf, binary.LittleEndian, nonce); err != nil {
		return "", err
	}
	buf.Write(S[:])
	ebuf := buf.Bytes()

	encryptionKey := sha512.Sum512(ebuf)
	tag := encryptionKey[0:32]
	iv := encryptionKey[32:48]

	// Verify checksum
	hash := sha256.Sum256(encryptionKey[:])
	expectedCheck32 := binary.LittleEndian.Uint32(hash[0:4])
	if expectedCheck32 != check32 {
		return "", errors.New("invalid key or checksum mismatch")
	}

	// Decrypt
	block, err := aes.NewCipher(tag)
	if err != nil {
		return "", err
	}
	mode := cipher.NewCBCDecrypter(block, iv)
	paddedPlaintext := make([]byte, len(ciphertext))
	mode.CryptBlocks(paddedPlaintext, ciphertext)

	plaintext, err := pkcs7Unpad(paddedPlaintext, aes.BlockSize)
	if err != nil {
		return "", fmt.Errorf("failed to unpad plaintext: %w", err)
	}

	// Parse string length prefix
	plainReader := bytes.NewReader(plaintext)
	memoLen, err := binary.ReadUvarint(plainReader)
	if err == nil && int(memoLen) <= plainReader.Len() {
		memoBytes := make([]byte, memoLen)
		n, err := plainReader.Read(memoBytes)
		if err == nil && n == int(memoLen) {
			return "#" + string(memoBytes), nil
		}
	}

	// Fallback to raw string
	return "#" + string(plaintext), nil
}
