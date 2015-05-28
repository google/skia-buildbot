package main

import (
	"bytes"
	"crypto/md5"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"text/template"

	"code.google.com/p/google-api-go-client/storage/v1"
	"github.com/skia-dev/glog"
	"go.skia.org/infra/fuzzer/go/config"
	"go.skia.org/infra/fuzzer/go/generator"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/util"
)

import (
	_ "go.skia.org/infra/fuzzer/go/generator/dummy"
	_ "go.skia.org/infra/fuzzer/go/generator/paths"
)

var (
	configFilename                    = flag.String("config", "fuzzer.toml", "Configuration filename")
	codeTemplate   *template.Template = nil
	gypTemplate    *template.Template = nil
	client         *http.Client       = nil
	store          *storage.Service   = nil
)

func setDefaults() {
	config.Config.Fuzzer.Indentation = 2
}

// setup does some app-wide initialization, initia and returns the path to the
// resources directory
func setup() (string, error) {
	if config.Config.Common.ResourcePath == "" {
		_, filename, _, _ := runtime.Caller(0)
		config.Config.Common.ResourcePath = filepath.Join(filepath.Dir(filename), "../..")
	}

	path, err := filepath.Abs(config.Config.Common.ResourcePath)
	if err != nil {
		return path, fmt.Errorf("Couldn't get abs path for %s: %s", config.Config.Common.ResourcePath, err)
	}
	if err := os.Chdir(path); err != nil {
		return path, fmt.Errorf("Couldn't change to directory %s: %s", path, err)

	}
	gypTemplate = template.Must(template.ParseFiles(filepath.Join(path, "templates/template.gyp")))
	return path, nil
}

// writeTemplate creates a given output file and writes the template
// result there.
func writeTemplate(filename string, t *template.Template, context interface{}) error {
	f, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("Couldn't create file %s: %s", filename, err)
	}
	defer util.Close(f)
	return t.Execute(f, context)
}

type CppTemplateContext struct {
	Hash         string
	ResourcePath string
}

func writeFuzz(code string) (string, error) {
	h := md5.New()
	_, err := h.Write([]byte(code))
	if err != nil {
		return "", fmt.Errorf("Couldn't make an md5 of the code [this should never happen]: %s", err)
	}
	hash := fmt.Sprintf("%x", h.Sum(nil))
	err = writeTemplate(fmt.Sprintf(filepath.Join(config.Config.Fuzzer.CachePath, "src/%s.cpp"), hash),
		codeTemplate,
		struct{ Code string }{code})
	if err != nil {
		return hash, fmt.Errorf("Coudln't write CPP template: %s", err)
	}

	err = writeTemplate(fmt.Sprintf(filepath.Join(config.Config.Fuzzer.CachePath, "%s.gyp"), hash),
		gypTemplate,
		CppTemplateContext{hash, config.Config.Common.ResourcePath})
	if err != nil {
		return hash, fmt.Errorf("Coudln't write GYP template: %s", err)
	}

	return hash, err
}

// createCodeTemplate builds the .cpp template that we will build each fuzz into.  We
// search the Skia source directory for any public include files and add them to the output.
func createCodeTemplate(outputPath string) {
	includeDirs := []string{"core", "effects", "pathops"}
	includeFiles := []string{}

	for _, dir := range includeDirs {
		includePath := filepath.Join(config.Config.Fuzzer.SkiaSourceDir, "include", dir)
		infos, err := ioutil.ReadDir(includePath)
		if err != nil {
			glog.Fatalf("Couldn't read include dir: %s", err)
		}
		for _, info := range infos {
			includeFiles = append(includeFiles, info.Name())
		}
	}

	sort.Strings(includeFiles)

	out, err := os.Create(outputPath)
	if err != nil {
		glog.Fatalf("Couldn't create code template: %s", err)
	}
	defer util.Close(out)

	for _, filename := range includeFiles {
		_, err = out.WriteString(fmt.Sprintf("#include \"%s\"\n", filename))
		if err != nil {
			glog.Fatalf("Couldn't write to the code template: %s", err)
		}
	}
	_, err = out.WriteString(`#include "sk_tool_utils.h"
#include "SkCommandLineFlags.h"

SkBitmap source;
void draw(SkCanvas* canvas) {
{{.Code}}
}
`)
	if err != nil {
		glog.Fatalf("Couldn't write to the code template: %s", err)
	}
}

// checkCPPTemplate checks for the existence of the CPP template that each fuzz will be
// build against, and creates it if it's not there.
func checkCPPTemplate(path string) {
	templatePath := filepath.Join(path, "templates/template.cpp")

	if _, err := os.Stat(templatePath); err != nil {
		createCodeTemplate(templatePath)
	}
	codeTemplate = template.Must(template.ParseFiles(templatePath))
}

