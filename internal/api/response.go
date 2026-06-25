package api

import "github.com/gin-gonic/gin"

type response struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data"`
}

func ok(c *gin.Context, data any) {
	c.JSON(200, response{Code: 0, Message: "ok", Data: data})
}

func fail(c *gin.Context, status int, err error) {
	c.JSON(status, response{Code: status, Message: err.Error(), Data: nil})
}
