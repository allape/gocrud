package gocrud

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"github.com/gin-gonic/gin"
	"io"
	"net/http"
	"os"
	"path"
	"strings"
)

const (
	XFileDigest = "X-File-Digest"
)

var (
	ErrorIncompleteWrite    = errors.New("incomplete write")
	ErrorFileExists         = errors.New("file already exists")
	ErrorFileIsDir          = errors.New("file is a directory")
	ErrorUploadNotAllowed   = errors.New("upload not allowed")
	ErrorFileDigestMismatch = errors.New("digest mismatch")
)

type HttpFileSystemConfig struct {
	AllowUpload    bool
	AllowOverwrite bool
	EnableDigest   bool // EnableDigest: if true, will save file with its digest, and discard client defined filename
	Coder          Coder
}

type (
	Filename   string
	FileDigest string
)

func SaveAsDigestedFile(
	folder string, // base folder
	filename string, // for extracting file extension
	reader io.Reader, // file content
	length int64, // leave it 0 to skip length check
	validigest FileDigest, // validation digest, leave it empty to skip validation
) (Filename, FileDigest, error) {
	tmpFile, err := os.CreateTemp(os.TempDir(), "gocrud-static-*.bin")
	if err != nil {
		return "", "", err
	}
	defer func() {
		_ = tmpFile.Close()
		_ = os.Remove(tmpFile.Name())
	}()

	hasher := sha256.New()

	mw := io.MultiWriter(tmpFile, hasher)

	n, err := io.Copy(mw, reader)
	if err != nil {
		return "", "", err
	} else if length > 0 && n != length {
		return "", "", ErrorIncompleteWrite
	}

	digest := hex.EncodeToString(hasher.Sum(nil))
	if validigest != "" && strings.ToLower(string(validigest)) != digest {
		return "", "", ErrorFileDigestMismatch
	}

	filename = path.Join(
		"/",
		digest[:2],
		digest[2:4],
		digest+path.Ext(filename),
	)

	fullpath := path.Join(folder, filename)

	stat, err := os.Stat(fullpath)
	if err != nil {
		if !os.IsNotExist(err) {
			//return "", ErrorFileExists
			return Filename(filename), "", nil
		}
	} else if stat.IsDir() {
		return "", "", ErrorFileIsDir
	}

	basepath := path.Dir(fullpath)

	if _, err = os.Stat(basepath); err != nil {
		if os.IsNotExist(err) {
			err = os.MkdirAll(basepath, os.ModePerm)
			if err != nil {
				return "", "", err
			}
		} else {
			return "", "", err
		}
	}

	file, err := os.Create(fullpath)
	if err != nil {
		return "", "", err
	}
	defer func() {
		_ = file.Close()
	}()

	_, err = tmpFile.Seek(0, io.SeekStart)
	if err != nil {
		return "", "", err
	}

	nn, err := io.Copy(file, tmpFile)
	if err != nil {
		_ = os.Remove(fullpath)
		return "", "", err
	} else if n != nn {
		_ = os.Remove(fullpath)
		return "", "", ErrorIncompleteWrite
	}

	return Filename(filename), FileDigest(digest), nil
}

func NewHttpFileSystem(group *gin.RouterGroup, folder string, config *HttpFileSystemConfig) error {
	if config == nil {
		config = &HttpFileSystemConfig{Coder: RestCoder}
	}

	if config.Coder == nil {
		config.Coder = RestCoder
	}

	group.Static("", folder)

	group.POST("/*filepath", func(context *gin.Context) {
		if !config.AllowUpload {
			MakeErrorResponse(context, config.Coder.MethodNotAllowed(), ErrorUploadNotAllowed)
			return
		}

		var err error
		var filename string
		if config.EnableDigest {
			var saved Filename
			saved, _, err = SaveAsDigestedFile(
				folder,
				context.Param("filepath"),
				context.Request.Body,
				context.Request.ContentLength,
				FileDigest(context.GetHeader(XFileDigest)),
			)
			if err != nil {
				MakeErrorResponse(context, config.Coder.InternalServerError(), err)
				return
			}
			filename = string(saved)
		} else {
			filename = context.Param("filepath")
			fullpath := path.Join(folder, filename)
			basepath := path.Dir(fullpath)

			stat, err := os.Stat(fullpath)
			if err == nil {
				if !config.AllowOverwrite {
					//MakeErrorResponse(context, config.Coder.Conflict(), ErrorFileExists)
					context.JSON(http.StatusOK, R[any]{
						Code:    config.Coder.Conflict(),
						Message: ErrorFileExists.Error(),
						Data:    filename,
					})
					return
				}
				if stat.IsDir() {
					MakeErrorResponse(context, config.Coder.InternalServerError(), ErrorFileIsDir)
					return
				}
			} else {
				if !os.IsNotExist(err) {
					MakeErrorResponse(context, config.Coder.InternalServerError(), err)
					return
				}
			}

			if _, err := os.Stat(basepath); err != nil {
				if os.IsNotExist(err) {
					err = os.MkdirAll(basepath, os.ModePerm)
					if err != nil {
						MakeErrorResponse(context, config.Coder.InternalServerError(), err)
						return
					}
				} else {
					MakeErrorResponse(context, config.Coder.InternalServerError(), err)
					return
				}
			}

			file, err := os.Create(fullpath)
			if err != nil {
				MakeErrorResponse(context, config.Coder.InternalServerError(), err)
				return
			}
			defer func() {
				_ = file.Close()
			}()

			n, err := io.Copy(file, context.Request.Body)
			if err != nil {
				_ = os.Remove(fullpath)
				MakeErrorResponse(context, config.Coder.InternalServerError(), err)
				return
			}

			contextLength := context.Request.ContentLength
			if contextLength > 0 && n != contextLength {
				_ = os.Remove(fullpath)
				MakeErrorResponse(context, config.Coder.InternalServerError(), ErrorIncompleteWrite)
				return
			}
		}

		context.JSON(http.StatusOK, R[any]{
			Code: config.Coder.OK(),
			Data: filename,
		})
	})

	return nil
}
