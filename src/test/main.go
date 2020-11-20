package main

import (
	"fmt"
	"github.com/golang/protobuf/proto"
    // test.pb.go 的路径
    "test/pb"
)

// import (
// 	"github.com/cihub/seelog"
// 	"github.com/gin-gonic/gin"
// )

// func main() {
// 	r := gin.Default()
// 	r.GET("/ping", func(c *gin.Context) {
// 		c.JSON(200,
// 			gin.H{
// 				"message": "pong",
// 			})
// 	})
// 	seelog.Info("app run ")
// 	r.Run() // listen and serve on 0.0.0.0:8080
// 	seelog.Info("app over")
// }

func main() {
	fmt.Println("hello")
	test :=&pb.IM.test_msg{
        num:  5,
	}

	data = {}
	proto.Unmarshal(data,test_msg)

}
