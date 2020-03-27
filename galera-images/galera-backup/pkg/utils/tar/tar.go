package tar

import (
	"archive/tar"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// checkFileName is a regular expression used to verify if a give filename ends with ".tar"
var checkFileName = regexp.MustCompile("(?im)^(.*).tar$")

// Tar is a function used to tar a source directory to a target directory with
// the specified filename
func Tar(sourceDir, targetDir, fileName string) error {
	if !checkFileName.Match([]byte(fileName)) {
		fileName = fmt.Sprintf("%s.tar", fileName)
	}
	fileName = filepath.Join(targetDir, fileName)

	tarFile, err := os.Create(fileName)
	if err != nil {
		return err
	}
	defer tarFile.Close()

	tarBall := tar.NewWriter(tarFile)
	defer tarBall.Close()

	info, err := os.Stat(sourceDir)
	if err != nil {
		return err
	}

	if !info.IsDir() {
		err = errors.New(fmt.Sprintf("specified source directory is not a directory"))
		return err
	}

	baseDir := filepath.Base(sourceDir)

	return filepath.Walk(sourceDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		header, err := tar.FileInfoHeader(info, info.Name())
		if err != nil {
			return err
		}

		if baseDir != "" {
			header.Name = filepath.Join(baseDir, strings.TrimPrefix(path, sourceDir))
		}

		if err := tarBall.WriteHeader(header); err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		file, err := os.Open(path)
		if err != nil {
			return err
		}
		defer file.Close()
		_, err = io.Copy(tarBall, file)
		return err
	})
}

// UnTar is a function that untar the specified tarBall to the target directory
func UnTar(tarBall, targetDir string) error {
	reader, err := os.Open(tarBall)
	if err != nil {
		return err
	}
	defer reader.Close()
	tarReader := tar.NewReader(reader)

	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		} else if err != nil {
			return err
		}

		path := filepath.Join(targetDir, header.Name)
		info := header.FileInfo()
		if info.IsDir() {
			if err = os.MkdirAll(path, info.Mode()); err != nil {
				return err
			}
			continue
		}

		file, err:= os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, info.Mode())
		if err != nil {
			return err
		}
		defer file.Close()

		_, err =io.Copy(file, tarReader)
		if err != nil {
			return err
		}
	}

	return err
}
