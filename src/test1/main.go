package main

import ( "fmt"
       "sync"
)
//var global_map = make(map[string]string)
var global_map sync.Map


	func main(){
	fmt.Println("begin")
	global_map.Store("abc","hello")
	global_map.Range(func(k, v interface{}) bool {
		fmt.Println("iterate:", k, v)
		return true
	})
	fmt.Println(global_map)
}
