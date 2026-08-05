package gocrud

import (
	"bytes"
	crand "crypto/rand"
	"crypto/sha256"
	"io"
	"math/rand"
	"os"
	"path"
	"testing"

	"github.com/minio/sio"
	"golang.org/x/crypto/hkdf"
)

var masterKey = SHASum256FromString("123456")

func TestDare(t *testing.T) {
	plainData, err := NewRandomBytes(1024 + rand.Intn(1024))
	if err != nil {
		t.Fatal(err)
	}
	plainDataSize := int64(len(plainData))

	nonce := make([]byte, 32)
	if _, err := crand.Read(nonce); err != nil {
		t.Fatal(err)
	}

	key := make([]byte, 32)
	kdf := hkdf.New(sha256.New, masterKey, nonce, nil)
	_, err = io.ReadFull(kdf, key)
	if err != nil {
		t.Fatal(err)
	}

	config := sio.Config{Key: key}

	encryptedData := bytes.NewBuffer(nil)

	n, err := sio.Encrypt(encryptedData, bytes.NewReader(plainData), config)
	if err != nil {
		t.Fatal(err)
	} else if n < plainDataSize {
		t.Fatalf("%d < %d", n, plainDataSize)
	}

	if bytes.Compare(plainData, encryptedData.Bytes()) == 0 {
		t.Fatalf("expected not equal, but got equal")
	}

	decryptedData := bytes.NewBuffer(nil)
	nn, err := sio.Decrypt(decryptedData, encryptedData, config)
	if err != nil {
		t.Fatal(err)
	} else if nn != plainDataSize {
		t.Fatalf("expected %d bytes, got %d", plainDataSize, nn)
	}

	if !bytes.Equal(plainData, decryptedData.Bytes()) {
		t.Fatalf("decrypted data does not match original data")
	}
}

func TestSaveFileDarelly(t *testing.T) {
	plainData, err := NewRandomBytes(1024 + rand.Intn(1024))
	if err != nil {
		t.Fatal(err)
	}
	plainDataSize := int64(len(plainData))

	httpFile, err := SaveFileDarelly(bytes.NewReader(plainData), &SaveFileDarellyConfig{
		BaseFolder: TestDataDir,
		Length:     FileSize(plainDataSize),
		MasterKey:  masterKey,
		Validigest: FileDigest(HexedSHASum256(plainData)),
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Log(httpFile)

	filePath := path.Join(TestDataDir, string(httpFile.Filename))
	stat, err := os.Stat(filePath)
	if err != nil {
		t.Fatal(err)
	} else if stat.IsDir() {
		t.Fatalf("%s is a directory", filePath)
	}

	file, err := os.Open(filePath)
	if err != nil {
		t.Fatal(err)
	}

	decryptedData := bytes.NewBuffer(nil)
	n, err := sio.Decrypt(decryptedData, file, sio.Config{Key: httpFile.FileKey})
	if err != nil {
		t.Fatal(err)
	} else if n != plainDataSize {
		t.Fatalf("expected %d bytes, got %d", plainDataSize, n)
	} else if bytes.Compare(plainData, decryptedData.Bytes()) != 0 {
		t.Fatalf("decrypted data does not match original data")
	}
}
