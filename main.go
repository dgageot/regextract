package main

import (
	"archive/tar"
	"compress/gzip"
	"bufio"
	"flag"
	"io"
	"log"
	"os"
	"strings"

	"github.com/heroku/docker-registry-client/registry"
)

var layer int
var output string

func init() {
	flag.IntVar(&layer, "layer", -1, "layer to extract from")
	flag.StringVar(&output, "output", "", "output file")
}

func main() {
	flag.Parse()

	if flag.NArg() < 1 {
		log.Fatalf("usage: regextract [--output=file.tgz] image[:tag] [files...]")
	}
	tag := "latest"
	it := strings.SplitN(flag.Arg(0), ":", 2)
	if len(it) == 2 {
		tag = it[1]
	}
	image := it[0]

	allfiles := flag.NArg() == 1
	files := flag.Args()[1:]
	fileset := make(map[string]bool)
	for _, v := range files {
		fileset[v] = true
	}

	if output != "" && flag.NArg() != 1 {
		log.Fatalf("Cannot specify an output file when only some files are extracted")
	}

	url := "https://registry-1.docker.io/"
	username := ""
	password := ""
	hub, err := registry.New(url, username, password)
	if err != nil {
		log.Fatalf("Cannot connect to registry")
	}

	manifest, err := hub.Manifest(image, tag)
	if err != nil {
		log.Fatalf("Cannot fetch manifest: %s", err)
	}

	log.Printf("Found %d manifest layers, using layer %d", len(manifest.FSLayers), len(manifest.FSLayers)-1)

	if layer < 0 {
		layer += len(manifest.FSLayers)
	}
	fslayer := manifest.FSLayers[layer]

	reader, err := hub.DownloadLayer(image, fslayer.BlobSum)
	if err != nil {
		log.Fatalf("cannot read layer")
	}
	defer reader.Close()

	if output != "" {
		f, err := os.Create(output)
		if err != nil {
			log.Fatalf("Unable to create file %s: %s", output, err)
		}

		out := bufio.NewWriter(f)
		defer out.Flush()

		_, err = io.Copy(out, reader)
		if err != nil {
			log.Fatalln(err)
		}

		return
	}

	unzipper, err := gzip.NewReader(reader)
	if err != nil {
		log.Fatalf("Cannot uncompress: %s", err)
	}

	tr := tar.NewReader(unzipper)
	tw := tar.NewWriter(os.Stdout)

	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Fatalln(err)
		}

		if fileset[hdr.Name] || allfiles {
			err = tw.WriteHeader(hdr)
			if err != nil {
				log.Fatalf("cannot write to tar: %s", err)
			}

			_, err = io.Copy(tw, tr)
			if err != nil {
				log.Fatalln(err)
			}
		}
	}

	err = tw.Close()
	if err != nil {
		log.Fatalf("Tar close error: %s", err)
	}
}
