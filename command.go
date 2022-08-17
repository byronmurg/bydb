package main

import (
	s "strings"
	"regexp"
	"errors"
	"encoding/json"
)

type CommandType uint64

const (
	GET CommandType = iota
	PUT
	DEL
	POST
	SEARCH
)


var (
	getRegex = regexp.MustCompile(`^GET (\w+) (\S+)$`)
	delRegex = regexp.MustCompile(`^DEL (\w+) (\S+)$`)
	putRegex = regexp.MustCompile(`^PUT ({.*})$`)
	postRegex = regexp.MustCompile(`^POST ({.*})$`)
	searchRegex = regexp.MustCompile(`^SEARCH (\w+) (.+)$`)

	unknownCommandErr = errors.New("unknown command")
)

type Command struct {
	Type CommandType
	Doc Document
	Query string
	Id string
	Part string
}

func ParseCommand(rawMsg string) (*Command, error) {
	msg := s.TrimSpace(rawMsg)
	cmd := Command{}

	if getRegex.MatchString(msg) {
		parts := getRegex.FindStringSubmatch(msg)
		cmd.Type = GET
		cmd.Part = parts[1]
		cmd.Id = parts[2]

	} else if delRegex.MatchString(msg) {
		parts := delRegex.FindStringSubmatch(msg)
		cmd.Type = DEL
		cmd.Part = parts[1]
		cmd.Id = parts[2]

	} else if putRegex.MatchString(msg) {
		parts := putRegex.FindStringSubmatch(msg)
		cmd.Type = PUT
		jsErr := json.Unmarshal([]byte(parts[1]), &cmd.Doc)
		if jsErr != nil { return nil, jsErr }
		cmd.Part = cmd.Doc.Part
		cmd.Id = cmd.Doc.Id

	} else if postRegex.MatchString(msg) {
		parts := postRegex.FindStringSubmatch(msg)
		cmd.Type = POST
		jsErr := json.Unmarshal([]byte(parts[1]), &cmd.Doc)
		if jsErr != nil { return nil, jsErr }
		cmd.Part = cmd.Doc.Part
		cmd.Id = cmd.Doc.Id

	} else if searchRegex.MatchString(msg) {
		parts := searchRegex.FindStringSubmatch(msg)
		cmd.Type = SEARCH
		cmd.Part = parts[1]
		cmd.Query = parts[2]

	} else {
		return nil, unknownCommandErr
	}
	
	return &cmd, nil
}
