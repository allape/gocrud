package gocrud

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"os"
	"path"
	"strings"
	"time"

	"github.com/minio/sio"
	"golang.org/x/crypto/hkdf"
)

type SaveDareFileConfig struct {
	BaseFolder     string        // will use cwd when empty
	Ext            string        // will use .bin when empty
	Size           FileSize      // will check the size of dst file when not 0
	Validigest     FileDigest    // will check file digest when not empty
	HashSalt       FileHashSalt  // salt for file hash
	MasterKey      FileMasterKey // encryption password for the file
	OnFileDigested func(digest FileDigest, saltyDigest FileSaltyDigest) (*HttpFile, error)
}

func SaveDareFile(source io.Reader, config *SaveDareFileConfig) (httpFile *HttpFile, err error) {
	var (
		name        FileName
		size        FileSize
		digest      FileDigest
		nonce       FileNonce
		saltyDigest FileSaltyDigest
		fileKey     FileKey
	)

	if config == nil {
		config = &SaveDareFileConfig{}
	}

	if config.BaseFolder == "" {
		config.BaseFolder = "."
	}

	if config.Ext == "" {
		config.Ext = ".bin"
	} else if !strings.HasPrefix(config.Ext, ".") {
		config.Ext = "." + config.Ext
	}

	if config.Validigest != "" {
		config.Validigest = FileDigest(strings.ToLower(string(config.Validigest)))
	}

	if config.MasterKey != nil {
		if len(config.MasterKey) != 32 {
			err = ErrorFileMasterKeyMustBe32Bytes
			return
		}

		nonce = make([]byte, 32)
		_, err = io.ReadFull(rand.Reader, nonce)
		if err != nil {
			return
		}

		fileKey = make([]byte, 32)
		kdf := hkdf.New(sha256.New, config.MasterKey, nonce, nil)
		_, err = io.ReadFull(kdf, fileKey)
		if err != nil {
			return
		}
	}

	tmpFile, err := os.CreateTemp(os.TempDir(), "gocrud-static-*.bin")
	if err != nil {
		return
	}
	defer func() {
		_ = tmpFile.Close()
		_ = os.Remove(tmpFile.Name())
	}()

	hasher := sha256.New()

	mw := io.MultiWriter(tmpFile, hasher)

	tmpSize, err := io.Copy(mw, source)
	if err != nil {
		return
	}
	size = FileSize(tmpSize)
	if config.Size > 0 && size != config.Size {
		err = ErrorIncompleteWrite
		return
	}

	digest = FileDigest(strings.ToLower(hex.EncodeToString(hasher.Sum(nil))))
	if config.Validigest != "" && config.Validigest != digest {
		err = ErrorFileDigestMismatch
		return
	}

	if len(config.HashSalt) == 0 {
		saltyDigest = FileSaltyDigest(digest)
	} else {
		hasher.Write(config.HashSalt)
		saltyDigest = FileSaltyDigest(strings.ToLower(hex.EncodeToString(hasher.Sum(nil))))
	}

	if config.OnFileDigested != nil {
		var exists *HttpFile
		exists, err = config.OnFileDigested(digest, saltyDigest)
		if err != nil {
			return
		}
		if exists != nil {
			httpFile = exists
			return
		}
	}

	strDigest := string(saltyDigest)

	name = FileName(path.Join(
		"/",
		strDigest[:2],
		strDigest[2:4],
		strDigest+config.Ext,
	))

	httpFile = &HttpFile{
		Name:        name,
		Size:        size,
		Digest:      digest,
		Nonce:       nonce,
		SaltyDigest: saltyDigest,
		FileKey:     fileKey,
	}

	fullPath := path.Join(config.BaseFolder, string(name))
	stat, err := os.Stat(fullPath)
	if err != nil {
		if !os.IsNotExist(err) {
			return
		}
	} else {
		if stat.IsDir() {
			err = ErrorFileIsDir
			return
		}

		// it is impossible to occur the same hash file after OnFileDigested check
		return
	}

	parentDir := path.Dir(fullPath)
	if _, err = os.Stat(parentDir); err != nil {
		if os.IsNotExist(err) {
			err = os.MkdirAll(parentDir, os.ModePerm)
			if err != nil {
				return
			}
		} else {
			return
		}
	}

	file, err := os.Create(fullPath)
	if err != nil {
		return
	}
	defer func() {
		_ = file.Close()
	}()

	_, err = tmpFile.Seek(0, io.SeekStart)
	if err != nil {
		return
	}

	var n int64
	var src io.Reader = tmpFile
	var dst io.Writer = file

	if fileKey != nil {
		n, err = sio.Encrypt(dst, src, sio.Config{Key: fileKey})
		if err != nil {
			_ = os.Remove(fullPath)
			return
		}

		// n should be larger than tmpSize,
		// sio.Encrypt should increase the file size ~0.05%
		if n < tmpSize {
			err = ErrorIncompleteWrite
			return
		}
	} else {
		n, err = io.Copy(dst, src)
		if err != nil {
			_ = os.Remove(fullPath)
			return
		}

		if tmpSize != n {
			_ = os.Remove(fullPath)
			err = ErrorIncompleteWrite
			return
		}
	}

	return
}

type DareReader struct {
	io.ReadSeeker
	io.ReaderAt
	io.Closer

	reader       io.ReaderAt
	currentIndex int64
	fileSize     int64
}

func (d *DareReader) Close() error {
	if closer, ok := d.reader.(io.Closer); ok {
		return closer.Close()
	}
	return nil
}

func (d *DareReader) Read(p []byte) (n int, err error) {
	n, err = d.reader.ReadAt(p, d.currentIndex)
	d.currentIndex += int64(n)
	return
}

func (d *DareReader) Seek(offset int64, whence int) (int64, error) {
	switch whence {
	case io.SeekStart:
		d.currentIndex = offset
	case io.SeekCurrent:
		d.currentIndex += offset
	case io.SeekEnd:
		d.currentIndex = d.fileSize - offset
	}
	return d.currentIndex, nil
}

func (d *DareReader) ReadAt(p []byte, off int64) (n int, err error) {
	return d.reader.ReadAt(p, off)
}

func NewDareReader(src io.ReaderAt, fileSize FileSize, fileKey FileKey) (*DareReader, error) {
	dst, err := sio.DecryptReaderAt(src, sio.Config{Key: fileKey})
	if err != nil {
		return nil, err
	}

	return &DareReader{
		currentIndex: 0,
		fileSize:     int64(fileSize),
		reader:       dst,
	}, nil
}

func NewDareHttpServeFunc(file io.ReaderAt, httpFile *HttpFile) (http.HandlerFunc, error) {
	reader, err := NewDareReader(file, httpFile.Size, httpFile.FileKey)
	if err != nil {
		return nil, err
	}

	return func(writer http.ResponseWriter, request *http.Request) {
		defer func() {
			_ = reader.Close()
		}()
		http.ServeContent(writer, request, path.Base(string(httpFile.Name)), time.UnixMilli(0), reader)
	}, nil
}
