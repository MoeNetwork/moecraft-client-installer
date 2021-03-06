package main

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/alexflint/go-arg"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"errors"
)

type Repo struct {
	ID      int    `json:"id"`
	Name    string `json:"name"`
	BaseURL string `json:"baseurl"`
}

var repos = [...]Repo{
	{1, "MoeCraft CDN", "https://cdn.kotori.net/MoeCraft/"},
	{2, "国外节点", "https://gitlab.com/Kenvix/moxbin/raw/master/"},
}

type Arguments struct {
	Repo    int
	BaseURL string `help:"Overrides the --repo argument"`
	Dir     string
	Yes     bool   `help:"Automatically answer yes for all questions"`
	NoLogo  bool   `help:"Do not show logo"`
	Exit    bool   `help:"Exit after things from command line done"`
	Print   string `help:"Print one of the following information in JSON format: repos"`
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

type KeepList map[string]bool

func (keep KeepList) Add(path string) {
	keep[filepath.Clean(path)] = true

	parent := path
	for parent != "." {
		parent = filepath.Dir(parent)
		keep[parent] = true
	}
}

func (keep KeepList) Has(path string) bool {
	return keep[filepath.Clean(path)]
}

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

func validatePath(path string) {
	if path[0] == '/' || path[1] == ':' {
		panic("Absolute path is not allowed: " + path)
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

func setAuthlibInjectorServer() {
	data, err := ioutil.ReadFile("HMCLData/hmcl.json")
	if err != nil {
		return
	}

	hmclConfig := make(map[string]interface{})
	json.Unmarshal(data, &hmclConfig)

	url := json.RawMessage(`"https://accounts.moecraft.net/?s\u003dAPI/Mc/authlib\u0026params\u003d/"`)
	hmclConfig["authlibInjectorServers"] = []struct {
		URL  json.RawMessage `json:"url"`
		Name string          `json:"name"`
	}{{url, "MoeCraft"}}

	data, err = json.Marshal(hmclConfig)
	bullshit(err)

	err = ioutil.WriteFile("HMCLData/hmcl.json", data, 644)
	bullshit(err)
}

func main() {
	var args Arguments
	arg.MustParse(&args)

	if !args.NoLogo && !args.Exit {
		fmt.Print(`MoeCraft 客户端安装器
======================================================
如遇问题, 请在群里求助管理员, 或前去以下网址汇报:
https://github.com/balthild/moecraft-client-installer

警告:该程序将于它所在的文件夹安装 MoeCraft 客户端, 并删除该文件夹内的其他 Minecraft 版本. 请勿把安装器与无关文件放在同一文件夹内, 否则, 使用者需自行承担可能发生的数据损失.

如果你需要添加自定义 Mod, 请在安装器旁边建立 mods 文件夹, 并把Mod 放入这个文件夹中. 不要把 Mod 直接放在 .minecraft/mods 中, 否则它们会被删除.
======================================================
`)
	}

	if len(args.Print) > 0 {
		var PrintType = strings.ToLower(args.Print)
		switch PrintType {
		case "repos":
			jsonBytes, jsonError := json.Marshal(repos)
			if jsonError != nil {
				fmt.Print(jsonError)
			}
			fmt.Print(string(jsonBytes))
		}
	}

	if args.Exit {
		os.Exit(0)
	}

	if len(args.BaseURL) != 0 {
		if strings.HasSuffix(args.BaseURL, "/") {
			baseURL = args.BaseURL
		} else {
			baseURL = args.BaseURL + "/"
		}
	} else if args.Repo != 0 {
		if args.Repo > 0 && args.Repo <= len(repos) {
			baseURL = repos[args.Repo-1].BaseURL
		} else {
			panic("Invalid repo")
		}
	} else {
		if len(repos) > 1 && !args.Yes {
			fmt.Println("目前可用的下载源:")
			for i, repo := range repos {
				fmt.Printf("[%d] %s", i+1, repo.Name)
				fmt.Println()
			}

			for {
				fmt.Print("选择一个下载源(输入序号): ")

				var choose int
				fmt.Scan(&choose)

				if choose > 0 && choose <= len(repos) {
					baseURL = repos[choose-1].BaseURL
					break
				}

				fmt.Println("Are you kidding me?")
			}
		} else {
			fmt.Println("使用默认下载源: " + repos[0].Name)
			baseURL = repos[0].BaseURL
		}
		fmt.Println()
	}

	if len(args.Dir) == 0 {
		ex, err := os.Executable()
		bullshit(err)

		dir := filepath.Dir(ex) + "/MoeCraft"

		stat, err := os.Stat(dir)
		if os.IsNotExist(err) {
			os.MkdirAll(dir, 0755)
		} else if !stat.IsDir() {
			bullshit(errors.New("Not a directory: " + dir))
		}

		err = os.Chdir(dir)
		bullshit(err)

		if !args.Yes {
			fmt.Println("请确认安装位置:", dir)
			fmt.Print("如无错误，按 [Enter] 继续:")

			var fuckGo string // Fix bugs on fucking Windows
			fmt.Scanln(&fuckGo)
			fmt.Println()
		}
	} else {
		err := os.Chdir(args.Dir)
		bullshit(err)
	}

	resp, err := http.Get(baseURL + "metadata.json")
	bullshit(err)

	data, err := ioutil.ReadAll(resp.Body)
	bullshit(err)

	resp.Body.Close()

	var metadata Metadata
	err = json.Unmarshal(data, &metadata)
	bullshit(err)

	keep := make(KeepList)

	for _, dir := range metadata.SyncedDirs {
		for _, file := range dir.Files {
			validatePath(file.Path)
			keep.Add(file.Path)
			ensureFile(file.Path, file.MD5)
		}
	}

	for _, file := range metadata.SyncedFiles {
		validatePath(file.Path)
		keep.Add(file.Path)
		ensureFile(file.Path, file.MD5)
	}

	for _, file := range metadata.DefaultFiles {
		validatePath(file.Path)
		keep.Add(file.Path)

		err := os.MkdirAll(filepath.Dir(file.Path), 0755)
		bullshit(err)

		_, err = os.Stat(file.Path)
		if os.IsNotExist(err) {
			downloadFile(file.Path)
			continue
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
		keep.Add(".minecraft/mods/" + file.Name())
	}

	for _, dir := range metadata.SyncedDirs {
		filepath.Walk(dir.Path, func(path string, info os.FileInfo, err error) error {
			if !keep.Has(path) {
				os.RemoveAll(path)
				fmt.Println("已删除:", path)
			}

			return nil
		})
	}

	for _, mod := range customMods {
		func() {
			src, err := os.Open("mods/" + mod)
			bullshit(err)
			defer src.Close()

			dst, err := os.Create(".minecraft/mods/" + mod)
			bullshit(err)
			defer dst.Close()

			_, err = io.Copy(dst, src)
			bullshit(err)

			fmt.Println("自定义 Mod:", mod)
		}()
	}

	setAuthlibInjectorServer()

	fmt.Println("安装完成")
}
