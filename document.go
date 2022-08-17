package main

type Document struct {
	Id string `json:"id"`
	Part string `json:"part"`
	Index map[string]any `json:"index"`
	Data map[string]any `json:"data"`
}
