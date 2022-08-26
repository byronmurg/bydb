package command

import (
	s "strings"
	"regexp"
	"errors"
	"strconv"
	"encoding/json"

	. "omanom.com/bydb/document"
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
	delRegex = regexp.MustCompile(`^DEL (\w+) (\S+) (\d+)$`)
	putRegex = regexp.MustCompile(`^PUT ({.*})$`)
	postRegex = regexp.MustCompile(`^POST ({.*})$`)
	searchRegex = regexp.MustCompile(`^SEARCH (\w+) (.+)$`)

	unknownCommandErr = errors.New("unknown command")
)

type Command struct {
	Type CommandType
	Doc *Document
	Query string
	Id string
	Part string
	Raw string
	StringDoc string
	BytesDoc []byte
	Index uint64
	Ts int64
}

func ParseCommand(rawMsg string) (*Command, error) {
	msg := s.TrimSpace(rawMsg)
	cmd := Command{
		Raw: rawMsg,
		Doc: &Document{},
	}

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
		ts, tsErr := strconv.ParseInt(parts[3], 10, 64)
		if tsErr != nil {
			return nil, tsErr
		}
		cmd.Ts = ts
	} else if putRegex.MatchString(msg) {
		parts := putRegex.FindStringSubmatch(msg)
		cmd.Type = PUT
		cmd.StringDoc = parts[1]
		cmd.BytesDoc = []byte(cmd.StringDoc)
		jsErr := json.Unmarshal(cmd.BytesDoc, &cmd.Doc)
		if jsErr != nil { return nil, jsErr }
		cmd.Part = cmd.Doc.Part
		cmd.Id = cmd.Doc.Id
		cmd.Ts = cmd.Doc.Updated
	} else if postRegex.MatchString(msg) {
		parts := postRegex.FindStringSubmatch(msg)
		cmd.Type = POST
		cmd.StringDoc = parts[1]
		cmd.BytesDoc = []byte(cmd.StringDoc)
		jsErr := json.Unmarshal(cmd.BytesDoc, &cmd.Doc)
		if jsErr != nil { return nil, jsErr }
		cmd.Part = cmd.Doc.Part
		cmd.Id = cmd.Doc.Id
		cmd.Ts = cmd.Doc.Created //@TODO is this correct?
		// @TODO error if Created and Updated are not the same / greater
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
