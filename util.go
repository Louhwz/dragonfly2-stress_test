package main

import jsoniter "github.com/json-iterator/go"

func ToJsonString(i interface{}) string {
	s, _ := jsoniter.MarshalToString(i)
	return s
}

