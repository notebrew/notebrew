package main

import (
	"bufio"
	"compress/gzip"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"log"
	"math/big"
	"os"
	"path/filepath"
	"strings"
)

var (
	dirFlag  = flag.String("dir", "esmodules", "the directory to copy the output files to.")
	gzipFlag = flag.Bool("gzip", false, "gzips output files so they can be served directly with Content-Encoding: gzip.")
)

type PackageInfo struct {
	Name       string
	Version    string
	ImportPath string
}

// NOTE: Nested dependencies are completely ignored.
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
	err = os.MkdirAll(*dirFlag, 0755)
	if err != nil {
		log.Fatal(err)
	}
	packageNames := make([]string, 0, len(dirEntries))
	packageInfos := make(map[string]PackageInfo)
	for _, d := range dirEntries {
		if !d.IsDir() {
			continue
		}
		file, err := os.Open(filepath.Join("node_modules", d.Name(), "package.json"))
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			log.Println(d.Name(), err)
			continue
		}
		m := make(map[string]any)
		err = json.NewDecoder(file).Decode(&m)
		if err != nil {
			log.Println(d.Name(), err)
			continue
		}
		_ = file.Close()
		packageNames = append(packageNames, d.Name())
		packageInfo := PackageInfo{Name: d.Name()}
		exports, ok := m["exports"].(map[string]any)
		if ok {
			packageInfo.ImportPath, ok = exports["import"].(string)
			if !ok {
				dot, ok := exports["."].(map[string]any)
				if ok {
					packageInfo.ImportPath, _ = dot["import"].(string)
				}
			}
		}
		if packageInfo.ImportPath == "" {
			packageInfo.ImportPath, _ = m["module"].(string)
		}
		if packageInfo.ImportPath == "" {
			log.Println("(skip) " + filepath.Join("node_modules", d.Name(), "package.json"))
			continue
		}
		packageInfo.Version, _ = m["version"].(string)
		if packageInfo.Version != "" {
			packageInfos[packageInfo.Name] = packageInfo
			continue
		}
		file, err = os.Open(filepath.Join("node_modules", d.Name(), packageInfo.ImportPath))
		if err != nil {
			log.Println(d.Name(), err)
			continue
		}
		hash := sha256.New()
		_, err = io.Copy(hash, file)
		if err != nil {
			log.Println(d.Name(), err)
			continue
		}
		_ = file.Close()
		packageInfo.Version = new(big.Int).SetBytes(hash.Sum(nil)).Text(62)
		packageInfos[packageInfo.Name] = packageInfo
	}
	for _, packageName := range packageNames {
		packageInfo, ok := packageInfos[packageName]
		if !ok {
			continue
		}
		inputName := filepath.Join("node_modules", packageName, packageInfo.ImportPath)
		inputFile, err := os.Open(inputName)
		if err != nil {
			log.Println(packageName, err)
			continue
		}
		outputName := filepath.Join(*dirFlag, packageName+"@"+packageInfo.Version+".js")
		if *gzipFlag {
			outputName += ".gz"
		}
		outputFile, err := os.OpenFile(outputName, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
		if err != nil {
			log.Println(packageName, err)
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
			if !strings.HasPrefix(strings.TrimSpace(line), "import") {
				_, err = io.WriteString(output, line)
				if err != nil {
					log.Println(packageName, err)
				}
			} else {
				sep := "\""
				if strings.Index(line, sep) < 0 {
					sep = "'"
				}
				before, after, ok := strings.Cut(line, sep)
				if !ok {
					_, err = io.WriteString(output, line)
					if err != nil {
						log.Println(packageName, err)
					}
					continue
				}
				name, after, ok := strings.Cut(after, sep)
				if !ok {
					_, err = io.WriteString(output, line)
					if err != nil {
						log.Println(packageName, err)
					}
					continue
				}
				pkg, ok := packageInfos[name]
				if !ok {
					_, err = io.WriteString(output, line)
					if err != nil {
						log.Println(packageName, err)
					}
					continue
				}
				newName := filepath.ToSlash(filepath.Join(*dirFlag, name+"@"+pkg.Version+".js"))
				if *gzipFlag {
					newName += ".gz"
				}
				_, err = io.WriteString(output, before+sep+"/"+newName+sep+after)
				if err != nil {
					log.Println(packageName, err)
				}
			}
			if isDone {
				break
			}
		}
		fmt.Println(filepath.ToSlash(outputName))
		err = inputFile.Close()
		if err != nil {
			log.Println(packageName, err)
		}
		if gzipWriter != nil {
			err = gzipWriter.Close()
			if err != nil {
				log.Println(packageName, err)
			}
		}
		err = outputFile.Close()
		if err != nil {
			log.Println(packageName, err)
		}
	}
}
