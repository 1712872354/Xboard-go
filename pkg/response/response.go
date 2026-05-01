package response

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// PHP-compatible response format: { "status":"success"|"fail", "data":..., "message":"...", "error":null }

type ApiResponse struct {
	Status  string      `json:"status"`
	Message string      `json:"message"`
	Data    interface{} `json:"data"`
	Error   interface{} `json:"error"`
}

type PaginatedData struct {
	Total       int         `json:"total"`
	CurrentPage int         `json:"current_page"`
	PerPage     int         `json:"per_page"`
	LastPage    int         `json:"last_page"`
	Data        interface{} `json:"data"`
}

func Success(c *gin.Context, data interface{}) {
	if data == nil {
		data = struct{}{}
	}
	c.JSON(http.StatusOK, ApiResponse{
		Status:  "success",
		Message: "操作成功",
		Data:    data,
		Error:   nil,
	})
}

func SuccessWithMessage(c *gin.Context, message string, data interface{}) {
	if data == nil {
		data = struct{}{}
	}
	c.JSON(http.StatusOK, ApiResponse{
		Status:  "success",
		Message: message,
		Data:    data,
		Error:   nil,
	})
}

// Created - HTTP 201, response with data
func Created(c *gin.Context, data interface{}) {
	if data == nil {
		data = struct{}{}
	}
	c.JSON(http.StatusCreated, ApiResponse{
		Status:  "success",
		Message: "操作成功",
		Data:    data,
		Error:   nil,
	})
}

// NoContent - HTTP 204, empty body
func NoContent(c *gin.Context) {
	c.JSON(http.StatusNoContent, nil)
}

// Error - generic error helper
func Error(c *gin.Context, statusCode int, message string) {
	c.JSON(statusCode, ApiResponse{
		Status:  "fail",
		Message: message,
		Data:    nil,
		Error:   nil,
	})
}

func Fail(c *gin.Context, message string) {
	if message == "" {
		message = "操作失败"
	}
	c.JSON(http.StatusOK, ApiResponse{
		Status:  "fail",
		Message: message,
		Data:    nil,
		Error:   nil,
	})
}

func FailWithCode(c *gin.Context, httpCode int, message string) {
	if message == "" {
		message = "操作失败"
	}
	c.JSON(httpCode, ApiResponse{
		Status:  "fail",
		Message: message,
		Data:    nil,
		Error:   nil,
	})
}

func BadRequest(c *gin.Context, message string) {
	if message == "" {
		message = "参数错误"
	}
	Fail(c, message)
}

func Unauthorized(c *gin.Context, message ...string) {
	msg := "授权失败，请先登录"
	if len(message) > 0 && message[0] != "" {
		msg = message[0]
	}
	c.JSON(http.StatusUnauthorized, ApiResponse{
		Status:  "fail",
		Message: msg,
		Data:    nil,
		Error:   nil,
	})
}

func Forbidden(c *gin.Context, message ...string) {
	msg := "禁止访问"
	if len(message) > 0 && message[0] != "" {
		msg = message[0]
	}
	c.JSON(http.StatusForbidden, ApiResponse{
		Status:  "fail",
		Message: msg,
		Data:    nil,
		Error:   nil,
	})
}

func NotFound(c *gin.Context, message ...string) {
	msg := "没有找到该页面"
	if len(message) > 0 && message[0] != "" {
		msg = message[0]
	}
	c.JSON(http.StatusNotFound, ApiResponse{
		Status:  "fail",
		Message: msg,
		Data:    nil,
		Error:   nil,
	})
}

func InternalError(c *gin.Context, message string) {
	if message == "" {
		message = "服务器错误"
	}
	c.JSON(http.StatusInternalServerError, ApiResponse{
		Status:  "fail",
		Message: message,
		Data:    nil,
		Error:   nil,
	})
}

func Paginated(c *gin.Context, items interface{}, total int64, page, pageSize int) {
	lastPage := int(total) / pageSize
	if int(total)%pageSize > 0 {
		lastPage++
	}
	c.JSON(http.StatusOK, PaginatedData{
		Total:       int(total),
		CurrentPage: page,
		PerPage:     pageSize,
		LastPage:    lastPage,
		Data:        items,
	})
}
