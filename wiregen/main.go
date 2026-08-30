package main

import (
	"flag"
	"fmt"
	"os"
	"path"
	"reflect"
	"regexp"
	"strings"

	"github.com/mengelbart/moqtransport/internal/wire"
)

var msgs = []any{
	wire.Setup{},
	wire.GoAwayCtrl{},
	wire.GoAwayReq{},
	wire.Subscribe{},
	wire.SubscribeOk{},
	wire.Publish{},
	wire.PublishOk{},
	wire.PublishDone{},
	wire.Fetch{},
	wire.FetchOk{},
	wire.TrackStatus{},
	wire.PublishNamespace{},
	wire.SubscribeNamespace{},
	wire.SubscribeTracks{},
	wire.Namespace{},
	wire.NamespaceDone{},
	wire.PublishBlocked{},
	wire.RequestUpdate{},
	wire.RequestOk{},
	wire.RequestError{},

	wire.FetchHeader{},
	wire.Padding{},

	wire.SubgroupHeader{},
	wire.ObjectDatagram{},

	wire.KeyValuePair{},
	wire.Location{},
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
		format, err := generate(mt, "wire", fmt.Sprintf("_v%v", *version))
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
