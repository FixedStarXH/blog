package middleware

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"

	"blog-system/model"

	"github.com/gin-gonic/gin"
)

// ============================================================
// 操作审计中间件
//
// 目标：把"谁在什么时候对什么做了什么"完整记录下来，可追溯。
//   - AdminAudit：挂在后台管理组，自动记录所有写操作（含失败）
//   - LoginAudit：挂在登录/注册/登出接口，记录认证尝试（防爆破留痕）
//
// 为什么用中间件而不是在每个 Controller 里手动写？
//   手动写容易漏（新增接口忘了加），中间件零侵入、全覆盖。
//   语义化动作名靠 FullPath 映射（见 auditActionMap），未匹配的路由
//   自动降级为 method + path 原文，不会漏记录。
// ============================================================

// captureWriter 包装 gin.ResponseWriter，捕获响应体
// （默认 ResponseWriter 只暴露状态码，读不到 body；
//
//	审计需要判断"操作成功还是失败"，所以把 body 先存一份）
type captureWriter struct {
	gin.ResponseWriter
	body []byte
}

func (w *captureWriter) Write(b []byte) (int, error) {
	w.body = append(w.body, b...)
	return w.ResponseWriter.Write(b)
}

func (w *captureWriter) WriteString(s string) (int, error) {
	w.body = append(w.body, s...)
	return w.ResponseWriter.WriteString(s)
}

// readBody 读取请求体并原样放回（Controller 还会再读一次）
func readBody(r *http.Request) []byte {
	if r.Body == nil {
		return nil
	}
	b, err := io.ReadAll(r.Body)
	if err != nil {
		return nil
	}
	r.Body = io.NopCloser(bytes.NewBuffer(b))
	return b
}

// writeAudit 落库一条审计记录（失败只告警，不影响业务）
func writeAudit(operatorID uint, username string, role int, action, method, path, targetType string, targetID uint, detail, ip string, ok bool) {
	if len(detail) > 500 {
		detail = detail[:500]
	}
	status := 1
	if !ok {
		status = 0
	}
	if err := model.DB.Create(&model.AuditLog{
		OperatorID: operatorID,
		Username:   username,
		Role:       role,
		Action:     action,
		Method:     method,
		Path:       path,
		TargetType: targetType,
		TargetID:   targetID,
		Detail:     detail,
		IP:         ip,
		Status:     status,
	}).Error; err != nil {
		println("[audit] 写入审计日志失败:", err.Error())
	}
}

// respCode 从捕获的响应体解析业务 code（utils 约定：200 成功）
func respCode(body []byte) (int, string) {
	if len(body) == 0 {
		return -1, ""
	}
	var resp struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	}
	if json.Unmarshal(body, &resp) != nil {
		return -1, ""
	}
	return resp.Code, resp.Message
}

// auditActionMap 路由模板(method + fullPath) → 中文动作名
// 只覆盖"写操作"，GET 等只读操作不在此列（也不会被记录）
var auditActionMap = map[string]string{
	"POST /api/admin/articles":            "新建文章",
	"PUT /api/admin/articles/:id":         "更新文章",
	"DELETE /api/admin/articles/:id":      "删除文章",
	"PUT /api/admin/articles/:id/approve": "审核通过文章",
	"PUT /api/admin/articles/:id/reject":  "驳回文章",
	"POST /api/admin/articles/batch":      "批量操作文章",
	"POST /api/admin/categories":          "新建分类",
	"PUT /api/admin/categories/:id":       "更新分类",
	"DELETE /api/admin/categories/:id":    "删除分类",
	"POST /api/admin/tags":                "新建标签",
	"PUT /api/admin/tags/:id":             "更新标签",
	"DELETE /api/admin/tags/:id":          "删除标签",
	"POST /api/admin/upload":              "上传图片",
	"DELETE /api/admin/images/:id":        "删除图片",
	"POST /api/admin/links":               "新建友链",
	"PUT /api/admin/links/:id":            "更新友链",
	"DELETE /api/admin/links/:id":         "删除友链",
	"PUT /api/admin/settings":             "更新站点设置",
	"PUT /api/admin/users/:id/status":     "修改用户状态",
	"PUT /api/admin/users/:id/role":       "修改用户角色",
	"PUT /api/admin/comments/:id/approve": "审核通过评论",
	"PUT /api/admin/comments/:id/reject":  "驳回评论",
	"DELETE /api/admin/comments/:id":      "删除评论",
	"PUT /api/admin/comments/:id/status":  "修改评论状态",
	"POST /api/admin/comments/batch":      "批量操作评论",
	"POST /api/admin/ai/summary":          "AI 生成摘要",
	"POST /api/admin/ai/polish":           "AI 润色",
	"POST /api/admin/ai/index/:id":        "重建文章索引",
	"POST /api/admin/ai/index-all":        "重建全部索引",
	"PUT /api/admin/password":             "修改后台密码",
	"DELETE /api/admin/audit-logs":        "清空审计日志",
}

