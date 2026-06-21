// Package validator Package request 提供请求参数绑定与 v10 校验的便捷封装
// 统一用法、统一错误返回（调用方只需判断返回的 bool）。
package validator

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/go-playground/validator/v10"

	"trade-gateway/internal/server/http/response"
)

// 初始化 将 v10 的字段名映射到 tag 名（优先 json，其次 form）
// 这样无论是 JSON 还是 form/query，错误里的 field 都是前端认识的字段名
func init() {
	if v, ok := binding.Validator.Engine().(*validator.Validate); ok {
		v.RegisterTagNameFunc(func(fld reflect.StructField) string {
			// 优先使用 json 标签
			if tag := pickNameFromTag(fld.Tag.Get("json")); tag != "" {
				return tag
			}
			// 退化为 form（适用于 x-www-form-urlencoded、multipart、query）
			if tag := pickNameFromTag(fld.Tag.Get("form")); tag != "" {
				return tag
			}
			// 都没有则返回结构体字段名
			return fld.Name
		})
	}
}

// 从 struct tag 中提取字段名，处理 omitempty 等情况
func pickNameFromTag(tag string) string {
	if tag == "" {
		return ""
	}
	name := strings.Split(tag, ",")[0]
	if name == "" || name == "-" {
		return ""
	}
	return name
}

// BindQuery 仅绑定 URL Query 参数（GET/...?a=1&b=2），并做 v10 校验；失败时已返回 400。
func BindQuery[T any](c *gin.Context, dst *T) bool {
	if err := c.ShouldBindQuery(dst); err != nil {
		badRequest(c, err)
		return false
	}
	return true
}

// BindJSON 仅绑定 JSON Body（Content-Type: application/json），并做 v10 校验；失败时已返回 400。
func BindJSON[T any](c *gin.Context, dst *T) bool {
	if err := c.ShouldBindJSON(dst); err != nil {
		badRequest(c, err)
		return false
	}
	return true
}

// Bind 通用绑定（自动根据 Content-Type / 请求方式选择解析器），支持：
// - JSON: application/json
// - 表单: application/x-www-form-urlencoded, multipart/form-data
// - 其他 gin 内置格式
// 失败时已返回 400。
func Bind[T any](c *gin.Context, dst *T) bool {
	if err := c.ShouldBind(dst); err != nil {
		badRequest(c, err)
		return false
	}
	return true
}

// 统一处理绑定/校验错误，并返回规范化字段错误信息
func badRequest(c *gin.Context, err error) {
	var fieldErrs []gin.H
	if verrs, ok := err.(validator.ValidationErrors); ok {
		for _, fe := range verrs {
			// 简洁消息：field <name> failed on <tag>
			msg := fmt.Sprintf("field %s failed on %s", fe.Field(), fe.Tag())
			fieldErrs = append(fieldErrs, gin.H{
				"field": fe.Field(), // 此处已映射为 json/form 字段名
				"tag":   fe.Tag(),
				"param": fe.Param(),
				"msg":   msg,
			})
		}
	} else {
		fieldErrs = append(fieldErrs, gin.H{"msg": err.Error()})
	}
	response.BadRequest(c, "invalid parameters", gin.H{"errors": fieldErrs})
}
