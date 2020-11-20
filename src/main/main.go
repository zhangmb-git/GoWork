package main

import (
	"fmt" //"os"  "time"
	"os"

	lua "github.com/yuin/gopher-lua"
	"github.com/yuin/gopher-lua/parse"
)

func main() {
	curpid := os.Getpid()
	fmt.Println(curpid)
	// ticker := time.NewTicker(1 * time.Second)
	// for _ = range ticker.C {
	// 	fmt.Println("test123")
	// }

	L := lua.NewState()
	defer L.Close()
	if err := L.DoString(`print("hello")`); err != nil {
		panic(err)
	}

	if err := L.DoFile("D:/GoWork/src/main/hello.lua"); err != nil {
		panic(err)
	}


}
