package main

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
)

//go:embed input.json
var inputJSON []byte

type JSONWorkload struct{}

func (w JSONWorkload) Request() error {
	var m interface{}
	if err := json.Unmarshal(inputJSON, &m); err != nil {
		return err
	} else if err := json.NewEncoder(io.Discard).Encode(m); err != nil {
		return err
	}
	return nil
}

type PrintfWorkload struct{}

func (w PrintfWorkload) Request() error {
	for i := 0; i < 1e6; i++ {
		fmt.Fprintf(ioutil.Discard, "%d", i)
	}
	return nil
}
