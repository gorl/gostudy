package main

import (
	"os"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"sort"
)


type Writable interface {
	WriteString(s string) (n int, err error)
}

type FileNames []os.FileInfo;

func (fn FileNames) Len() int {
	return len(fn)
}
func (fn FileNames) Swap(i, j int) {
	fn[i], fn[j] = fn[j], fn[i]
}

func (fn FileNames) Less(i, j int) bool {
	return fn[i].Name() < fn[j].Name()
}

func filter(arr []os.FileInfo, predicate func(fi os.FileInfo) bool) []os.FileInfo {
	result := make([]os.FileInfo, 0, len(arr))
	for _, fi := range arr {
		if predicate(fi) {
			result = append(result, fi)
		}
	}
	return result
}

func prefixString(prefix []bool) string {
	var s string
	for idx, flag := range prefix {
		isLast := idx == len(prefix) - 1
		switch {
		case flag && isLast:
			s += "└───"
		case flag && !isLast:
			s += "\t"
		case !flag && !isLast:
			s += "│\t"
		case !flag && isLast:
			s += "├───"
		}
	}
	return s
}

func dirTree2(out Writable, path string, showFiles bool, prefix []bool) error {
	files, err := ioutil.ReadDir(path)
	if err != nil {
		return fmt.Errorf("can't open directory")
	}

	files = filter(FileNames(files), func(fi os.FileInfo) bool {
		return fi.IsDir() || showFiles;
	})

	sort.Sort(FileNames(files))

	nextPrefix := make([]bool, len(prefix) + 1)
	copy(nextPrefix, prefix)

	for idx, f := range files {
		nextPrefix[len(prefix)] = idx == len(files) - 1
		if f.IsDir() {

			if _, err = out.WriteString(fmt.Sprintf("%s%s\n", prefixString(nextPrefix), f.Name())); err != nil {
				return err
			}

			if err := dirTree2(out, filepath.Join(path, f.Name()), showFiles, nextPrefix); err != nil {
				return err
			}
		} else if showFiles {
			if _, err = out.WriteString(fmt.Sprintf("%s%s (%s)\n", prefixString(nextPrefix), f.Name(), prettySize(f.Size()))); err != nil {
				return err
			}
		}

	}
	return nil
}

func prettySize(size int64) string {
	if size == 0 {
		return "empty"
	}
	return fmt.Sprintf("%db", size)
}


func dirTree(out Writable, path string, showFiles bool) error {
	return dirTree2(out, path, showFiles, []bool{})
}


func main() {
	out := os.Stdout
	if !(len(os.Args) == 2 || len(os.Args) == 3) {
		panic("usage go run main.go . [-f]")
	}
	path := os.Args[1]
	printFiles := len(os.Args) == 3 && os.Args[2] == "-f"
	err := dirTree(out, path, printFiles)
	if err != nil {
		panic(err.Error())
	}
}

