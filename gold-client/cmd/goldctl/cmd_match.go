package main

import (
	"bytes"
	"fmt"
	"image"
	"image/png"
	"io/ioutil"
	"strings"

	"github.com/spf13/cobra"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/gold-client/go/imgmatching"
)

// matchEnv provides the environment for the match command.
type matchEnv struct {
	algorithmName string
	parameters    []string
}

// getMatchCmd returns the definition of the match command.
func getMatchCmd() *cobra.Command {
	env := &matchEnv{}
	cmd := &cobra.Command{
		Use:   "match",
		Short: "Runs an image matching algorithm against two images on disk",
		Long: `
Takes a (potentially non-exact) image matching algorithm (and any algorithm-specific parameters)
and executes it against the two given images on disk.

Reports whether or not the specified algorithm considers the two images to be equivalent.

This command is intended for experimenting with various combinations of non-exact image matching
algorithms and parameters.
`,
		Args: cobra.ExactArgs(2), // Takes exactly two images as positional arguments.
		Run:  env.runMatchCmd,
	}

	cmd.Flags().StringVar(&env.algorithmName, "algorithm", "", "Image matching algorithm (e.g. exact, fuzzy, sobel).")
	cmd.Flags().StringArrayVar(&env.parameters, "parameter", []string{}, "Any number of algorithm-specific parameters represented as name:value pairs (e.g. sobel_edge_threshold:10).")
	Must(cmd.MarkFlagRequired("algorithm"))

	return cmd
}

// runMatchCmd instantiates the specified image matching algorithm and runs it against two images.
func (m *matchEnv) runMatchCmd(cmd *cobra.Command, args []string) {
	leftImage, err := loadPng(args[0])
	ifErrLogExit(cmd, err)

	rightImage, err := loadPng(args[1])
	ifErrLogExit(cmd, err)

	optionalKeys, err := makeOptionalKeys(m.algorithmName, m.parameters)
	ifErrLogExit(cmd, err)

	_, matcher, err := imgmatching.MakeMatcher(optionalKeys)
	ifErrLogExit(cmd, err)

	imagesMatch := matcher.Match(leftImage, rightImage)

	if imagesMatch {
		fmt.Println("Images match.")
	} else {
		fmt.Println("Images do not match.")
	}
	exitProcess(cmd, 0)
}

// loadPng loads a PNG image from disk.
func loadPng(fileName string) (image.Image, error) {
	// Load the image and save the bytes because we need to return them.
	imgBytes, err := ioutil.ReadFile(fileName)
	if err != nil {
		return nil, skerr.Wrapf(err, "loading file %s", fileName)
	}
	img, err := png.Decode(bytes.NewReader(imgBytes))
	if err != nil {
		return nil, skerr.Wrapf(err, "decoding PNG file %s", fileName)
	}
	return img, nil
}

// makeOptionalKeys turns the specified algorithm name and parameters into the equivalent map of
// optional keys that function imgmatching.MakeMatcher() expects.
func makeOptionalKeys(algorithmName string, parameters []string) (map[string]string, error) {
	keys := map[string]string{
		imgmatching.AlgorithmNameOptKey: algorithmName,
	}

	for _, pair := range parameters {
		split := strings.SplitN(pair, ":", 2)
		if len(split) != 2 {
			return nil, skerr.Fmt("parameter %q must be a key:value pair", pair)
		}
		keys[split[0]] = split[1]
	}

	return keys, nil
}
