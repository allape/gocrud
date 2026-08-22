package gocrud

import (
	"bytes"
	crand "crypto/rand"
	"crypto/sha256"
	"errors"
	"io"
	"math/rand"
	"os"
	"path"
	"testing"

	"github.com/minio/sio"
	"golang.org/x/crypto/hkdf"
)

var masterKey = SHASum256FromString("123456")
var hashSalt FileHashSalt = []byte{1, 2, 3, 4, 5, 6, 7, 8, 9}

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

func TestSaveDareFile(t *testing.T) {
	plainData, err := NewRandomBytes(1024 + rand.Intn(1024))
	if err != nil {
		t.Fatal(err)
	}
	plainDataSize := int64(len(plainData))

	_, err = SaveDareFile(bytes.NewReader(plainData), &SaveDareFileConfig{
		BaseFolder: TestDataDir,
		Size:       FileSize(plainDataSize + 1),
		Validigest: FileDigest(HexedSHASum256(plainData)),
	})
	if !errors.Is(err, ErrorIncompleteWrite) {
		t.Fatalf("expected ErrorIncompleteWrite, but got %v", err)
	}

	_, err = SaveDareFile(bytes.NewReader(plainData), &SaveDareFileConfig{
		BaseFolder: TestDataDir,
		Size:       FileSize(plainDataSize),
		Validigest: FileDigest(HexedSHASum256(append(plainData, 1, 2, 3))),
	})
	if !errors.Is(err, ErrorFileDigestMismatch) {
		t.Fatalf("expected ErrorFileDigestMismatch, but got %v", err)
	}

	correctHash := HexedSHASum256(plainData)

	httpFile, err := SaveDareFile(bytes.NewReader(plainData), &SaveDareFileConfig{
		BaseFolder: TestDataDir,
		Size:       FileSize(plainDataSize),
		Validigest: FileDigest(correctHash),
	})
	if err != nil {
		t.Fatal(err)
	} else if string(httpFile.Digest) != correctHash {
		t.Fatalf("expected %s, but got %s", correctHash, httpFile.Digest)
	} else if string(httpFile.Digest) != string(httpFile.SaltyDigest) {
		t.Fatalf("expected %s, but got %s", string(httpFile.Digest), string(httpFile.SaltyDigest))
	}

	if ok, err := compareFileBytes(path.Join(TestDataDir, string(httpFile.Name)), plainData); err != nil {
		t.Fatal(err)
	} else if !ok {
		t.Fatalf("local file not equal")
	}

	saltyDigest := HexedSHASum256(append(plainData, hashSalt...))

	httpFile1, err := SaveDareFile(bytes.NewReader(plainData), &SaveDareFileConfig{
		BaseFolder: TestDataDir,
		Size:       FileSize(plainDataSize),
		Validigest: FileDigest(correctHash),
		HashSalt:   hashSalt,
	})
	if err != nil {
		t.Fatal(err)
	} else if string(httpFile1.Digest) == string(httpFile1.SaltyDigest) {
		t.Fatalf("digest and salty digests are the same")
	} else if string(httpFile1.SaltyDigest) != saltyDigest {
		t.Fatalf("expected %s, but got %s", saltyDigest, httpFile1.SaltyDigest)
	}
}

func TestSaveFileDarelly(t *testing.T) {
	plainData, err := NewRandomBytes(1024 + rand.Intn(1024))
	if err != nil {
		t.Fatal(err)
	}
	plainDataSize := int64(len(plainData))

	httpFile, err := SaveDareFile(bytes.NewReader(plainData), &SaveDareFileConfig{
		BaseFolder: TestDataDir,
		Size:       FileSize(plainDataSize),
		MasterKey:  masterKey,
		HashSalt:   hashSalt,
		Validigest: FileDigest(HexedSHASum256(plainData)),
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Log(httpFile)

	if string(httpFile.Digest) == string(httpFile.SaltyDigest) {
		t.Fatalf("digests are the same")
	}

	filePath := path.Join(TestDataDir, string(httpFile.Name))
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
