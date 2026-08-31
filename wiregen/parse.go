package main

import (
	"text/template"
)

var parserTemplates = map[string]*template.Template{
	"quicvarint": template.Must(template.New("quicvarint_parse").Parse(`	m.{{ .Field }}, err = quicvarint.Read(r)
	if err != nil {
		return err
	}
`)),

	"varint": template.Must(template.New("varint_parse").Parse(`	m.{{ .Field }}, err = varint.Read(r)
	if err != nil {
		return err
	}
`)),

	"byte": template.Must(template.New("byte_parse").Parse(`	m.{{ .Field }}, err = r.ReadByte()
	if err != nil {
		return err
	}
`)),

	"tlv_bytes": template.Must(template.New("tlv_bytes_parse").Parse(`	var {{ .Field }}Length uint64
	{{ .Field }}Length, err = varint.Read(r)
	if err != nil {
		return err
	}

	m.{{ .Field }}, err = readBytes(r, {{ .Field }}Length)
	if err != nil {
		return err
	}
`)),

	"tlv_string": template.Must(template.New("tlv_string_parse").Parse(`	var {{ .Field }}Length uint64
	{{ .Field }}Length, err = varint.Read(r)
	if err != nil {
		return err
	}

	var {{ .Field }}Bytes []byte
	{{ .Field }}Bytes, err = readBytes(r, {{ .Field }}Length)
	if err != nil {
		return err
	}
	m.{{ .Field }} = string({{ .Field }}Bytes)
`)),

	"remaining_bytes": template.Must(template.New("remaining_bytes_parse").Parse(`	m.{{ .Field }}, err = readRemaining(r)
	if err != nil {
		return err
	}
`)),

	"ntlv_bytes": template.Must(template.New("ntlv_bytes_parse").Parse(`	var num{{ .Field }} uint64
	num{{ .Field }}, err = varint.Read(r)
	if err != nil {
		return err
	}

	m.{{ .Field }} = make([][]byte, 0)
	for range num{{ .Field }} {
		var length uint64
		length, err = varint.Read(r)
		if err != nil {
			return err
		}

		var value []byte
		value, err = readBytes(r, length)
		if err != nil {
			return err
		}
		m.{{ .Field }} = append(m.{{ .Field }}, value)
	}
`)),

	"bool": template.Must(template.New("bool_parse").Parse(`	var {{ .Field }}Byte byte
	{{ .Field }}Byte, err = r.ReadByte()
	if err != nil {
		return err
	}
	if {{ .Field }}Byte > 1 {
		return errors.New("invalid bool flag value")
	}
	m.{{ .Field }} = {{ .Field }}Byte > 0
`)),

	"message": template.Must(template.New("message_parse").Parse(`	if err = m.{{ .Field }}.parse{{ .Suffix }}(r); err != nil {
		return err
	}
`)),

	"kvp_list": template.Must(template.New("kvp_list_parse").Parse(`	m.{{ .Field }}, err = parseKeyValuePairsCount{{ .Suffix }}(r)
	if err != nil {
		return err
	}
`)),

	"kvp_list_tlv": template.Must(template.New("kvp_list_tlv_parse").Parse(`	m.{{ .Field }}, err = parseKeyValuePairsTLV{{ .Suffix }}(r)
	if err != nil {
		return err
	}
`)),

	"kvp_list_remaining": template.Must(template.New("kvp_list_remaining_parse").Parse(`	m.{{ .Field }}, err = parseKeyValuePairsRemaining{{ .Suffix }}(r)
	if err != nil {
		return err
	}
`)),
}

func (g *generator) generateParse(m message) error {
	g.printf("func (m *%s) parse%s(r messageReader) error {\n", m.name, g.methodSuffix)

	if len(m.fields) > 0 {
		g.printf("	var err error\n\n")
	}

	for _, f := range m.fields {
		if err := g.emit(parserTemplates, f); err != nil {
			return err
		}
		g.printf("\n")
	}
	g.printf(`	return nil
}
`)
	return nil
}
