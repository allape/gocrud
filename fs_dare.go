package gocrud

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"path"
	"strings"

	"github.com/minio/sio"
	"golang.org/x/crypto/hkdf"
)

type SaveFileDarellyConfig struct {
	BaseFolder string        // will use cwd when empty
	Ext        string        // will use .bin when empty
	Length     FileSize      // will check length of dst file when not 0
	Validigest FileDigest    // will check file digest when not empty
	Nonce      FileNonce     // will modify the file digest of returning values
	MasterKey  FileMasterKey // will encrypt file
}

func SaveFileDarelly(source io.Reader, config *SaveFileDarellyConfig) (httpFile *HttpFile, err error) {
	var filename Filename
	var length FileSize
	var digest FileDigest
	var nonce FileNonce
	var noncedDigest FileNoncedDigest
	var fileKey FileKey

	if config == nil {
		config = &SaveFileDarellyConfig{}
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

	if config.Nonce != nil {
		if len(config.Nonce) != 32 {
			err = ErrorFileNonceMustBe32Bytes
			return
		}
		nonce = config.Nonce
	}

	if config.MasterKey != nil {
		if len(config.MasterKey) != 32 {
			err = ErrorFileMasterKeyMustBe32Bytes
			return
		}

		if config.Nonce == nil {
			config.Nonce = make([]byte, 32)
			_, err = io.ReadFull(rand.Reader, config.Nonce)
			if err != nil {
				return
			}
			nonce = config.Nonce
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

	tmpLength, err := io.Copy(mw, source)
	if err != nil {
		return
	}
	length = FileSize(tmpLength)
	if config.Length > 0 && length != config.Length {
		err = ErrorIncompleteWrite
		return
	}

	digest = FileDigest(strings.ToLower(hex.EncodeToString(hasher.Sum(nil))))
	if config.Validigest != "" && config.Validigest != digest {
		err = ErrorFileDigestMismatch
		return
	}

	if nonce == nil {
		noncedDigest = FileNoncedDigest(digest)
	} else {
		hasher.Write(nonce)
		noncedDigest = FileNoncedDigest(strings.ToLower(hex.EncodeToString(hasher.Sum(nil))))
	}

	strDigest := string(noncedDigest)

	filename = Filename(path.Join(
		"/",
		strDigest[:2],
		strDigest[2:4],
		strDigest+config.Ext,
	))

	fullPath := path.Join(config.BaseFolder, string(filename))
	stat, err := os.Stat(fullPath)
	if err != nil {
		if !os.IsNotExist(err) {
			// the same file + the same nonce, ignore this file even we are occurring a collision of SHA256
			return
		}
	} else if stat.IsDir() {
		err = ErrorFileIsDir
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

		// n should be larger than tmpLength,
		// sio.Encrypt should increase the file size ~0.05%
		if n < tmpLength {
			err = ErrorIncompleteWrite
			return
		}
	} else {
		n, err = io.Copy(dst, src)
		if err != nil {
			_ = os.Remove(fullPath)
			return
		}

		if tmpLength != n {
			_ = os.Remove(fullPath)
			err = ErrorIncompleteWrite
			return
		}
	}

	httpFile = &HttpFile{
		Filename:     filename,
		Length:       length,
		Digest:       digest,
		Nonce:        nonce,
		NoncedDigest: noncedDigest,
		FileKey:      fileKey,
	}

	return
}

type DareReader struct {
	io.ReadSeeker

	reader       io.ReaderAt
	currentIndex int64
	fileSize     int64
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
