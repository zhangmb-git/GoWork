package main

import (
	"flag"
	"os"

	"github.com/golang/glog"
)

func IsExist(fileName string) bool {
	_, err := os.Stat(fileName)
	return err == nil || os.IsExist(err)
}

func main() {
	flag.Parse()
	bExist := IsExist("D://a.txt")
	if bExist {
		glog.Info("exist")

	} else {
		glog.Error("not exist")
	}

	return
}
