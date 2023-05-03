package cmd

import (
	"codepass/app"
	utils "codepass/util"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/spf13/cobra"
	"github.com/unrolled/secure"
	"html/template"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"
)

var serviceCmd = &cobra.Command{
	Use:   "service",
	Short: "启动服务",
	PreRun: func(cmd *cobra.Command, args []string) {
		if !utils.CheckOs() {
			utils.PrintError("暂不支持的操作系统")
			os.Exit(1)
		}
		_, err := utils.Cmd("-c", "multipass version")
		if err != nil {
			utils.PrintError("未安装 multipass，请使用 ./codepass install 命令安装或手动安装")
			os.Exit(1)
		}
		err = utils.WriteFile(utils.RunDir("/.codepass/service"), utils.FormatYmdHis(time.Now()))
		if err != nil {
			utils.PrintError("无法写入文件")
			os.Exit(1)
		}
		if !utils.IsFile(app.ServiceConf.Key) {
			utils.PrintError("SSL私钥路径错误")
			os.Exit(1)
		}
		if !utils.IsFile(app.ServiceConf.Crt) {
			utils.PrintError("SSL证书路径错误")
			os.Exit(1)
		}
		app.UpdateProxy()
	},
	Run: func(cmd *cobra.Command, args []string) {
		router := gin.Default()
		templates, err := loadTemplate()
		if err != nil {
			utils.PrintError(err.Error())
			os.Exit(1)
		}
		router.SetHTMLTemplate(templates)
		//
		router.Any("/*path", func(c *gin.Context) {
			urlHost := c.Request.Host
			regFormat := fmt.Sprintf("^((\\d+)-)*([a-zA-Z][a-zA-Z0-9_]*)-code.%s", app.ServiceConf.Host)
			if utils.Test(urlHost, regFormat) {
				// 工作区实例
				reg := regexp.MustCompile(regFormat)
				match := reg.FindStringSubmatch(urlHost)
				port := match[2]
				name := match[3]
				lose := true
				for _, entry := range app.ProxyList {
					if entry.Name == name {
						c.Request.Header.Set("X-Real-Ip", c.ClientIP())
						c.Request.Header.Set("X-Forwarded-For", c.ClientIP())
						var targetUrl *url.URL
						if port == "" {
							targetUrl, _ = url.Parse(fmt.Sprintf("http://%s:55123", entry.Ip))
						} else {
							targetUrl, _ = url.Parse(fmt.Sprintf("http://%s:%s", entry.Ip, port))
						}
						proxy := httputil.NewSingleHostReverseProxy(targetUrl)
						proxy.ServeHTTP(c.Writer, c.Request)
						lose = false
						break
					}
				}
				if lose {
					if port == "" {
						c.String(http.StatusNotFound, fmt.Sprintf("%s not found", name))
					} else {
						c.String(http.StatusNotFound, fmt.Sprintf("%s(%s) not found", name, port))
					}
				}
			} else {
				// 接口、页面
				app.ServiceConf.OAuth(c)
			}
		})
		//
		router.Use(tlsHandler())
		err = router.RunTLS(fmt.Sprintf(":%s", app.ServiceConf.Port), app.ServiceConf.Crt, app.ServiceConf.Key)
		if err != nil {
			utils.PrintError(err.Error())
		}
	},
}

func loadTemplate() (*template.Template, error) {
	t := template.New("")
	for name, file := range Assets.Files {
		// 可以用.tmpl .html
		if file.IsDir() || !strings.HasSuffix(name, ".html") {
			continue
		}
		h, err := io.ReadAll(file)
		if err != nil {
			return nil, err
		}
		t, err = t.New(name).Parse(string(h))
		if err != nil {
			return nil, err
		}
	}
	return t, nil
}

func tlsHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		secureMiddleware := secure.New(secure.Options{
			SSLRedirect: true,
			SSLHost:     fmt.Sprintf("%s:%s", app.ServiceConf.Host, app.ServiceConf.Port),
		})
		err := secureMiddleware.Process(c.Writer, c.Request)
		if err != nil {
			return
		}
		c.Next()
	}
}

func init() {
	rootCmd.AddCommand(serviceCmd)
	serviceCmd.Flags().StringVar(&app.ServiceConf.Host, "host", "0.0.0.0", "主机地址或IP")
	serviceCmd.Flags().StringVar(&app.ServiceConf.Port, "port", "443", "服务端口")
	serviceCmd.Flags().StringVar(&app.ServiceConf.Key, "key", "", "SSL私钥路径(KEY)")
	serviceCmd.Flags().StringVar(&app.ServiceConf.Crt, "crt", "", "SSL证书路径(PEM格式)")
}
