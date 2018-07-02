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

type Repo struct {
	Name    string
	BaseURL string
}

var repos = [...]Repo{
	// {"MoeCraft CDN", "https://cdn.moecraft.net/"},
	{"Git@OSC", "https://gitee.com/balthild/client/raw/master/"},
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
	UpdatedAt    int64        `json:"updated_at"`
	SyncedDirs   []*DirEntry  `json:"synced_dirs"`
	SyncedFiles  []*FileEntry `json:"synced_files"`
	DefaultFiles []*FileEntry `json:"default_files"`
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

func addToKeepList(path string, keep map[string]bool) {
	keep[path] = true

	basePath := path
	for basePath != "." {
		basePath = filepath.Dir(basePath)
		keep[basePath] = true
	}
}

func main() {
	fmt.Print(`MoeCraft 客户端安装器

如遇问题, 请在群里求助管理员, 或前去以下网址汇报:
https://github.com/balthild/moecraft-client-installer

警告:
该程序将于它所在的文件夹安装 MoeCraft 客户端, 并删除该文件夹内的其他 Minecraft 版本
请勿把安装器与无关文件放在同一文件夹内, 否则, 使用者需自行承担由此引起的数据损失

如果你需要安装自定义 Mod, 请在安装器旁边建立 mods 文件夹, 并把自定义 Mod 放入其中
不要把自定义 Mod 直接放在 .minecraft/mods 中, 否则它们会被删除

`)

	baseURL = os.Getenv("BASE_URL")
	if len(baseURL) == 0 {
		fmt.Println("目前可用的下载源:")
		for i, repo := range repos {
			fmt.Printf("[%d] %s", i+1, repo.Name)
			fmt.Println()
		}

		for {
			fmt.Print("选择一个下载源 (默认为 1): ")

			var choose int
			fmt.Scan(&choose)

			if choose > 0 && choose <= len(repos) {
				baseURL = repos[choose-1].BaseURL
				break
			}

			fmt.Println("Are you kidding me?")
		}

		fmt.Println()
	}

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

	resp, err := http.Get(baseURL + "metadata.json")
	bullshit(err)

	data, err := ioutil.ReadAll(resp.Body)
	bullshit(err)

	resp.Body.Close()

	err = json.Unmarshal(data, &metadata)
	bullshit(err)

	keep := make(map[string]bool)

	for _, dir := range metadata.SyncedDirs {
		for _, file := range dir.Files {
			addToKeepList(file.Path, keep)
			ensureFile(file.Path, file.MD5)
		}
	}

	for _, file := range metadata.SyncedFiles {
		addToKeepList(file.Path, keep)
		ensureFile(file.Path, file.MD5)
	}

	for _, file := range metadata.DefaultFiles {
		addToKeepList(file.Path, keep)

		_, err := os.Stat(file.Path)
		if os.IsNotExist(err) {
			downloadFile(file.Path)
			return
		}
	}

	waitForDownloading()

	var customMods []string
	files, _ := ioutil.ReadDir("mods")
	for _, file := range files {
		if file.IsDir() {
			continue
		}

		customMods = append(customMods, file.Name())
		keep[".minecraft/mods/"+file.Name()] = true
	}

	for _, dir := range metadata.SyncedDirs {
		filepath.Walk(dir.Path, func(path string, info os.FileInfo, err error) error {
			if !keep[path] {
				os.RemoveAll(path)
				fmt.Println("已删除:", path)
			}

			return nil
		})
	}

	for _, mod := range customMods {
		src, err := os.Open("mods/" + mod)
		bullshit(err)
		defer src.Close()

		dst, err := os.Create(".minecraft/mods/" + mod)
		bullshit(err)
		defer dst.Close()

		_, err = io.Copy(dst, src)
		bullshit(err)

		fmt.Println("自定义 Mod:", mod)
	}

	fmt.Println("安装完成")
}
