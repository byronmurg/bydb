package main

import "testing"

func TestCommandParse(t *testing.T) {
	{
		cmd, err := ParseCommand("GET omanom 1234")
		if err != nil {
			t.Log("error should be nil", err)
			t.Fail()
		}

		if cmd == nil {
			t.Log("command should not be nil")
			t.Fail()
		}

		if cmd.Type != GET {
			t.Log("command should be a GET")
			t.Fail()
		}

		if cmd.Part != "omanom" {
			t.Log("part is empty")
			t.Fail()
		}
	}

	{
		cmd, err := ParseCommand("DEL omanom 1234")
		if err != nil {
			t.Log("error should be nil", err)
			t.Fail()
		}

		if cmd == nil {
			t.Log("command should not be nil")
			t.Fail()
		}

		if cmd.Type != DEL {
			t.Log("command should be a DEL")
			t.Fail()
		}

		if cmd.Part != "omanom" {
			t.Log("part is empty")
			t.Fail()
		}
	}

	{
		cmd, err := ParseCommand(`PUT { "id":"1234", "part":"omanom", "index":{ "foo":"bar" } }`)
		if err != nil {
			t.Log("error should be nil", err)
			t.Fail()
		}

		if cmd == nil {
			t.Log("command should not be nil")
			t.Fail()
		}

		if cmd.Type != PUT {
			t.Log("command should be a POST")
			t.Fail()
		}

		if cmd.Part != "omanom" {
			t.Log("part is empty")
			t.Fail()
		}

		if cmd.Doc.Index["foo"] != "bar" {
			t.Log("deserialization failed")
			t.Fail()
		}
	}

	{
		cmd, err := ParseCommand(`POST { "id":"1234", "part":"omanom", "index":{ "foo":"bar" } }`)
		if err != nil {
			t.Log("error should be nil", err)
			t.Fail()
		}

		if cmd == nil {
			t.Log("command should not be nil")
			t.Fail()
		}

		if cmd.Type != POST {
			t.Log("command should be a POST")
			t.Fail()
		}

		if cmd.Part != "omanom" {
			t.Log("part is empty")
			t.Fail()
		}

		if cmd.Doc.Index["foo"] != "bar" {
			t.Log("deserialization failed")
			t.Fail()
		}
	}

	{
		cmd, err := ParseCommand(`SEARCH omanom "some phrase" +foo`)
		if err != nil {
			t.Log("error should be nil", err)
			t.Fail()
		}

		if cmd == nil {
			t.Log("command should not be nil")
			t.Fail()
		}

		if cmd.Type != SEARCH {
			t.Log("command should be a SEARCH")
			t.Fail()
		}

		if cmd.Part != "omanom" {
			t.Log("part is empty")
			t.Fail()
		}

		if cmd.Query != `"some phrase" +foo` {
			t.Log("query parse failed", cmd.Query)
			t.Fail()
		}
	}
}
