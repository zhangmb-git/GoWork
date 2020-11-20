package protocol

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
)

type TestNet struct {
	Url string
}

type TestHandler interface {
	NewPacket() *TestPacket
	OnPacket(packet *TestPacket) error
}

func (net *TestNet) Send(handler TestHandler) {
	pkt := handler.NewPacket()
	sendData, err := json.Marshal(pkt)
	if err != nil {
		fmt.Println("get packet err:%s", err.Error())
		return
	}
	fmt.Println(net.Url)

	rsp, err := http.Post(net.Url, "application/json", bytes.NewReader(sendData))
	if err != nil {
		fmt.Println("post url:%s err:%s", net.Url, err.Error())
		return
	}

	defer func() {
		err = rsp.Body.Close()
		if err != nil {
			fmt.Println("rsp body close err:%s", err.Error())
		}
	}()

	fmt.Println("rsp body:%s", rsp.Body)
}

func (net *TestNet) OnRecv(rsp *http.Response, handler TestHandler) {
	if rsp == nil {
		fmt.Println(errors.New("rsp is nil"))
		return
	}

	if rsp.StatusCode != 200 {
		fmt.Printf("http rsp status:%s", rsp.Status)
		return
	}

	recvData, err := ioutil.ReadAll(rsp.Body)
	if err != nil {
		fmt.Printf("ioutil readall:%s", err.Error())
		return
	}

	recvMsg := &TestPacket{}
	err = json.Unmarshal(recvData, recvMsg)
	if err != nil {
		fmt.Println("unmarshal recvdata err %s", err.Error())
		return
	}

	if handler == nil {
		return
	}

	err = handler.OnPacket(recvMsg)
	if err != nil {
		fmt.Println("handle packet error:%s", err.Error())
		return
	}

	fmt.Println()
}
