package mocking

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"text/template"
	"unicode"

	"github.com/silvan-talos/mock"
)

var (
	ErrNotFound             = errors.New("no match between interface name and filepath found. Please add one manually")
	ErrMoreThanOneInterface = errors.New("more than one interface found, only one at a time is supported")
)

type Service interface {
	Process(interfaces []string, filePath string) error
	ProcessOne(input io.Reader, output io.Writer) error
}

type Mocker interface {
	Mock(in string, out io.Writer, intf string) error
}

type service struct {
	mocker Mocker
}

func NewService(m Mocker) Service {
	return &service{
		mocker: m,
	}
}

func (s *service) Process(interfaces []string, filePath string) error {
	pairs := make(map[string]string, 0)
	var mutex sync.RWMutex
	var wg sync.WaitGroup
	if filePath == "" {
		for _, intf := range interfaces {
			wg.Add(1)
			go s.findInterface(intf, pairs, &mutex, &wg)
		}
		wg.Wait()
	} else if interfaces == nil {
		err := s.findAllAt(filePath, pairs)
		if err != nil {
			log.Println("failed to find interfaces at", filePath, "err:", err)
			return err
		}
	} else {
		path := filePath
		if !strings.Contains(path, ".go") {
			path += ".go"
		}
		for _, intf := range interfaces {
			pairs[intf] = path
		}
	}
	if len(pairs) == 0 {
		return ErrNotFound
	}

	funcMap := template.FuncMap{
		"argNames":  ArgNames,
		"retValues": ReturnValues,
	}
	templ := template.Must(template.New("mockFile").Funcs(funcMap).Parse(mockFileTemplate))
	for intf, path := range pairs {
		wg.Add(1)
		go s.mock(intf, path, templ, &wg)
	}
	wg.Wait()
	return nil
}

func (s *service) ProcessOne(input io.Reader, output io.Writer) error {
	var b strings.Builder
	_, err := io.Copy(&b, input)
	if err != nil {
		log.Printf("error reading input: %s\n", err)
		return err
	}
	raw := b.String()
	intfs := s.findAllIn(raw)
	if len(intfs) > 1 {
		return ErrMoreThanOneInterface
	}
	log.Println("mocking interface:", intfs[0])
	err = s.mocker.Mock(raw, output, intfs[0])
	if err != nil {
		log.Printf("failed to mock interface %s, err: %s\n", intfs[0], err)
	}
	return nil
}

func (s *service) findInterface(name string, pairs map[string]string, mutex *sync.RWMutex, wg *sync.WaitGroup) {
	defer wg.Done()
	toFind := fmt.Sprintf("type %s interface", name)
	err := filepath.WalkDir(".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if !strings.Contains(path, ".go") {
			return nil
		}
		fileBytes, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if strings.Contains(string(fileBytes), toFind) {
			mutex.Lock()
			pairs[name] = path
			mutex.Unlock()
			return fs.SkipAll
		}
		return nil

	})
	if err != nil {
		log.Println("encounted error", err, ", interface name:", name)
	}
}

func (s *service) findAllAt(filePath string, pairs map[string]string) error {
	path := filePath
	if !strings.Contains(path, ".go") {
		path += ".go"
	}
	fileBytes, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	r := regexp.MustCompile(`type (\S+) interface`)
	matches := r.FindAllStringSubmatch(string(fileBytes), -1)
	if matches == nil {
		return nil
	}
	for _, intf := range matches {
		pairs[intf[1]] = path
	}
	return nil
}

func (s *service) mock(name, path string, templ *template.Template, wg *sync.WaitGroup) {
	defer wg.Done()
	fileBytes, err := os.ReadFile(path)
	if err != nil {
		return
	}
	pattern := fmt.Sprintf(`type %s interface {([\s\S]+?)\n}`, name)
	r := regexp.MustCompile(pattern)
	matches := r.FindStringSubmatch(string(fileBytes))
	if matches == nil {
		log.Printf("couldn't find interface %s at %s\n", name, path)
		return
	}
	methods := strings.TrimSpace(matches[1])
	mockStruct := mock.Structure{
		Name:       name,
		NameAbbrev: abbrev(name),
	}
	for _, method := range strings.Split(methods, "\n") {
		r := regexp.MustCompile(`(\w+)\((.*?)\)\s(.*)`)
		matches := r.FindStringSubmatch(strings.TrimSpace(method))
		if matches == nil {
			log.Println("couldn't find interface methods")
			continue
		}
		fn := mock.Func{
			Name:    matches[1],
			Args:    addNamesIfMissing(matches[2], hasNamedParams(method)),
			RetArgs: matches[3],
		}
		mockStruct.Methods = append(mockStruct.Methods, fn)

		log.Println("Name:", matches[1], "ARGS:", matches[2], "rets:", matches[3])
	}
	mockDir := findMockFolder()
	filePath := fmt.Sprintf("%s/%s.go", mockDir, toCamel(name))
	file, err := os.Create(filePath)
	if err != nil {
		log.Println("failed to create file:", err)
		return
	}
	err = templ.Execute(file, mockStruct)
	if err != nil {
		log.Println("failed to execute template:", err)
		return
	}
	err = exec.Command("gofmt", "-w", filePath).Run()
	if err != nil {
		log.Println("failed to format file:", err)
		return
	}
}

func (s *service) findAllIn(raw string) []string {
	r := regexp.MustCompile(`type (\S+) interface`)
	matches := r.FindAllStringSubmatch(raw, -1)
	if matches == nil {
		return nil
	}
	intfs := make([]string, 0, len(matches))
	for _, match := range matches {
		if match[1] != "" {
			intfs = append(intfs, match[1])
		}
	}
	return intfs
}

func findMockFolder() string {
	var mockDirPath string
	err := filepath.WalkDir(".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if strings.Contains(path, "mock") {
				mockDirPath = path
				return fs.SkipAll
			}
		}
		return nil
	})
	if err != nil {
		log.Println("error looking for mock folder:", err)
		return "."
	}
	if mockDirPath != "" {
		return mockDirPath
	}
	err = filepath.WalkDir("../", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if strings.Contains(path, "mock") {
				mockDirPath = path
				return fs.SkipAll
			}
		}
		return nil
	})
	if err != nil {
		log.Println("error looking for mock folder in parent:", err)
		return "."
	}
	return mockDirPath
}

func toCamel(s string) string {
	r := []rune(s)
	if len(r) == 0 {
		log.Println("empty rune array string:", s)
		return ""
	}
	r[0] = unicode.ToLower(r[0])
	return string(r)
}

func abbrev(s string) string {
	var abb string
	for _, r := range s {
		if unicode.IsUpper(r) {
			abb += string(unicode.ToLower(r))
		}
	}
	return abb
}

func ArgNames(f mock.Func) string {
	names := make([]string, 0)
	for _, rawArg := range strings.Split(f.Args, ", ") {
		argName := strings.Split(rawArg, " ")[0]
		names = append(names, argName)
	}
	return strings.Join(names, ", ")
}

func ReturnValues(f mock.Func) string {
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
