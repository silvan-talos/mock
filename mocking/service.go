package mocking

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"unicode"
)

var (
	ErrNotFound             = errors.New("interface not found")
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

	for intf, path := range pairs {
		wg.Add(1)
		go s.mock(intf, path, &wg)
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

func (s *service) mock(name, path string, wg *sync.WaitGroup) {
	defer wg.Done()
	fileBytes, err := os.ReadFile(path)
	if err != nil {
		log.Println("failed to read file:", err)
		return
	}
	mockDir := findMockFolder()
	filePath := fmt.Sprintf("%s/%s.go", mockDir, toCamel(name))
	file, err := os.Create(filePath)
	if err != nil {
		log.Println("failed to create file:", err)
		return
	}
	err = s.mocker.Mock(string(fileBytes), file, name)
	if err != nil {
		log.Println("failed to create mock:", err)
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
