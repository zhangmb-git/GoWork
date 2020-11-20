package main

import (
	"fmt"

	"client/protocol"
)

func TestAll(url string) {
	testNet := &protocol.TestNet{}
	testNet.Url = url
	fmt.Println("test")
	list := []protocol.TestHandler{
		&protocol.TestInfo{},
	}
	for _, i := range list {
		testNet.Send(i)
	}
}

func main() {
	// file, err := ini.Load("D:/GoWork/src/client/main/conf.ini")
	// if err != nil {
	// 	panic(fmt.Sprintf("ini load err:%s", err))
	// 	return
	// }
	// flag, _ := file.Section("server").Key("show").Int()
	// if flag == 1 {
	// 	fmt.Println("hello")

	// } else {

	// 	fmt.Println(" world")
	// }
	url := "http://localhost:8081/serve/newtest"
	TestAll(url)
}
