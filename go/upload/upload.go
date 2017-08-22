package upload

import (
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"io"
	"net/http"
	"os"

	"go.skia.org/infra/go/sklog"
)

func UploadHandler(w http.ResponseWriter, r *http.Request) {
	md5Hash := md5.New()

	_, err := io.Copy(md5Hash, r.Body)
	if err != nil {
		sklog.Errorf("Copy error: %s", err)
		return
	}

	sklog.Infof("Received file: %x", md5Hash.Sum(nil))
}

func UploadFile(fileName, address string) (string, error) {
	file, err := os.Open(fileName)
	if err != nil {
		return "", err
	}
	defer file.Close()

	var buf bytes.Buffer
	_, err = io.Copy(&buf, file)
	if err != nil {
		return "", err
	}
	content := buf.Bytes()

	res, err := http.Post(address, "binary/octet-stream", bytes.NewBuffer(content))
	if err != nil {
		return "", err
	}
	defer res.Body.Close()

	md5Hash := md5.Sum(content)
	return hex.EncodeToString(md5Hash[:]), nil
}
