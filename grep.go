package main

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"github.com/fatih/color"
)

var (
	useRegex bool

	yellow = color.New(color.FgBlack, color.BgYellow).SprintfFunc()
)

func init() {
	flag.BoolVar(&useRegex, "regexp", false, "match pattern as regexp")
}

func main() {
	// var opts options
	pat, files := parseArgs()
	var wg sync.WaitGroup

	resultChan := make(chan string)

	wg.Add(len(files))
	for _, _file := range files {
		go func(f string) {
			if _, err := os.Stat(f); os.IsNotExist(err) {
				println("file does not exist or you do not have permission to access it:", f)
			} else {
				grepFile(f, pat, resultChan)
			}
			wg.Done()
		}(_file)
	}

	go func() {
		wg.Wait()
		close(resultChan)
	}()

	for res := range resultChan {
		println(res)
	}

}

// func makeFlags() {
// 	flags.
// }

func parseArgs() (pat string, files []string) {
	flag.Parse()
	args := flag.Args()
	return args[0], args[1:]
	// if len(os.Args) < 3 {
	// 	println("usage: grep [-abcDEFGHhIiJLlmnOoqRSsUVvwxZ] [-A num] [-B num] [-C[num]]")
	// 	println("        [-e pattern] [-f file] [--binary-files=value] [--color=when]")
	// 	println("        [--context[=num]] [--directories=action] [--label] [--line-buffered]")
	// 	println("        [--null] [pattern] [file ...]")
	// 	os.Exit(1)
	// }

	// pat = os.Args[1]
	// files = os.Args[2:]
	// return files, pat
}

func getFiles(files []string, to chan<- string) {
}

func grepFile(_file string, pat string, to chan<- string) {
	var foundLines = ""
	linesChan := make(chan *Line)
	go readFile(_file, linesChan)

	for line := range linesChan {
		if useRegex {
			r := regexp.MustCompile(pat)
			if r.MatchString(line.Text) {
				num := color.YellowString(strconv.Itoa(line.Num))
				// text := r.ReplaceAllString(line.Text, yellow(pat))
				text := line.Text
				foundLines += fmt.Sprintf("    %s: %s", num, text)
			}
		} else if strings.Contains(line.Text, pat) {
			num := color.YellowString(strconv.Itoa(line.Num))
			text := strings.Replace(line.Text, pat, yellow(pat), -1)
			foundLines += fmt.Sprintf("    %s: %s", num, text)
		}
	}

	if foundLines != "" {
		fileColor := color.CyanString(_file)
		to <- fmt.Sprintf("%s: \n%s\n", fileColor, foundLines)
	}
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

// func grepLine(pat string, from <-chan string, result chan<- bool) {
// 	var wg sync.WaitGroup

// 	for line := range from {
// 		wg.Add(1)

// 		go func(l string) {
// 			defer wg.Done()
// 			if strings.Contains(l, pat) {
// 				result <- true
// 			}
// 		}(string(line))
// 	}

// 	wg.Wait()
// 	close(result)
// }

type options struct {
}

type Line struct {
	Num  int
	Text string
}
