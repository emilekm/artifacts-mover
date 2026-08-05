package main

import (
	"flag"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

const (
	flagsDir = "menu/External-zip/FlashMenu/images/joingame"
)

var (
	prbf2RepoDir    string
	assetsOutputDir string
)

func main() {
	flag.StringVar(&prbf2RepoDir, "repo-dir", ".", "Path to the PR:BF2 repository directory")
	flag.StringVar(&assetsOutputDir, "output-dir", "", "Path to output the extracted flags")
	flag.Parse()

	if err := run(prbf2RepoDir, assetsOutputDir); err != nil {
		panic(err)
	}
}

func run(repoDir, outputDir string) error {
	matches, err := fs.Glob(os.DirFS(filepath.Join(repoDir, flagsDir)), "flagLarge_*.png")
	if err != nil {
		return err
	}

	for _, match := range matches {
		srcPath := filepath.Join(repoDir, flagsDir, match)
		flagName := strings.ToLower(strings.SplitN(strings.TrimSuffix(match, ".png"), "_", 2)[1])

		dstPath := filepath.Join(outputDir, flagName+".png")

		srcFile, err := os.Open(srcPath)
		if err != nil {
			return err
		}
		defer srcFile.Close()

		dstFile, err := os.Create(dstPath)
		if err != nil {
			return err
		}
		defer dstFile.Close()

		_, err = io.Copy(dstFile, srcFile)
		if err != nil {
			return err
		}
	}

	return nil
}
