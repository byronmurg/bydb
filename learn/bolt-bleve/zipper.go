package main

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"archive/tar"
	"compress/gzip"
	"errors"
	"io"
	"os"
)

func getPathFiles(basePath string) ([]string, error) {
	var ret []string
	err := filepath.Walk(basePath, func(path string, info fs.FileInfo, err error) error {
		if err != nil { return err }
		if info.IsDir() { return nil }
		ret = append(ret, path)
		return nil
	})
	return ret, err
}

func CreateTarball(tarballFilePath string, sourcePath string) error {
	file, err := os.Create(tarballFilePath)
	if err != nil {
		return errors.New(fmt.Sprintf("Could not create tarball file '%s', got error '%s'", tarballFilePath, err.Error()))
	}
	defer file.Close()

	return WriteTarballToFD(file, sourcePath)
}

func WriteTarballToFD(writer io.Writer, sourcePath string) error {

	filePaths, locateError := getPathFiles(sourcePath)
	if locateError != nil {
		return locateError
	}
	
	gzipWriter := gzip.NewWriter(writer)
	defer gzipWriter.Close()

	tarWriter := tar.NewWriter(gzipWriter)
	defer tarWriter.Close()

	for _, filePath := range filePaths {
		err := addFileToTarWriter(filePath, tarWriter)
		if err != nil {
			return errors.New(fmt.Sprintf("Could not add file '%s', to tarball, got error '%s'", filePath, err.Error()))
		}
	}

	return nil
}


// Private methods

func addFileToTarWriter(filePath string, tarWriter *tar.Writer) error {
	file, err := os.Open(filePath)
	if err != nil {
		return errors.New(fmt.Sprintf("Could not open file '%s', got error '%s'", filePath, err.Error()))
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		return errors.New(fmt.Sprintf("Could not get stat for file '%s', got error '%s'", filePath, err.Error()))
	}

	header := &tar.Header{
		Name:    filePath,
		Size:    stat.Size(),
		Mode:    int64(stat.Mode()),
		ModTime: stat.ModTime(),
	}

	err = tarWriter.WriteHeader(header)
	if err != nil {
		return errors.New(fmt.Sprintf("Could not write header for file '%s', got error '%s'", filePath, err.Error()))
	}

	_, err = io.Copy(tarWriter, file)
	if err != nil {
		return errors.New(fmt.Sprintf("Could not copy the file '%s' data to the tarball, got error '%s'", filePath, err.Error()))
	}

	return nil
}
