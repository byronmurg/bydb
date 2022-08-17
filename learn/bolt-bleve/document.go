package main

type Document struct {
	Id string `json:"id"`
	Part string `json:"part"`
	Data map[string]any `json:"data"`
}
