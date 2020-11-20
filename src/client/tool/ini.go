package tool

import (
	"fmt"

	"github.com/go-ini/ini"
)

type iniTool struct {
}

func (*iniTool) Test() {
	file, err := ini.InsensitiveLoad("./conf.ini")
	if err != nil {
		panic(fmt.Sprintf("ini load err:%s", err))

	}
	flag, _ := file.Section("server").Key("show").Int()
	if flag == 1 {
		fmt.Println("hello")

	} else {
		fmt.Println(" world")
	}
}
