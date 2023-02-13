package main

import (
	"bufio"
	"compress/gzip"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"
)

var (
	dirFlag  = flag.String("dir", "es_modules", "")
	gzipFlag = flag.Bool("gzip", false, "")
)

func main() {
	flag.Parse()
	nodeModules, err := os.Open("node_modules")
	if err != nil {
		log.Fatal(err)
	}
	dirEntries, err := nodeModules.ReadDir(-1)
	if err != nil {
		log.Fatal(err)
	}
	err = os.RemoveAll(*dirFlag)
	if err != nil {
		log.Fatal(err)
	}
	err = os.MkdirAll(*dirFlag, 0755)
	if err != nil {
		log.Fatal(err)
	}
	for _, d := range dirEntries {
		if !d.IsDir() {
			continue
		}
		file, err := os.Open(filepath.Join("node_modules", d.Name(), "package.json"))
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			log.Println(err)
			continue
		}
		packageInfo := make(map[string]any)
		err = json.NewDecoder(file).Decode(&packageInfo)
		if err != nil {
			log.Println(err)
			continue
		}
		// Search order: exports.import -> exports["."].import -> module
		var importPath string
		exports, ok := packageInfo["exports"].(map[string]any)
		if ok {
			Import, ok := exports["import"].(string)
			if ok {
				importPath = Import
			} else {
				dot, ok := exports["."].(map[string]any)
				if ok {
					Import, ok := dot["import"].(string)
					if ok {
						importPath = Import
					}
				}
			}
		}
		if importPath == "" {
			module, ok := packageInfo["module"].(string)
			if ok {
				importPath = module
			}
		}
		if importPath == "" {
			log.Println("(skip) " + filepath.Join("node_modules", d.Name(), "package.json"))
			continue
		}
		_ = file.Close()
		inputFile, err := os.Open(filepath.Join("node_modules", d.Name(), importPath))
		if err != nil {
			log.Println(err)
			continue
		}
		outputName := filepath.Join(*dirFlag, filepath.Base(d.Name())+".js")
		if *gzipFlag {
			outputName += ".gz"
		}
		err = os.MkdirAll(filepath.Dir(outputName), 0755)
		if err != nil {
			log.Println(err)
			continue
		}
		outputFile, err := os.OpenFile(outputName, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
		if err != nil {
			log.Println(err)
			continue
		}
		var output io.Writer
		var gzipWriter *gzip.Writer
		if *gzipFlag {
			gzipWriter, _ = gzip.NewWriterLevel(outputFile, gzip.BestCompression)
			output = gzipWriter
		} else {
			output = outputFile
		}
		bufioReader := bufio.NewReader(inputFile)
		for {
			line, err := bufioReader.ReadString('\n')
			isDone := err == io.EOF
			if err != nil && !isDone {
				log.Println(err)
				break
			}
			if strings.HasPrefix(strings.TrimSpace(line), "import") {
				sep := "\""
				if strings.Index(line, sep) < 0 {
					sep = "'"
				}
				before, after, ok := strings.Cut(line, sep)
				if !ok {
					continue
				}
				name, after, ok := strings.Cut(after, sep)
				if !ok {
					continue
				}
				newName := "/" + strings.TrimPrefix(filepath.ToSlash(filepath.Join(*dirFlag, filepath.Base(name)+".js")), "/")
				if *gzipFlag {
					newName += ".gz"
				}
				_, err = io.WriteString(output, before+sep+newName+sep+after)
				if err != nil {
					log.Println(err)
					break
				}
			} else {
				_, err = io.WriteString(output, line)
				if err != nil {
					log.Println(err)
					break
				}
			}
			if isDone {
				break
			}
		}
		fmt.Println(filepath.ToSlash(outputName))
		err = inputFile.Close()
		if err != nil {
			log.Println(err)
		}
		if gzipWriter != nil {
			err = gzipWriter.Close()
			if err != nil {
				log.Println(err)
			}
		}
		err = outputFile.Close()
		if err != nil {
			log.Println(err)
		}
	}
}
