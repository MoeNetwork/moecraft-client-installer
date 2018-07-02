package main

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
)

// @TODO
var baseURLs = [...]string{
	"https://cdn.moecraft.net/client/",
}

type FileEntry struct {
	Path string `json:"path"`
	MD5  string `json:"md5"`
}

type DirEntry struct {
	Path  string       `json:"path"`
	Files []*FileEntry `json:"files"`
}

type Metadata struct {
	SyncedDirs  []*DirEntry  `json:"synced_dirs"`
	SyncedFiles []*FileEntry `json:"synced_files"`
}

var metadata Metadata
var baseURL string

// Limit downloading concurrency to 5
var sem = make(chan bool, 5)

func bullshit(err error) {
	// Fuck the shitty golang error handling
	if err != nil {
		panic(err)
	}
}

func hashFile(path string) string {
	f, err := os.Open(path)
	bullshit(err)
	defer f.Close()

	hash := md5.New()
	_, err = io.Copy(hash, f)
	bullshit(err)

	sum := hash.Sum(nil)[:16]
	return hex.EncodeToString(sum)
}

func downloadFile(path string) {
	f, err := os.Create(path)
	bullshit(err)

	sem <- true
	go func() {
		defer f.Close()

		resp, err := http.Get(baseURL + path)
		bullshit(err)
		defer resp.Body.Close()

		_, err = io.Copy(f, resp.Body)
		bullshit(err)

		fmt.Println("已下载:", path)

		<-sem
	}()
}

func waitForDownloading() {
	for i := 0; i < cap(sem); i++ {
		sem <- true
	}
}

func ensureFile(path string, md5 string) {
	err := os.MkdirAll(filepath.Dir(path), 0755)
	bullshit(err)

	stat, err := os.Stat(path)

	if os.IsNotExist(err) {
		downloadFile(path)
		return
	}

	bullshit(err)

	if stat.IsDir() || hashFile(path) != md5 {
		err = os.RemoveAll(path)
		bullshit(err)

		downloadFile(path)
		return
	}
}

func main() {
	baseURL = os.Getenv("BASE_URL")
	if len(baseURL) == 0 {
		fmt.Println("目前可用的下载源:")
		for i, url := range baseURLs {
			fmt.Printf("[%d] %s", i+1, url)
			fmt.Println()
		}

		for {
			fmt.Print("选择一个下载源 (默认为 1): ")

			var choose int
			fmt.Scan(&choose)

			if choose > 0 && choose <= len(baseURLs) {
				baseURL = baseURLs[choose-1]
				break
			}

			fmt.Println("Are you kidding me?")
		}
	}

	resp, err := http.Get(baseURL + "metadata.json")
	bullshit(err)

	data, err := ioutil.ReadAll(resp.Body)
	bullshit(err)

	resp.Body.Close()

	err = json.Unmarshal(data, &metadata)
	bullshit(err)

	removeDirs := make(map[string]bool)
	removeFiles := make(map[string]bool)

	for _, dir := range metadata.SyncedDirs {
		os.MkdirAll(dir.Path, 0755)

		filepath.Walk(dir.Path, func(path string, info os.FileInfo, err error) error {
			if info.IsDir() {
				removeDirs[path] = true
			} else {
				removeFiles[path] = true
			}

			return nil
		})

		for _, file := range dir.Files {
			removeFiles[file.Path] = false

			basePath := filepath.Dir(file.Path)
			for basePath != "." {
				removeDirs[basePath] = false
				basePath = filepath.Dir(basePath)
			}

			ensureFile(file.Path, file.MD5)
		}
	}

	for _, file := range metadata.SyncedFiles {
		removeFiles[file.Path] = false

		basePath := filepath.Dir(file.Path)
		for basePath != "." {
			removeDirs[basePath] = false
			basePath = filepath.Dir(basePath)
		}

		ensureFile(file.Path, file.MD5)
	}

	for path, remove := range removeFiles {
		if remove {
			os.Remove(path)
			fmt.Println("已删除:", path)
		}
	}

	for path, remove := range removeDirs {
		if remove {
			os.RemoveAll(path)
			fmt.Println("已删除:", path)
		}
	}

	waitForDownloading()

	fmt.Println("安装完成")
}
