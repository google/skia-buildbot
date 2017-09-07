package upload

import (
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"io"
	"io/ioutil"
	"net/http"
	"os"

	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
)

func UploadHandler(w http.ResponseWriter, r *http.Request) {

	f, err := ioutil.TempFile("./results", "img-")
	if err != nil {
		sklog.Errorf("Error opening temp files: %s", err)
		return
	}
	defer util.Close(f)

	md5Hash := md5.New()
	multiWriter := io.MultiWriter(f, md5Hash)
	_, err = io.Copy(multiWriter, r.Body)
	if err != nil {
		sklog.Errorf("Copy error: %s", err)
		return
	}

	// if err := r.ParseMultipartForm(1024 * 1024 * 50); err != nil {
	// 	sklog.Errorf("Error parsing multipart form: %s", err)
	// 	return
	// }

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
