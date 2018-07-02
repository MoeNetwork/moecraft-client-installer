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
	if err != nil {
		fmt.Println()
		fmt.Println("安装失败")

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
	sem <- true
	go func() {
		f, err := os.Create(path)
		bullshit(err)
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
	fmt.Println("[ MoeCraft 客户端安装器 ]")
	fmt.Println("如遇问题, 请在群里求助管理员, 或前去以下网址汇报: ")
	fmt.Println("https://github.com/balthild/moecraft-client-installer")
	fmt.Println()

	fmt.Println("警告: 该程序会在它所在的文件夹内安装/更新 MoeCraft 专用客户端, 并删除该文件夹内其它的 Minecraft 版本")
	fmt.Println("请勿把安装器与无关文件, 尤其是 Minecraft 客户端放在同一个文件夹内, 否则, 由此引起的数据损失, 安装器概不负责")
	fmt.Println()

	fmt.Println("如果你需要安装自定义 mod, 请在安装器旁边建立 mods 文件夹, 并把自定义 mod 放入其中")
	fmt.Println("不要把自定义 mod 直接放在 .minecraft/mods 中, 否则它们会被删除")
	fmt.Println()

	useWorkDir := os.Getenv("USE_WORK_DIR")
	if useWorkDir != "true" && useWorkDir != "1" {
		ex, err := os.Executable()
		bullshit(err)

		dir := filepath.Dir(ex)
		os.Chdir(dir)

		fmt.Println("请确认安装位置:", dir)
		fmt.Print("如无错误，按 [Enter] 继续:")
		fmt.Scanln()
		fmt.Println()
	}

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

	waitForDownloading()

	var customMods []string
	files, _ := ioutil.ReadDir("mods")
	for _, file := range files {
		if file.IsDir() {
			continue
		}

		customMods = append(customMods, file.Name())
		removeFiles[".minecraft/mods/" + file.Name()] = false
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

	if len(customMods) > 0 {
		fmt.Println()
		fmt.Println("正在安装自定义 mod:")

		for _, mod := range customMods {
			src, err := os.Open("mods/" + mod)
			if err != nil {
				fmt.Println("安装", mod, "失败:", err.Error())
			}

			dst, err := os.Create(".minecraft/mods/" + mod)
			if err != nil {
				fmt.Println("安装", mod, "失败:", err.Error())
			}

			_, err = io.Copy(dst,src)
			if err != nil {
				fmt.Println("安装", mod, "失败:", err.Error())
			}

			src.Close()
			dst.Close()
		}
	}

	fmt.Println("安装完成")
}
