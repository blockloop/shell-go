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
	opts = &options{}

	yellow = color.New(color.FgBlack, color.BgYellow).SprintfFunc()
)

func init() {
	getopt.BoolVarLong(&opts.useRegex, "regexp", 'R', "match pattern as regexp")
	getopt.BoolVar(&opts.noCase, 'i', "ignore case when searching")
	getopt.BoolVarLong(&opts.listFiles, "list", 'l', "only list files where match is found")
}

func main() {
	// var opts options
	pattern, files := parseArgs()
	var wg sync.WaitGroup

	wg.Add(len(files))
	for _, _file := range files {
		go func(f string) {
			processFile(f, pattern)
			wg.Done()
		}(_file)
	}

	wg.Wait()
}

func processFile(_file string, pattern string) {
	if _, err := os.Stat(_file); os.IsNotExist(err) {
		println("file does not exist or you do not have permission to access it:", _file)
		return
	}
	matches := make(chan *Match)

	go grepFile(_file, pattern, matches)

	if opts.listFiles {
		if <-matches != nil {
			println(_file)
			return
		}
	}

	first := true
	for match := range matches {
		if first {
			println()
			fmt.Printf("%s:\n", color.CyanString(_file))
			first = false
		}
		print(fmt.Sprintf("    %s: %s", color.YellowString(strconv.Itoa(match.Num)), match.Line))
	}
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
	linesChan := make(chan *Line)
	go readFile(_file, linesChan)

	for line := range linesChan {
		if opts.noCase {
			line.Text = strings.ToLower(line.Text)
			pattern = strings.ToLower(pattern)
		}
		if opts.useRegex {
			r := regexp.MustCompile(pattern)
			finds := r.FindAllString(line.Text, -1)
			if finds != nil {
				to <- &Match{Num: line.Num, MatchStr: finds[0], Line: line.Text}
			}
		} else if strings.Contains(line.Text, pattern) {
			to <- &Match{Num: line.Num, MatchStr: pattern, Line: line.Text}
		}
	}

	close(to)
}

func readFile(_file string, to chan<- *Line) {
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
			to <- &Line{Num: i, Text: string(line)}
		} else {
			break
		}
		i++
	}
	close(to)
}

// func grepLine(pattern string, from <-chan string, result chan<- bool) {
// 	var wg sync.WaitGroup

// 	for line := range from {
// 		wg.Add(1)

// 		go func(l string) {
// 			defer wg.Done()
// 			if strings.Contains(l, pattern) {
// 				result <- true
// 			}
// 		}(string(line))
// 	}

// 	wg.Wait()
// 	close(result)
// }

type options struct {
	useRegex  bool
	noCase    bool
	listFiles bool
}

type Line struct {
	Text string
	Num  int
}

type Match struct {
	Num      int
	MatchStr string
	Line     string
}
