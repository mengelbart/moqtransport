package main

import (
	"flag"
	"fmt"
	"os"
	"path"
	"reflect"
	"regexp"
	"strings"

	"github.com/mengelbart/moqtransport/internal/wire2"
)

var msgs = []any{
	wire2.Setup{},
	wire2.GoAway{},
	wire2.Subscribe{},
	wire2.SubscribeOk{},
	wire2.Publish{},
	wire2.PublishOk{},
	wire2.PublishDone{},
	wire2.Fetch{},
	wire2.FetchOk{},
	wire2.TrackStatus{},
	wire2.PublishNamespace{},
	wire2.SubscribeNamespace{},
	wire2.SubscribeTracks{},
	wire2.Namespace{},
	wire2.NamespaceDone{},
	wire2.PublishBlocked{},
	wire2.RequestUpdate{},
	wire2.RequestOk{},
	wire2.RequestError{},
}

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
	directory := flag.String("dir", ".", "directory to save the generated files")
	flag.Parse()

	for _, m := range msgs {
		mt := reflect.TypeOf(m)
		format, err := generate(mt, "wire2", fmt.Sprintf("_v%v", *version))
		if err != nil {
			panic(err)
		}

		filename := toSnakeCase(mt.Name()) + fmt.Sprintf("_v%v", *version) + ".go"
		filename = path.Join(*directory, filename)
		fmt.Println(filename)

		if err := os.WriteFile(filename, format, 0o644); err != nil {
			panic(err)
		}
	}
}
