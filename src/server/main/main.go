package main

import ( 
    "fmt"
    "github.com/go-ini/ini"
     "test"  
)


func  main(){
     file,err:=ini.InsensitiveLoad("./conf.ini")
     if err!=nil{
         panic(fmt.Sprintf("ini load err:%s",err))
         return
     }
     flag,_ := file.Section("server").Key("show").Int()
     if flag==1 {
           fmt.Println("hello")

     } else {

            fmt.Println(" world")
      }
     test.Test()

    
}   
