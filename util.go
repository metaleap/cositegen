package main

import (
	"crypto/md5"
	"crypto/sha1"
	"encoding/json"
	"io/ioutil"
	"os"
)

func contentHash(content []byte) []byte {
	b1, b2 := sha1.Sum(content), md5.Sum(content)
	return append(b1[:], b2[:]...)
}

func jsonLoad(filename string, intoPtr Any, defaultIfNotExist []byte) {
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		if defaultIfNotExist != nil && os.IsNotExist(err) {
			data = defaultIfNotExist
		} else {
			panic(err)
		}
	}
	if err = json.Unmarshal(data, intoPtr); err != nil {
		panic(err)
	}
}

func jsonSave(filename string, obj Any) {
	data, err := json.MarshalIndent(obj, "", "  ")
	if err != nil {
		panic(err)
	}
	if err = ioutil.WriteFile(filename, data, os.ModePerm); err != nil {
		panic(err)
	}
}
