package protocol

import (
	"encoding/json"
	"fmt"
)

type TestPacket struct {
	Head TestHeader      `json:"head"`
	Body json.RawMessage `json:"body"`
}

type TestHeader struct {
	PkgLen uint32
	CmdID  uint32
}

func NewPkt(cmdID uint32, pb interface{}) *TestPacket {
	head := &TestHeader{}
	fmt.Println(pb)
	data, err := json.Marshal(pb)
	if err != nil {
		panic(err)
	}
	fmt.Println(string(data))
	//body := []byte(data)
	// fmt.Println(body)
	paket := &TestPacket{
		Head: *head,
		Body: data,
	}
	return paket
}
