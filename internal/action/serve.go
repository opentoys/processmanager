package action

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"processmanager/internal/utils"

	"github.com/urfave/cli/v3"
)

// ServeStaticAction 内置静态文件服务器（作为子进程运行）
func ServeStaticAction(ctx context.Context, cmd *cli.Command) error {
	dir := cmd.Args().First()
	if dir == "" {
		dir = "."
	}

	port := cmd.Int("port")
	if port <= 0 {
		port = 8080
	}

	upload := cmd.Bool("upload")
	key := cmd.String("key")
	prefix := "/" + key
	mux := http.NewServeMux()
	mux.HandleFunc(prefix+"/", func(w http.ResponseWriter, r *http.Request) {
		relPath := strings.TrimPrefix(r.URL.Path, prefix+"/")
		switch r.Method {
		case http.MethodGet:
			http.StripPrefix(prefix+"/", http.FileServer(http.Dir(dir))).ServeHTTP(w, r)
		case http.MethodDelete:
			if !upload {
				http.Error(w, "upload not enabled", http.StatusForbidden)
				return
			}
			fp := filepath.Join(dir, relPath)
			if !strings.HasPrefix(filepath.Clean(fp), filepath.Clean(dir)) {
				http.Error(w, "path traversal not allowed", http.StatusBadRequest)
				return
			}
			if err := os.Remove(fp); err != nil {
				http.Error(w, "delete failed: "+err.Error(), http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprintf(w, `{"success":true,"message":"deleted %s"}`, relPath)
		case http.MethodPost:
			if !upload {
				http.Error(w, "upload not enabled", http.StatusForbidden)
				return
			}
			if err := r.ParseMultipartForm(32 << 20); err != nil {
				http.Error(w, "parse form error: "+err.Error(), http.StatusBadRequest)
				return
			}
			files := r.MultipartForm.File["files"]
			if len(files) == 0 {
				http.Error(w, "no files uploaded", http.StatusBadRequest)
				return
			}
			targetDir := filepath.Join(dir, relPath)
			if !strings.HasPrefix(filepath.Clean(targetDir), filepath.Clean(dir)) {
				http.Error(w, "path traversal not allowed", http.StatusBadRequest)
				return
			}
			os.MkdirAll(targetDir, 0755)
			var uploaded []string
			for _, fh := range files {
				name := filepath.Base(fh.Filename)
				dst := filepath.Join(targetDir, name)
				src, err := fh.Open()
				if err != nil {
					http.Error(w, "open file error: "+err.Error(), http.StatusInternalServerError)
					return
				}
				out, err := os.Create(dst)
				if err != nil {
					src.Close()
					http.Error(w, "create file error: "+err.Error(), http.StatusInternalServerError)
					return
				}
				if _, err = io.Copy(out, src); err != nil {
					src.Close()
					out.Close()
					os.Remove(dst)
					http.Error(w, "save file error: "+err.Error(), http.StatusInternalServerError)
					return
				}
				src.Close()
				out.Close()
				uploaded = append(uploaded, name)
			}
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprintf(w, `{"success":true,"message":"%d file(s) uploaded","files":%s}`,
				len(uploaded), toJSONStrings(uploaded))
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})

	addr := ":" + strconv.Itoa(port)
	fmt.Printf("Serving %s on http://localhost:%d%s/\n", dir, port, prefix)
	return http.ListenAndServe(addr, mux)
}

func toJSONStrings(ss []string) string {
	var b strings.Builder
	b.WriteByte('[')
	for i, s := range ss {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteByte('"')
		for _, c := range s {
			switch c {
			case '"', '\\':
				b.WriteByte('\\')
				b.WriteByte(byte(c))
			default:
				b.WriteByte(byte(c))
			}
		}
		b.WriteByte('"')
	}
	b.WriteByte(']')
	return b.String()
}

// GetServeStaticCommand 返回内置 serve-static 子命令
func GetServeStaticCommand() *cli.Command {
	return &cli.Command{
		Name:      "serve-static",
		Usage:     "Start a static file server (internal use)",
		Hidden:    true,
		ArgsUsage: "[directory]",
		Flags: []cli.Flag{
			&cli.IntFlag{
				Name:    "port",
				Aliases: []string{"p"},
				Usage:   "Port to listen on",
				Value:   8080,
			},
			&cli.BoolFlag{
				Name:    "upload",
				Aliases: []string{"u"},
				Usage:   "Enable file upload support",
				Value:   false,
			},
			&cli.StringFlag{
				Name:    "key",
				Aliases: []string{"k"},
				Usage:   "Access key as route prefix (default: random)",
			},
		},
		Action: ServeStaticAction,
	}
}

// ServeAction serve 命令 - 将静态服务器注册为 pm 管理的进程
func ServeAction(ctx context.Context, cmd *cli.Command) error {
	if !IsDaemonRunning() {
		return errors.New(utils.ProcessManagerName + " daemon is not running")
	}

	dir := cmd.Args().First()
	if dir == "" {
		dir = "."
	}
	dir, err := filepath.Abs(dir)
	if err != nil {
		return fmt.Errorf("invalid directory: %w", err)
	}

	if info, err := os.Stat(dir); err != nil || !info.IsDir() {
		return fmt.Errorf("directory %s does not exist", dir)
	}

	port := cmd.Int("port")
	if port <= 0 {
		port = 8080
	}

	name := cmd.String("name")
	if name == "" {
		name = "serve-" + strconv.Itoa(port)
	}

	// 构建启动参数
	args := []string{"serve-static", dir, "-p", strconv.Itoa(port)}
	if cmd.Bool("upload") {
		args = append(args, "-u")
	}
	if k := cmd.String("key"); k != "" {
		args = append(args, "-k", k)
	} else {
		args = append(args, "-k", utils.RandHash())
	}

	// 获取当前可执行文件的绝对路径
	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %w", err)
	}

	startArgs := map[string]any{
		"name":   name,
		"script": execPath,
		"args":   args,
	}

	resp, err := SendCommand("start", startArgs)
	if err != nil {
		return err
	}

	if !resp.Success {
		return errors.New(resp.Message)
	}

	fmt.Println(resp.Message)
	return nil
}

// GetServeCommand 返回 serve 命令
func GetServeCommand() *cli.Command {
	return &cli.Command{
		Name:      "serve",
		Usage:     "Start a static file server managed by pm",
		ArgsUsage: "[directory]",
		Flags: []cli.Flag{
			&cli.IntFlag{
				Name:    "port",
				Aliases: []string{"p"},
				Usage:   "Port to listen on",
				Value:   8080,
			},
			&cli.StringFlag{
				Name:    "name",
				Aliases: []string{"n"},
				Usage:   "Process name",
			},
			&cli.BoolFlag{
				Name:    "upload",
				Aliases: []string{"u"},
				Usage:   "Enable file upload support",
				Value:   false,
			},
			&cli.StringFlag{
				Name:    "key",
				Aliases: []string{"k"},
				Usage:   "Access key as route prefix (default: random)",
			},
		},
		Action: ServeAction,
	}
}
