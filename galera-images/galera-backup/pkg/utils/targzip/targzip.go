package targzip

import (
	"archive/tar"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// checkFileName is a regular expression used to verify if a give filename ends with ".tar.gz"
var checkFileName = regexp.MustCompile("(?im)^(.*).tar.gz$")

func DestFile (fileName string) (io.Writer, error) {
	if !checkFileName.Match([]byte(fileName)) {
		fileName = fmt.Sprintf("%s.tar.gz", fileName)
	}

	return os.Create(fileName)
}

// Tar takes a source and variable writers and walks 'source' writing each file
// found to the tar writer; the purpose for accepting multiple writers is to allow
// for multiple outputs (for example a file, or md5 hash)
func TarGzip(sourceDir string, writers ...io.Writer) error {
	info, err := os.Stat(sourceDir)
	if err != nil {
		return err
	}

	if !info.IsDir() {
		err = errors.New(fmt.Sprintf("specified source directory is not a directory"))
		return err
	}

	mw := io.MultiWriter(writers...)

	gzw := gzip.NewWriter(mw)
	defer gzw.Close()

	tw := tar.NewWriter(gzw)
	defer tw.Close()

//	baseDir := filepath.Base(sourceDir)

	// walk path
	return filepath.Walk(sourceDir, func(path string, info os.FileInfo, err error) error {
		// return on any error
		if err != nil {
			return err
		}
		// create a new dir/file header
		header, err := tar.FileInfoHeader(info, info.Name())
		if err != nil {
			return err
		}

		// update the name to correctly reflect the desired destination when untaring
		header.Name = strings.TrimPrefix(strings.Replace(path, sourceDir, "", -1), string(filepath.Separator))
		/*
		if baseDir != "" {
			header.Name = filepath.Join(baseDir, strings.TrimPrefix(path, sourceDir))
		}
		*/

		if header.Name == "" {
			return nil
		}

		// write the header
		if err := tw.WriteHeader(header); err != nil {
			return err
		}

		// return on non-regular files
		if info.IsDir() {
	//	if !info.Mode().IsRegular() {
			return nil
		}

		// open files for taring
		file, err := os.Open(path)
		if err != nil {
			return err
		}
		defer file.Close()
		_, err = io.Copy(tw, file)
		return err
	})
}

// Untar takes a destination path and a reader; a tar reader loops over the tarfile
// creating the file structure at 'dst' along the way, and writing any files
func UnTarGzip(destDir string, r io.Reader) error {

	gzr, err := gzip.NewReader(r)
	if err != nil {
		return err
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)

	for {
		header, err := tr.Next()

		switch {
		// if no more files are found return
		case err == io.EOF:
			return nil

		// return any other error
		case err != nil:
			return err

		// if the header is nil, just skip it (not sure how this happens)
		case header == nil:
			continue
		}

		// the target location where the dir/file should be created
		target := filepath.Join(destDir, header.Name)

		// the following switch could also be done using fi.Mode(), not sure if there
		// a benefit of using one vs. the other.
		// fi := header.FileInfo()

		// check the file type
		switch header.Typeflag {

		// if its a dir and it doesn't exist create it
		case tar.TypeDir:
			if _, err := os.Stat(target); err != nil {
				if err := os.MkdirAll(target, 0755); err != nil {
					return err
				}
			}

		// if it's a file create it
		case tar.TypeReg:
			f, err := os.OpenFile(target, os.O_CREATE|os.O_RDWR, os.FileMode(header.Mode))
			if err != nil {
				return err
			}

			// copy over contents
			if _, err := io.Copy(f, tr); err != nil {
				return err
			}

			// manually close here after each file operation; defering would cause each file close
			// to wait until all operations have completed.
			f.Close()
		}
	}
}
