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

	// Admin commands
	JOIN_NODE
)

type CommandParser = func(*Command, []string) (*Command, error)

type CommandFormat struct {
	Prefix string
	regexPattern *regexp.Regexp
	Pattern string
	parser CommandParser
}

func NewCommandFormat(prefix string, pattern string, parser CommandParser) *CommandFormat {
	r := regexp.MustCompile(pattern)

	return &CommandFormat{
		Prefix: prefix,
		regexPattern: r,
		Pattern: pattern,
		parser: parser,
	}
}

func (f *CommandFormat) Test(raw string) bool {
	return s.HasPrefix(raw, f.Prefix)
}

func (f *CommandFormat) Run(raw string) (*Command, error) {
	
	if ! f.regexPattern.MatchString(raw) {
		return nil, errors.New("invalid format for command "+ f.Prefix)
	}

	parts := f.regexPattern.FindStringSubmatch(raw)

	ret := &Command{
		Raw: raw,
		Doc: &Document{},
	}

	return f.parser(ret, parts)
}






var (
	CommandFormats = []*CommandFormat{

		// GET
		NewCommandFormat(
			"GET",
			`^GET (\S+) (\S+)$`,
			func(cmd *Command, parts []string) (*Command, error) {
				cmd.Type = GET
				cmd.Part = parts[1]
				cmd.Id = parts[2]
				return cmd, nil
			},
		),

		// DEL
		NewCommandFormat(
			"DEL",
			`^DEL (\S+) (\S+) (\d+)$`,
			func(cmd *Command, parts []string) (*Command, error) {
				cmd.Type = DEL
				cmd.Part = parts[1]
				cmd.Id = parts[2]
				ts, tsErr := strconv.ParseInt(parts[3], 10, 64)
				if tsErr != nil {
					return nil, tsErr
				}
				cmd.Ts = ts
				return cmd, nil
			},
		),


		// PUT
		NewCommandFormat(
			"PUT",
			`^PUT (\d+) ({.*})$`,
			func(cmd *Command, parts []string) (*Command, error) {
				cmd.Type = PUT
				cmd.StringDoc = parts[2]
				cmd.BytesDoc = []byte(cmd.StringDoc)
				jsErr := json.Unmarshal(cmd.BytesDoc, &cmd.Doc)
				if jsErr != nil { return nil, jsErr }
				cmd.Part = cmd.Doc.Part
				cmd.Id = cmd.Doc.Id
				ts, tsErr := strconv.ParseInt(parts[1], 10, 64)
				if tsErr != nil {
					return nil, tsErr
				}
				cmd.Ts = ts
				return cmd, nil
			},
		),

		// POST
		NewCommandFormat(
			"POST",
			`^POST ({.*})$`,
			func(cmd *Command, parts []string) (*Command, error) {
				cmd.Type = POST
				cmd.StringDoc = parts[1]
				cmd.BytesDoc = []byte(cmd.StringDoc)
				jsErr := json.Unmarshal(cmd.BytesDoc, &cmd.Doc)
				if jsErr != nil { return nil, jsErr }
				cmd.Part = cmd.Doc.Part
				cmd.Id = cmd.Doc.Id
				cmd.Ts = cmd.Doc.Created //@TODO is this correct?
				// @TODO error if Created and Updated are not the same / greater
				return cmd, nil
			},
		),

		// SEARCH
		NewCommandFormat(
			"SEARCH",
			`^SEARCH (\S+) (.+)$`,
			func(cmd *Command, parts []string) (*Command, error) {
				cmd.Type = SEARCH
				cmd.Part = parts[1]
				cmd.Query = parts[2]
				return cmd, nil
			},
		),

		// JOIN_NODE
		NewCommandFormat(
			"JOIN_NODE",
			`^JOIN_NODE (\S+)$`,
			func(cmd *Command, parts []string) (*Command, error) {
				cmd.Type = JOIN_NODE
				cmd.Id = parts[1]
				return cmd, nil
			},
		),

	}
)



var (
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
	for _, cmdFormat := range CommandFormats {
		if cmdFormat.Test(rawMsg) {
			return cmdFormat.Run(rawMsg)
		}
	}
	return nil, unknownCommandErr
}
