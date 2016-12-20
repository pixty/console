package main

import "gopkg.in/gin-gonic/gin.v1"

type api struct {
	ginEngine *gin.Engine
}




func newAPI() *api {
	return &api{gin.New()}
}
