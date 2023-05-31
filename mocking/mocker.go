package mocking

import (
	"bytes"
	_ "embed"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"io"
	"log"
	"regexp"
	"strings"
	"text/template"

	"github.com/silvan-talos/mock"
)

//go:embed mockFile.templ
var mockFileTemplate string

type mocker struct {
	templ *template.Template
}

func NewMocker() Mocker {
	funcMap := template.FuncMap{
		"argNames":  ArgNames,
		"retValues": ReturnValues,
	}
	return &mocker{
		templ: template.Must(template.New("mockFile").Funcs(funcMap).Parse(mockFileTemplate)),
	}
}

func (m *mocker) Mock(input string, out io.Writer, intf string) error {
	pattern := fmt.Sprintf(`type %s interface {([\s\S]+?)\n}`, intf)
	r := regexp.MustCompile(pattern)
	matches := r.FindStringSubmatch(input)
	if matches == nil {
		log.Printf("couldn't find interface %s\n", intf)
		return ErrNotFound
	}
	methods := strings.TrimSpace(matches[1])
	mockStruct := mock.Structure{
		Name:       intf,
		NameAbbrev: abbrev(intf),
	}
	for _, method := range strings.Split(methods, "\n") {
		r := regexp.MustCompile(`(\w+)\((.*?)\)\s(.*)`)
		matches := r.FindStringSubmatch(strings.TrimSpace(method))
		if matches == nil {
			log.Println("couldn't find any interface methods")
			return ErrNotFound
		}
		funcSignature := fmt.Sprintf("func (%s) %s", matches[2], matches[3])
		fn := mock.Func{
			Name:    matches[1],
			Args:    addNamesIfMissing(matches[2], hasNamedParams(funcSignature)),
			RetArgs: cleanupNames(matches[3]),
		}
		mockStruct.Methods = append(mockStruct.Methods, fn)

		log.Println("Name:", matches[1], "ARGS:", matches[2], "rets:", matches[3])
	}
	var buf bytes.Buffer
	err := m.templ.Execute(&buf, mockStruct)
	if err != nil {
		log.Println("failed to execute template:", err)
		return err
	}
	fmted, err := format.Source(buf.Bytes())
	if err != nil {
		log.Println("failed to format output:", err)
		log.Println("generated unformatted output", buf.String())
		return err
	}
	_, err = out.Write(fmted)
	if err != nil {
		log.Println("failed to write output:", err)
		return err
	}

	return nil
}

func addNamesIfMissing(s string, hasNamedParams bool) string {
	if s == "" {
		return ""
	}
	if hasNamedParams {
		return s
	}
	rawArgs := strings.Split(s, ", ")
	noOfArgs := len(rawArgs)
	args := make([]string, 0, noOfArgs)
	for i, rawArg := range rawArgs {
		arg := strings.Split(rawArg, " ")
		var argName, argType string
		if len(arg) < 2 {
			name := strings.Trim(arg[0], "[]*")
			if strings.Contains(arg[0], ".") {
				name = strings.Split(name, ".")[1]
			}
			argName = toCamel(name)
			if strings.Contains(arg[0], "[]") {
				argName += "s"
			}
			if argName != arg[0] {
				argType = arg[0]
			} else {
				argType = arg[0]
				argName = fmt.Sprintf("%c%d", argType[0], i)
			}
		}
		result := fmt.Sprintf("%s %s", argName, argType)
		args = append(args, result)
	}
	return strings.Join(args, ", ")
}

func hasNamedParams(sig string) bool {
	node, err := parser.ParseExpr(sig)
	if err != nil {
		log.Fatal("failed to parse signature", sig)
	}
	funcType, ok := node.(*ast.FuncType)
	if !ok {
		log.Fatal("not a function type", sig)
	}
	for _, field := range funcType.Params.List {
		if len(field.Names) > 0 {
			return true
		}
	}
	return false
}

func cleanupNames(s string) string {
	if s == "" {
		return ""
	}
	s = strings.Trim(s, "()")
	rawArgs := strings.Split(s, ", ")
	noOfArgs := len(rawArgs)
	args := make([]string, 0, noOfArgs)
	for _, rawArg := range rawArgs {
		arg := strings.Split(rawArg, " ")
		if len(arg) > 1 {
			args = append(args, arg[1])
		} else {
			args = append(args, arg[0])
		}
	}
	return fmt.Sprintf("(%s)", strings.Join(args, ", "))
}