// auditTargetMap 路由首段 → 目标类型中文名
var auditTargetMap = map[string]string{
	"articles":   "文章",
	"comments":   "评论",
	"users":      "用户",
	"categories": "分类",
	"tags":       "标签",
	"links":      "友链",
	"images":     "图片",
	"upload":     "图片",
	"settings":   "站点设置",
	"audit-logs": "审计日志",
}

// AdminAudit 后台写操作审计（挂在 admin 组，AuthRequired 之后）
func AdminAudit() gin.HandlerFunc {
	return func(c *gin.Context) {
		method := c.Request.Method
		if method == http.MethodGet || method == http.MethodHead || method == http.MethodOptions {
			c.Next()
			return
		}

		body := readBody(c.Request)
		cw := &captureWriter{ResponseWriter: c.Writer}
		c.Writer = cw
		c.Next()

		fullPath := c.FullPath()
		action := auditActionMap[method+" "+fullPath]
		if action == "" {
			action = method + " " + fullPath // 未映射的路由：降级为原文，保证不漏
		}
		targetType := ""
		if seg := routeFirstSeg(fullPath); seg != "" {
			targetType = auditTargetMap[seg]
			if targetType == "" {
				targetType = seg
			}
		}
		targetID := paramUint(c, "id")

		code, msg := respCode(cw.body)
		username := ""
		var u model.User
		if err := model.DB.Select("username").First(&u, GetUserID(c)).Error; err == nil {
			username = u.Username
		}
		detail := string(body)
		if detail == "" && msg != "" {
			detail = msg
		}
		writeAudit(GetUserID(c), username, GetRole(c), action, method, fullPath,
			targetType, targetID, detail, c.ClientIP(), code == 200)
	}
}

// LoginAudit 认证接口审计（登录/注册/登出，记录用户名与成败，防爆破留痕）
func LoginAudit() gin.HandlerFunc {
	return func(c *gin.Context) {
		body := readBody(c.Request)
		var req struct {
			Username string `json:"username"`
		}
		_ = json.Unmarshal(body, &req)

		cw := &captureWriter{ResponseWriter: c.Writer}
		c.Writer = cw
		c.Next()

		fullPath := c.FullPath()
		action := "登录"
		switch {
		case strings.Contains(fullPath, "admin/login"):
			action = "后台登录"
		case strings.Contains(fullPath, "register"):
			action = "用户注册"
		case strings.Contains(fullPath, "logout"):
			action = "退出登录"
		}
		code, msg := respCode(cw.body)
		writeAudit(0, req.Username, model.RoleGuest, action, c.Request.Method, fullPath,
			"auth", 0, msg, c.ClientIP(), code == 200)
	}
}

// routeFirstSeg 取路由模板第一段实体名（如 /api/admin/articles/:id/approve → articles）
// 跳过固定前缀 api/admin 与动态参数段
func routeFirstSeg(fullPath string) string {
	parts := strings.Split(strings.Trim(fullPath, "/"), "/")
	for _, p := range parts {
		if strings.HasPrefix(p, ":") {
			continue
		}
		if p == "api" || p == "admin" {
			continue
		}
		return p
	}
	return ""
}

// paramUint 安全解析路径参数为 uint（非法返回 0）
func paramUint(c *gin.Context, name string) uint {
	id, err := strconv.ParseUint(c.Param(name), 10, 64)
	if err != nil {
		return 0
	}
	return uint(id)
}
