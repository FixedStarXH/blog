package utils

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type Response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data"`
}

func Success(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, Response{
		Code:    200,
		Message: "success",
		Data:    data,
	})
}

func Fail(c *gin.Context, message string) {
	c.JSON(http.StatusOK, Response{
		Code:    400,
		Message: message,
		Data:    nil,
	})
}

func Error(c *gin.Context, message string) {
	c.JSON(http.StatusInternalServerError, Response{
		Code:    500,
		Message: message,
		Data:    nil,
	})
}

// Unauthorized 401：未登录 / token 失效（鉴权中间件用）
// 业务 code 与 HTTP 状态码保持一致（401），前端 fetch 可以统一判断
func Unauthorized(c *gin.Context, message string) {
	c.JSON(http.StatusUnauthorized, Response{
		Code:    401,
		Message: message,
		Data:    nil,
	})
}

// Forbidden 403：已登录但权限不足（角色不够）
// 区别于 401：401 是"不知道你是谁"，403 是"知道你是谁但不让进"
func Forbidden(c *gin.Context, message string) {
	c.JSON(http.StatusForbidden, Response{
		Code:    403,
		Message: message,
		Data:    nil,
	})
}
