package app

import (
	utils "codepass/util"
	"encoding/json"
	"fmt"
	"github.com/gin-gonic/gin"
	"net/http"
	"strconv"
	"strings"
)

// WorkspacesCreate 创建工作区
func (model *ServiceModel) WorkspacesCreate(c *gin.Context) {
	// 参数校验
	var (
		name   = c.Query("name")
		pass   = c.Query("pass")
		cpus   = c.Query("cpus")
		disk   = c.Query("disk")
		memory = c.Query("memory")
	)
	if name == "" {
		c.JSON(http.StatusOK, gin.H{
			"ret": 0,
			"msg": "工作区名称不能为空",
		})
		return
	}
	if !utils.Test(name, "^[a-zA-Z][a-zA-Z0-9_]*$") {
		c.JSON(http.StatusOK, gin.H{
			"ret": 0,
			"msg": "工作区名称只允许字母开头，数字、字母、下划线组成",
		})
		return
	}
	if pass == "" {
		pass = utils.GenerateString(16)
	}
	if !utils.Test(pass, "^[a-zA-Z0-9_]*$") {
		c.JSON(http.StatusOK, gin.H{
			"ret": 0,
			"msg": "工作区密码只允许数字、字母、下划线组成",
		})
		return
	}
	// 检测工作区是否已存在
	dirPath := utils.RunDir(fmt.Sprintf("/.codepass/workspaces/%s", name))
	if utils.IsDir(dirPath) {
		c.JSON(http.StatusOK, gin.H{
			"ret": 0,
			"msg": "工作区已存在",
		})
		return
	}
	// 生成创建脚本
	cmdFile := utils.RunDir(fmt.Sprintf("/.codepass/workspaces/%s/create.sh", name))
	logFile := utils.RunDir(fmt.Sprintf("/.codepass/workspaces/%s/create.log", name))
	err := utils.WriteFile(cmdFile, utils.TemplateContent(utils.CreateExecContent, map[string]any{
		"NAME": name,
		"PASS": pass,

		"CPUS":   cpus,
		"DISK":   disk,
		"MEMORY": memory,
	}))
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"ret": 0,
			"msg": "创建工作区失败",
			"data": gin.H{
				"err": err.Error(),
			},
		})
		return
	}
	// 执行创建脚本
	go func() {
		_, _ = utils.Cmd("-c", fmt.Sprintf("chmod +x %s", cmdFile))
		_, _ = utils.Cmd("-c", fmt.Sprintf("/bin/sh %s > %s 2>&1", cmdFile, logFile))
		_ = updateDomain()
	}()
	//
	c.JSON(http.StatusOK, gin.H{
		"ret": 1,
		"msg": "创建工作区成功",
		"data": gin.H{
			"name": name,
			"pass": pass,
		},
	})
}

// WorkspacesCreateLog 查看创建日志
func (model *ServiceModel) WorkspacesCreateLog(c *gin.Context) {
	name := c.Query("name")
	tail, _ := strconv.Atoi(c.Query("tail"))
	if tail <= 0 {
		tail = 200
	}
	if tail > 10000 {
		tail = 10000
	}
	logFile := utils.RunDir(fmt.Sprintf("/.codepass/workspaces/%s/create.log", name))
	createFile := utils.RunDir(fmt.Sprintf("/.codepass/workspaces/%s/create", name))
	if !utils.IsFile(logFile) {
		c.JSON(http.StatusOK, gin.H{
			"ret": 0,
			"msg": "日志文件不存在",
		})
		return
	}
	logContent, _ := utils.Cmd("-c", fmt.Sprintf("tail -%d %s", tail, logFile))
	c.JSON(http.StatusOK, gin.H{
		"ret": 1,
		"msg": "读取成功",
		"data": gin.H{
			"create": strings.TrimSpace(utils.ReadFile(createFile)),
			"log":    strings.TrimSpace(logContent),
		},
	})
}

// WorkspacesList 获取工作区列表
func (model *ServiceModel) WorkspacesList(c *gin.Context) {
	list := workspacesList()
	if list == nil {
		c.JSON(http.StatusOK, gin.H{
			"ret": 0,
			"msg": "暂无数据",
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"ret": 1,
		"msg": "获取成功",
		"data": gin.H{
			"list": list,
		},
	})
}

// WorkspacesInfo 查看工作区信息
func (model *ServiceModel) WorkspacesInfo(c *gin.Context) {
	name := c.Query("name")
	dirPath := utils.RunDir(fmt.Sprintf("/.codepass/workspaces/%s", name))
	if !utils.IsDir(dirPath) {
		c.JSON(http.StatusOK, gin.H{
			"ret": 0,
			"msg": "工作区不存在",
		})
		return
	}
	result, err := utils.Cmd("-c", fmt.Sprintf("multipass info %s --format json", name))
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"ret": 0,
			"msg": "获取失败",
			"data": gin.H{
				"err": err.Error(),
			},
		})
		return
	}
	var data infoModel
	if err = json.Unmarshal([]byte(result), &data); err != nil {
		c.JSON(http.StatusOK, gin.H{
			"ret": 0,
			"msg": "解析失败",
			"data": gin.H{
				"err": err.Error(),
			},
		})
		return
	}
	createFile := utils.RunDir(fmt.Sprintf("/.codepass/workspaces/%s/create", name))
	passFile := utils.RunDir(fmt.Sprintf("/.codepass/workspaces/%s/pass", name))
	c.JSON(http.StatusOK, gin.H{
		"ret": 1,
		"msg": "获取成功",
		"data": gin.H{
			"create": strings.TrimSpace(utils.ReadFile(createFile)),
			"pass":   strings.TrimSpace(utils.ReadFile(passFile)),
			"info":   data.Info[name],
		},
	})
}

// WorkspacesDelete 删除工作区
func (model *ServiceModel) WorkspacesDelete(c *gin.Context) {
	name := c.Query("name")
	//
	dirPath := utils.RunDir(fmt.Sprintf("/.codepass/workspaces/%s", name))
	if utils.IsDir(dirPath) {
		_, _ = utils.Cmd("-c", fmt.Sprintf("rm -rf %s", dirPath)) // 删除工作区目录
	}
	_, err := utils.Cmd("-c", fmt.Sprintf("multipass info %s", name))
	if err == nil {
		_, err = utils.Cmd("-c", fmt.Sprintf("multipass delete --purge %s", name)) // 删除工作区
	}
	_ = updateDomain()
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"ret": 0,
			"msg": "工作区删除失败",
			"data": gin.H{
				"err": err.Error(),
			},
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"ret": 1,
		"msg": "工作区删除成功",
	})
}
