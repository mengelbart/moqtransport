package main

import (
	"flag"
	"fmt"
	"os"
	"path"
	"regexp"
	"strings"
)

var (
	matchFirstCap = regexp.MustCompile("(.)([A-Z][a-z]+)")
	matchAllCap   = regexp.MustCompile("([a-z0-9])([A-Z])")
)

func toSnakeCase(str string) string {
	snake := matchFirstCap.ReplaceAllString(str, "${1}_${2}")
	snake = matchAllCap.ReplaceAllString(snake, "${1}_${2}")
	return strings.ToLower(snake)
}

func main() {
	version := flag.Int("version", 18, "version suffix for generated files and methods")
	directory := flag.String("dir", ".", "directory to read the messages from and save the generated files to")
	flag.Parse()

	pkg, msgs, err := loadPackage(*directory)
	if err != nil {
		panic(err)
	}

	for _, m := range msgs {
		format, err := generate(m, pkg, fmt.Sprintf("_v%v", *version))
		if err != nil {
			panic(err)
		}

		filename := toSnakeCase(m.name) + fmt.Sprintf("_v%v", *version) + ".go"
		filename = path.Join(*directory, filename)
		fmt.Println(filename)

		if err := os.WriteFile(filename, format, 0o644); err != nil {
			panic(err)
		}
	}
}
