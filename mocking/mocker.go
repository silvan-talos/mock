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
		"argNames":  argNames,
		"retValues": returnValues,
	}
	return &mocker{
		templ: template.Must(template.New("mockFile").Funcs(funcMap).Parse(mockFileTemplate)),
	}
}
func argNames(f mock.Func) string {
	names := make([]string, 0)
	for _, rawArg := range strings.Split(f.Args, ", ") {
		argName := strings.Split(rawArg, " ")[0]
		names = append(names, argName)
	}
	return strings.Join(names, ", ")
}

func returnValues(f mock.Func) string {
	ret := make([]string, 0)
	for _, arg := range strings.Split(f.RetArgs, ", ") {
		retType := strings.Trim(arg, "())")
		var val string
		switch retType {
		case "error":
			val = "nil"
		case "int", "uint", "int16", "int32", "int64", "uint16", "uint32", "uint64":
			val = "1"
		case "float32", "float64":
			val = "1.1"
		case "string", "interface{}":
			val = "\"\""
		default:
			val = retType + "{}"
		}
		switch {
		case strings.Contains(retType, "[]"):
			val = fmt.Sprintf("%s{}", retType)
		case strings.Contains(retType, "*"):
			val = strings.Replace(retType, "*", "&", 1)
			if abbrev(val) != "" {
				val += "{}"
			}
		}
		ret = append(ret, val)
	}
	return strings.Join(ret, ", ")
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
	log.Printf("interface %s successfully mocked\n", intf)
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
