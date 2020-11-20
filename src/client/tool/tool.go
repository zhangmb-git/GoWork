package tool

import (
	"bytes"
	"crypto/md5"
	"encoding/binary"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
)

//float  转byte
func Floated2ToBytes(n float32) []byte {
	bytesBuffer := bytes.NewBuffer([]byte{})
	binary.Write(bytesBuffer, binary.BigEndian, n)
	return bytesBuffer.Bytes()
}

func BytesToFloat32(b []byte) float32 {
	bytesBuffer := bytes.NewBuffer(b)
	var tmp float32
	binary.Read(bytesBuffer, binary.BigEndian, &tmp)
	return tmp
}

//生成32位md5
func MD5(byText []byte) string {
	ctx := md5.New()
	ctx.Write(byText)
	return hex.EncodeToString(ctx.Sum(nil))
}

//遍历
func traverDir(strPath, suffix string) (files []string, err error) {
	files = make([]string, 0, 30)
	err = filepath.Walk(strPath, func(filename string, fi os.FileInfo, err error) error {
		if fi.IsDir() {
			return nil
		}
		if strings.HasSuffix(fi.Name(), suffix) {
			files = append(files, filename)
		}
		return nil
	})
	return files, err

}
