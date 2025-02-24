package utils

import (
	"encoding/json"
	"fmt"
	"net/http/httputil"

	"github.com/gin-gonic/gin"
)

func LimitConcurrent(maxConcurrent int, errorHandler gin.HandlerFunc) gin.HandlerFunc {
	semaphore := make(chan bool, maxConcurrent)
	return func(c *gin.Context) {
		select {
		case semaphore <- true:
			c.Next()
			<-semaphore
		default:
			errorHandler(c)
			return
		}
	}
}

func CheckParams(f interface{}, c *gin.Context) (occurred bool) {
	err := c.ShouldBind(f)
	if err != nil {
		s, _ := json.Marshal(f)
		c.AbortWithStatusJSON(400, gin.H{"code": 400, "msg": "参数不全", "debug": err.Error(), "data": string(s)})
		occurred = true
	}
	return
}

func AbortOn(err error, msg string, c *gin.Context) (occurred bool) {
	if err != nil {
		c.AbortWithStatusJSON(500, gin.H{"code": 500, "msg": msg, "debug": c.Request.Body, "data": nil})
		occurred = true
	}
	return
}

func ReturnErr(err error, c *gin.Context) {
	fmt.Println(err.Error())
	c.AbortWithStatusJSON(400, gin.H{"code": 400, "msg": err.Error(), "debug": err.Error(), "data": nil})
}

func NotFound(msg string, c *gin.Context) {
	c.AbortWithStatusJSON(404, gin.H{"code": 404, "msg": msg, "debug": nil, "data": nil})
}

func SuccessWith(msg string, c *gin.Context) {
	c.JSON(200, gin.H{"code": 200, "msg": msg, "debug": "", "data": nil})
}

func PatchRet(dt interface{}, c *gin.Context) {
	c.AbortWithStatusJSON(200, gin.H{"code": 200, "msg": "", "debug": nil, "data": dt})
}

func PatchErr(errMsg string, c *gin.Context) {
	c.AbortWithStatusJSON(400, gin.H{"code": 400, "msg": errMsg, "debug": c.Request.Body, "data": nil})
}

func Fuduji(c *gin.Context) {
	fmt.Println(c.Request.Host, c.Request.RemoteAddr, c.Request.RequestURI)
	requestDump, err := httputil.DumpRequest(c.Request, true)
	if err != nil {
		LogOn(err, "Fuduji failed to init.")
	}
	fmt.Println(string(requestDump))
	c.String(200, string(requestDump))
}
