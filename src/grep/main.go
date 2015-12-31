package main

import (
	"bufio"
	"container/ring"
	"fmt"
	"os"
	"regexp"
	"strings"
	"sync"

	"code.google.com/p/getopt"
	"github.com/fatih/color"
)

var (
	isStdin   = false
	printSync = &sync.Mutex{}
	opts      = &Options{}

	yellowBg = color.New(color.FgBlack, color.BgYellow).SprintfFunc()
)

func init() {
	getopt.BoolVarLong(&opts.ShowHelp, "help", 'h', "show help information and usage")
	getopt.BoolVarLong(&opts.ShowVersion, "version", 'V', "show version information")
	getopt.BoolVarLong(&opts.Debug, "debug", 'D', "show debug information")

	getopt.BoolVarLong(&opts.UseRegex, "regexp", 'R', "match pattern as regexp")
	getopt.BoolVar(&opts.NoCase, 'i', "ignore case when searching")
	getopt.BoolVarLong(&opts.ListFiles, "list", 'l', "only list files where match is found")
	getopt.BoolVarLong(&color.NoColor, "no-color", 'c', "don't colorize anything")
	getopt.BoolVarLong(&opts.NoFileName, "no-filename", 'f', "don't output filenames")

	getopt.IntVarLong(&opts.Context, "context", 'C', "show N lines of context on each side")
	getopt.IntVarLong(&opts.BeforeContext, "before", 'B', "show N lines of context before matches")
	getopt.IntVarLong(&opts.AfterContext, "after", 'A', "show N lines of context after matches")
}

func main() {
	pattern, files := parseArgs()

	wg := &sync.WaitGroup{}
	wg.Add(len(files))

	for _, file := range files {
		go func(f *os.File) {
			processFile(f, pattern)
			wg.Done()
		}(file)
	}

	wg.Wait()
}

func processFile(file *os.File, pattern string) {
	matches := make(chan *Match)

	go grepFile(file, pattern, matches)

	if opts.ListFiles {
		// if a match is returned then print the file name and move on
		if <-matches != nil {
			printSync.Lock()
			fmt.Println(file)
			printSync.Unlock()
		}
		// channel was closed without any results so there is no match
		return
	}

	result := ""
	if !isStdin && !opts.NoFileName {
		result += fmt.Sprintf("%s:\n", color.GreenString(file.Name()))
	}
	hasMatches := false
	for match := range matches {
		hasMatches = true

		for _, l := range match.LinesBefore {
			if l != nil {
				result += fmt.Sprintf("  %d- %s\n", l.Num, strings.TrimSpace(l.Text))
			}
		}

		lineNum := color.YellowString("%d", match.Line.Num)
		text := strings.Replace(match.Line.Text, match.MatchStr, yellowBg(match.MatchStr), -1)
		result += fmt.Sprintf("  %s: %s\n", lineNum, strings.TrimSpace(text))

		for _, l := range match.LinesAfter {
			if l != nil {
				result += fmt.Sprintf("  %d- %s\n", l.Num, strings.TrimSpace(l.Text))
			}
		}
	}
	if hasMatches {
		printSync.Lock()
		fmt.Println(result)
		printSync.Unlock()
	}
}

func parseArgs() (pattern string, files []*os.File) {
	getopt.Parse()
	args := getopt.Args()

	if opts.ShowVersion {
		fmt.Println("grep-go")
		fmt.Println("version: 1.0")
		fmt.Println("author: Brett Jones")
		fmt.Println("source: https://github.com/blockloop/shell-go")
		os.Exit(0)
	}

	if opts.ShowHelp {
		getopt.Usage()
		os.Exit(0)
	}
	if len(args) == 0 {
		getopt.Usage()
		os.Exit(0)
	}
	if len(args) == 1 {
		isStdin = true
		files = append(files, os.Stdin)
	} else {
		for _, f := range args[1:] {
			if fl, err := os.Open(f); err == nil {
				files = append(files, fl)
			} else {
				fmt.Println("warning:", err.Error())
			}
		}
	}

	// this makes things easier later
	if opts.Context > 0 {
		opts.BeforeContext = opts.Context
		opts.AfterContext = opts.Context
	}
	return args[0], files
}

func grepFile(file *os.File, pattern string, to chan<- *Match) {
	if opts.UseRegex && opts.NoCase {
		pattern = "(?i)" + pattern
	}
	linesChan := make(chan *contextualLine)

	go readContextualFile(file, linesChan)

	for line := range linesChan {
		if line == nil || line.Current == nil {
			continue
		}

		if match := findMatch(line.Current.Text, pattern); match != "" {
			to <- &Match{
				MatchStr:    match,
				LinesBefore: line.LinesBefore,
				LinesAfter:  line.LinesAfter,
				Line: &fileLine{
					Text: line.Current.Text,
					Num:  line.Current.Num,
				},
			}
		}
	}

	close(to)
}

func findMatch(s string, pattern string) (match string) {
	if opts.UseRegex {
		r := regexp.MustCompile(pattern)
		finds := r.FindAllString(s, -1)
		if finds != nil {
			return finds[0]
		}
	} else {
		if opts.NoCase && strings.Contains(strings.ToLower(s), strings.ToLower(pattern)) {
			return pattern
		} else if strings.Contains(s, pattern) {
			return pattern
		}
	}
	return ""
}

type contextualLine struct {
	LinesBefore []*fileLine
	LinesAfter  []*fileLine
	Current     *fileLine
}

func readContextualFile(file *os.File, to chan<- *contextualLine) {
	// ring to hold buffer before and after and current line
	totalContext := opts.BeforeContext + opts.AfterContext
	buffer := ring.New(totalContext + 1)

	lineChan := make(chan *fileLine)
	go func() {
		readFile(file, lineChan)
		// when the file is finished being read the last N lines will remain in the AFTER position
		// so we push nils into the channel to move the last lines through the current line
		for i := 0; i < totalContext; i++ {
			lineChan <- nil
		}
		close(lineChan)
	}()

	for line := range lineChan {
		res := &contextualLine{}

		buffer.Value = line
		buffer = buffer.Next()

		if line != nil && line.Num <= totalContext {
			// don't have enough buffer, wait for more
			continue
		}

		// start with -1 because i++ is used at the beginning of the Do loop so it can't be missed
		i := -1
		buffer.Do(func(b interface{}) {
			i++
			var fl *fileLine
			if b != nil {
				fl = b.(*fileLine)
			}
			if i < opts.BeforeContext {
				res.LinesBefore = append(res.LinesBefore, fl)
				return
			}
			if i == opts.BeforeContext {
				res.Current = fl
				return
			}
			res.LinesAfter = append(res.LinesAfter, fl)
		})

		to <- res
	}
	close(to)
}

func readFile(file *os.File, to chan<- *fileLine) {
	defer file.Close()

	freader := bufio.NewReader(file)
	for i := 1; ; i++ {
		line, _, er := freader.ReadLine()
		if er != nil {
			break
		}
		to <- &fileLine{Num: i, Text: string(line)}
	}
}

// Options from the command line
type Options struct {
	ShowHelp      bool
	ShowVersion   bool
	UseRegex      bool
	NoCase        bool
	ListFiles     bool
	Context       int
	AfterContext  int
	BeforeContext int
	Debug         bool
	NoFileName    bool
}

type fileLine struct {
	Text string
	Num  int
}

// Match is a matching line from a file
type Match struct {
	Line        *fileLine
	MatchStr    string
	LinesBefore []*fileLine
	LinesAfter  []*fileLine
}
