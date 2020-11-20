package  common
import(
    "time"
    "os"
)

func  BytesToFloat32(b []byte) float32{
   bytesBuffer :=bytes.NewBuffer(b)
   var  tmp  float32
   binary.Read(bytesBuffer,binary.BigEndian,&tmp)
   return  tmp
}
