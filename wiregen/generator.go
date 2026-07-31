package main

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"reflect"
	"strings"
	"text/template"

	"golang.org/x/tools/imports"
)

var errInvalidPackage = errors.New("invalid package name")

type unknownProtoTypeErr string

func (e unknownProtoTypeErr) Error() string {
	return fmt.Sprintf("unknown proto type: %s", string(e))
}

type generator struct {
	pkg          string
	methodSuffix string

	buf bytes.Buffer
}

var appenderTemplates = map[string]*template.Template{
	"quicvarint": template.Must(template.New("quicvarint_append").Parse(`	buf = quicvarint.Append(buf, uint64(m.{{ .Field }}))
`)),

	"varint": template.Must(template.New("varint_append").Parse(`	buf = varint.Append(buf, uint64(m.{{ .Field }}))
`)),

	"tlv_bytes": template.Must(template.New("tlv_bytes_append").Parse(`	buf = varint.Append(buf, uint64(len(m.{{ .Field }})))
	buf = append(buf, m.{{ .Field }}...)
`)),

	"tlv_string": template.Must(template.New("tlv_bytes_append").Parse(`	buf = varint.Append(buf, uint64(len(m.{{ .Field }})))
	buf = append(buf, []byte(m.{{ .Field }})...)
`)),

	"ntlv_bytes": template.Must(template.New("ntlv_bytes_append").Parse(`	buf = varint.Append(buf, uint64(len(m.{{ .Field }})))
	for _, v := range m.{{ .Field }} {
		buf = varint.Append(buf, uint64(len(v)))
		buf = append(buf, v...)
	}
`)),

	"moq_kvp_list": template.Must(template.New("moq_kvp_list_append").Parse(`	buf = varint.Append(buf, uint64(len(m.{{ .Field }})))
	for _, v:= range m.{{ .Field }} {
		buf = varint.Append(buf, uint64(v.Type))
		if v.Type % 2 == 0 {
			buf = varint.Append(buf, uint64(v.Varint))
		} else {
			buf = varint.Append(buf, uint64(len(v.Bytes)))
			buf = append(buf, v.Bytes...)
		}
	}
`)),

	"moq_kvp_list_no_length": template.Must(template.New("moq_kvp_list_no_length_append").Parse(`	for _, v:= range m.{{ .Field }} {
		buf = varint.Append(buf, uint64(v.Type))
		if v.Type % 2 == 0 {
			buf = varint.Append(buf, uint64(v.Varint))
		} else {
			buf = varint.Append(buf, uint64(len(v.Bytes)))
			buf = append(buf, v.Bytes...)
		}
	}
`)),

	"bool": template.Must(template.New("bool_append").Parse(`	if m.{{ .Field }} {
		buf = append(buf, byte(1))
	} else {
		buf = append(buf, byte(0))
	}
`)),

	"moq_location": template.Must(template.New("moq_location_append").Parse(`	buf = m.{{ .Field }}.append(buf)
`)),
}

