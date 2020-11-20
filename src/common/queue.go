package  common
import  (
   "os"
   "time"
)

type Queue struct  {
   pList  chan interface{}
   nCapacity  unit32
}

func  NewQueue(nCapacity unit32) *Queue{
    pQueue :=&Queue{}
    pQueue.nCapacity = nCapacity
    pQueue.plist = make(chan interface {},nCapacity )
    return  pQueue    
}

func  (this* Queue)  Release(){
    close(this.pList)
}

func  (this* Queue)  Push(obj interface{},nMilliSec unit32) bool {
    if obj==nil{
        return  false
    }
    select {
        case this.pList <-obj:{
             return  true
         }
        case  <-time.After(time.Millisecond*time.Duration(nMissiSec)):{
             return  false
         }
    }
}

(this* Queue)  Pop(nMilliSec unit32) (iterface {},bool) {
    
    select {
        case obj,ok ：=  <-this.pList :{
             if  !ok {
                 return  nil,false
              }
             return obj  true
         }
        case  <-time.After(time.Millisecond*time.Duration(nMissiSec)):{
             return  nil,false
         }
    }
}
