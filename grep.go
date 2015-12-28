package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"code.google.com/p/getopt"
	"github.com/fatih/color"
)

var (
	opts = &Options{}

	yellowBg = color.New(color.FgBlack, color.BgYellow).SprintfFunc()
)

func init() {
	getopt.BoolVarLong(&opts.UseRegex, "regexp", 'R', "match pattern as regexp")
	getopt.BoolVar(&opts.NoCase, 'i', "ignore case when searching")
	getopt.BoolVarLong(&opts.ListFiles, "list", 'l', "only list files where match is found")
	getopt.BoolVarLong(&color.NoColor, "no-color", 'c', "don't colorize anything")
}

func main() {
	pattern, files := parseArgs()

	wg := &sync.WaitGroup{}
	wg.Add(len(files))

	for _, _file := range files {
		go func(f string) {
			results := processFile(f, pattern)
			if results != "" {
				println(results)
			}
			wg.Done()
		}(_file)
	}

	wg.Wait()
}

func processFile(_file string, pattern string) string {
	if _, err := os.Stat(_file); os.IsNotExist(err) {
		println("ERROR: file does not exist or you do not have permission to open it:", _file)
		return ""
	}
	matches := make(chan *Match)

	go grepFile(_file, pattern, matches)

	if opts.ListFiles {
		if <-matches != nil {
			return _file
		}
		// channel was close without any results so there is no match
		return ""
	}

	result := fmt.Sprintf("%s:\n", color.CyanString(_file))
	hasMatches := false
	for match := range matches {
		hasMatches = true
		lineNum := color.YellowString(strconv.Itoa(match.Line))
		text := strings.Replace(match.LineText, match.MatchStr, yellowBg(match.MatchStr), -1)
		result += fmt.Sprintf("    %s: %s", lineNum, text)
	}
	if hasMatches {
		return result
	}

	return ""
}

func parseArgs() (pattern string, files []string) {
	getopt.Parse()
	args := getopt.Args()

	if len(args) < 2 {
		// println("usage: grep [-abcDEFGHhIiJLlmnOoqRSsUVvwxZ] [-A num] [-B num] [-C[num]]")
		// println("        [-e pattern] [-f file] [--binary-files=value] [--color=when]")
		// println("        [--context[=num]] [--directories=action] [--label] [--line-buffered]")
		// println("        [--null] [pattern] [file ...]")
		getopt.Usage()
		os.Exit(1)
	}

	return args[0], args[1:]

	// pattern = os.Args[1]
	// files = os.Args[2:]
	// return files, pattern
}

func grepFile(_file string, pattern string, to chan<- *Match) {
	linesChan := make(chan *FileLine)
	go readFile(_file, linesChan)

	for line := range linesChan {
		if opts.NoCase {
			line.Text = strings.ToLower(line.Text)
			pattern = strings.ToLower(pattern)
		}
		if opts.UseRegex {
			r := regexp.MustCompile(pattern)
			finds := r.FindAllString(line.Text, -1)
			if finds != nil {
				to <- &Match{Line: line.Num, MatchStr: finds[0], LineText: line.Text}
			}
		} else if strings.Contains(line.Text, pattern) {
			to <- &Match{Line: line.Num, MatchStr: pattern, LineText: line.Text}
		}
	}

	close(to)
}

func readFile(_file string, to chan<- *FileLine) {
	f, err := os.Open(_file)
	if err != nil {
		log.Panic(err)
	}
	defer f.Close()

	freader := bufio.NewReader(f)
	i := 1
	for {
		line, er := freader.ReadBytes('\n')
		if er == nil {
			to <- &FileLine{Num: i, Text: string(line)}
		} else {
			break
		}
		i++
	}
	close(to)
}

// Options from the command line
type Options struct {
	UseRegex  bool
	NoCase    bool
	ListFiles bool
}

// FileLine is a line from a file
type FileLine struct {
	Text string
	Num  int
}

// Match is a matching line from a file
type Match struct {
	Line     int
	MatchStr string
	LineText string
}
