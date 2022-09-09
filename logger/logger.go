package logger

import (
	"log"
	"os"
)

var def = log.Default()

var debugEnv = os.Getenv("DEBUG")

func Test(prefix string) bool {
	return debugEnv == "true"
}

type Logger struct {
	u *log.Logger
	prefix string
	debug bool
}

func (s *Logger) Debug(p ...any) {
	if s.debug {
		s.u.Print(p...)
	}
}

func (s *Logger) Debugf(format string, p ...any) {
	if s.debug {
		s.u.Printf(format, p...)
	}
}

func (s *Logger) Log(p ...any) {
	s.u.Print(p...)
}

func (s *Logger) Logf(format string, p ...any) {
	s.u.Printf(format, p...)
}

func (s *Logger) Fatal(p ...any) {
	s.u.Fatal(p...)
}

func (s *Logger) Fatalf(format string, p ...any) {
	s.u.Fatalf(format, p...)
}

func (s *Logger) Extend(prefix string) *Logger {
	newPrefix := s.prefix +"."+ prefix
	inDebug := Test(prefix)

	return &Logger{
		prefix: "",
		u: log.New(s.u.Writer(), newPrefix+": ", s.u.Flags()),
		debug: inDebug,
	}
}

var defaultLogger = &Logger{
	prefix: "bydb",
	u: log.New(def.Writer(), def.Prefix()+"bydb", def.Flags()),
	debug: false,
}

func New(prefix string) *Logger {
	return defaultLogger.Extend(prefix)
}