var parserTemplates = map[string]*template.Template{
	"quicvarint": template.Must(template.New("quicvarint_parse").Parse(`	m.{{ .Field }}, n, err = quicvarint.Parse(data)
	if err != nil {
		return err
	}
	data = data[n:]
`)),

	"varint": template.Must(template.New("varint_parse").Parse(`	m.{{ .Field }}, n, err = varint.Parse(data)
	if err != nil {
		return err
	}
	data = data[n:]
`)),

	"tlv_bytes": template.Must(template.New("tlv_bytes_parse").Parse(`	var {{ .Field }}Length uint64
	{{ .Field }}Length, n, err = varint.Parse(data)
	if err != nil {
		return err
	}
	data = data[n:]

	if len(data) < int({{ .Field }}Length) {
		return io.ErrUnexpectedEOF
	}
	m.{{ .Field }} = data[:{{ .Field }}Length]
	data = data[{{ .Field }}Length:]
`)),

	"tlv_string": template.Must(template.New("tlv_string_parse").Parse(`	var {{ .Field }}Length uint64
	{{ .Field }}Length, n, err = varint.Parse(data)
	if err != nil {
		return err
	}
	data = data[n:]

	if len(data) < int({{ .Field }}Length) {
		return io.ErrUnexpectedEOF
	}
	m.{{ .Field }} = string(data[:{{ .Field }}Length])
	data = data[{{ .Field }}Length:]
`)),

	"ntlv_bytes": template.Must(template.New("ntlv_bytes_parse").Parse(`	var num{{ .Field }} uint64
	num{{ .Field }}, n, err = varint.Parse(data)
	if err != nil {
		return err
	}
	data = data[n:]

	m.{{ .Field }} = make([][]byte, num{{ .Field }})
	for i := range num{{ .Field }} {
		var length uint64
		length, n, err = varint.Parse(data)
		if err != nil {
			return err
		}
		data = data[n:]

		if len(data) < int(length) {
			return io.ErrUnexpectedEOF
		}
		m.{{ .Field }}[i] = data[:length]
		data = data[length:]
	}
`)),

	"bool": template.Must(template.New("bool_parse").Parse(`	if len(data) < 1 {
		return io.ErrUnexpectedEOF
	}
	if data[0] > 1 {
		return errors.New("invalid bool flag value")
	}
	m.{{ .Field }} = data[0] > 0
	data = data[1:]
`)),

	"moq_kvp_list": template.Must(template.New("moq_kvp_list_parse").Parse(`	var num{{ .Field }} uint64
	num{{ .Field }}, n, err = varint.Parse(data)
	if err != nil {
		return err
	}
	data = data[n:]

	m.{{ .Field }} = make([]KeyValuePair, num{{ .Field }})
	for i := range num{{ .Field }} {
		typ, n, err := varint.Parse(data)
		if err != nil {
			return err
		}
		data = data[n:]

		if typ % 2 == 0 {
			val, n, err := varint.Parse(data)
			if err != nil {
				return err
			}
			m.{{ .Field }}[i] = KeyValuePair{
				Type: typ,
				Varint: val,
			}
			data = data[n:]
		} else {
			length, n, err := varint.Parse(data)
			if err != nil {
				return err
			}
			data = data[n:]
			if len(data) < int(length) {
				return io.ErrUnexpectedEOF
			}
			m.{{ .Field }}[i] = KeyValuePair{
				Type: typ,
				Bytes: data[:length],
			}
			data = data[length:]
		}
	}
`)),

	"moq_kvp_list_no_length": template.Must(template.New("moq_kvp_list_no_length_parse").Parse(`	m.{{ .Field }} = make([]KeyValuePair, 0)
	for len(data) > 0 {
		typ, n, err := varint.Parse(data)
		if err != nil {
			return err
		}
		data = data[n:]

		if typ % 2 == 0 {
			val, n, err := varint.Parse(data)
			if err != nil {
				return err
			}
			m.{{ .Field }} = append(m.{{ .Field }}, KeyValuePair{
				Type: typ,
				Varint: val,
			})
			data = data[n:]
		} else {
			length, n, err := varint.Parse(data)
			if err != nil {
				return err
			}
			data = data[n:]
			if len(data) < int(length) {
				return io.ErrUnexpectedEOF
			}
			m.{{ .Field }} = append(m.{{ .Field }}, KeyValuePair{
				Type: typ,
				Bytes: data[:length],
			})
			data = data[length:]
		}
	}
`)),

	"moq_location": template.Must(template.New("moq_location_parse").Parse(`	n, err = m.{{ .Field }}.parse(data)
	if err != nil {
		return err
	}
	data = data[n:]
`)),
}

func generate(typ reflect.Type, pkg, methodSuffix string) ([]byte, error) {
	g := generator{
		pkg:          pkg,
		methodSuffix: methodSuffix,
		buf:          bytes.Buffer{},
	}
	err := g.generateHeader()
	if err != nil {
		return nil, err
	}
	err = g.generateAppend(typ)
	if err != nil {
		return nil, err
	}
	err = g.generateParse(typ)
	if err != nil {
		return nil, err
	}
	return g.format(), nil
}

func (g *generator) generateHeader() error {
	if len(g.pkg) == 0 {
		return errInvalidPackage
	}
	g.printf(`// Code generated by \"protogen %s\"; DO NOT EDIT.

package %s

import (
	"errors"
	"io"

	"github.com/quic-go/quic-go/quicvarint"
)

`, strings.Join(os.Args[1:], " "), g.pkg)
	return nil
}

func (g *generator) generateAppend(typ reflect.Type) error {
	g.printf("func (m *%s) append%s(buf []byte) []byte {\n", typ.Name(), g.methodSuffix)
	for i := range typ.NumField() {
		f := typ.Field(i)
		proto, ok := f.Tag.Lookup("proto")
		if !ok {
			continue
		}
		data := map[string]string{
			"Field": f.Name,
		}
		tmpl, ok := appenderTemplates[proto]
		if !ok {
			return unknownProtoTypeErr(proto)
		}
		if err := tmpl.Execute(g, data); err != nil {
			return err
		}
	}
	g.printf("	return buf\n")
	g.printf("}\n")
	g.printf("\n")
	return nil
}

func (g *generator) generateParse(typ reflect.Type) error {
	g.printf(`func (m *%s) parse%s(data []byte) error {
	var err error
	var n int

`, typ.Name(), g.methodSuffix)

	for i := range typ.NumField() {
		f := typ.Field(i)
		proto, ok := f.Tag.Lookup("proto")
		if !ok {
			continue
		}
		data := map[string]string{
			"Field": f.Name,
		}
		tmpl, ok := parserTemplates[proto]
		if !ok {
			return unknownProtoTypeErr(proto)
		}
		if err := tmpl.Execute(g, data); err != nil {
			return err
		}
		g.printf("\n")
	}
	g.printf(`

	return nil
}
`)
	return nil
}

func (g *generator) printf(format string, args ...any) {
	fmt.Fprintf(&g.buf, format, args...)
}

func (g *generator) format() []byte {
	src, err := imports.Process("msg.go", g.buf.Bytes(), nil)
	if err != nil {
		fmt.Printf("error: %v\n", err)
		fmt.Println(g.buf.String())
		panic(err)
	}
	return src
}

func (g *generator) Write(buf []byte) (int, error) {
	return g.buf.Write(buf)
}