func runFuzz(hash string) (int, error) {
	cacheDir := config.Config.Fuzzer.CachePath
	skiaDir := config.Config.Fuzzer.SkiaSourceDir

	err := os.Chdir(skiaDir)
	if err != nil {
		glog.Fatalf("Couldn't change to the skia dir %s: %s", skiaDir, err)
	}

	gypFilename := fmt.Sprintf("%s.gyp", hash)

	glog.Infof("Moving %s to %s", filepath.Join(cacheDir, gypFilename), filepath.Join(skiaDir, "gyp", gypFilename))
	outPath := filepath.Join(skiaDir, "gyp", gypFilename)
	err = os.Rename(filepath.Join(cacheDir, gypFilename), outPath)
	if err != nil {
		glog.Fatalf("Couldn't copy the generated gyp file to %s: %s", outPath, err)
	}
	glog.Infof("Running gyp for %s...", hash)

	cmd := fmt.Sprintf("./gyp_skia gyp/%s.gyp gyp/most.gyp -Dskia_mesa=1", hash)
	message, err := util.DoCmd(cmd)
	if err != nil {
		glog.Fatalf("Couldn't run gyp: %s", err)
	}

	glog.Infof("Running ninja for %s...", hash)

	cmd = fmt.Sprintf("ninja -C %s/out/Release_Developer %s", skiaDir, hash)
	message, err = util.DoCmd(cmd)
	if err != nil {
		glog.Fatalf("Couldn't run ninja: %s", err)
	}

	cmd = fmt.Sprintf("%s/out/Release_Developer/%s --out %s/%s", skiaDir, hash, cacheDir, hash)
	message, err = util.DoCmd(cmd)
	if err != nil {
		glog.Fatalf("Couldn't run fuzz: %s", err)
	}

	glog.Infof(message)

	return 0, nil
}

func uploadFuzz(hash, code string, status int) error {
	glog.Infof("Uploading fuzz %s to GS bucket %s.", hash, config.Config.Common.FuzzOutputGSBucket)

	parent := "working"

	if status != 0 {
		parent = "failed"
	}

	buf := bytes.NewBufferString(code)
	req := store.Objects.Insert(config.Config.Common.FuzzOutputGSBucket, &storage.Object{Name: parent + "/" + hash + "/fuzz_fragment.cpp"}).Media(buf)
	if _, err := req.Do(); err != nil {
		return fmt.Errorf("Failed to write fuzz code to Google Storage for %s: %s", hash, err)
	}

	image, err := os.Open(config.Config.Fuzzer.CachePath + "/" + hash + "_raster.png")
	if err != nil {
		glog.Fatalf("Couldn't open the PNG file for %s: %s", hash, err)
	}
	defer util.Close(image)

	req = store.Objects.Insert(config.Config.Common.FuzzOutputGSBucket, &storage.Object{Name: parent + "/" + hash + "/raster.png"}).Media(image)
	if _, err := req.Do(); err != nil {
		return fmt.Errorf("Failed to write output image to Google Storage for %s: %s", hash, err)
	}
	return nil
}

func main() {
	flag.Parse()

	setDefaults()

	common.DecodeTomlFile(*configFilename, &config.Config)

	var err error
	if client, err = auth.NewClient(config.Config.Common.DoOAuth, config.Config.Common.OAuthCacheFile, storage.DevstorageFull_controlScope); err != nil {
		glog.Fatalf("Failed to create authenticated HTTP client: %s", err)
	}

	if store, err = storage.New(client); err != nil {
		glog.Fatalf("Failed to create storage service client: %s", err)
	}

	resourcePath, err := setup()
	if err != nil {
		glog.Fatalf("Couldn't setup: %s", err)
	}

	checkCPPTemplate(resourcePath)

	for {
		var status int

		fuzz, err := generator.Fuzz()
		if err != nil {
			glog.Fatalf("Couldn't create a fuzz: %s", err)
		}

		hash, err := writeFuzz(fuzz)
		if err != nil {
			glog.Fatalf("Couldn't create the fuzz hash: %s", err)
		}

		if status, err = runFuzz(hash); err != nil {
			glog.Fatalf("Couldn't run the fuzz (%s): %s", hash, err)
		}

		if err := uploadFuzz(hash, fuzz, status); err != nil {
			glog.Fatalf("Couldn't upload the fuzz results (%s): %s", hash, err)
		}
	}
}
