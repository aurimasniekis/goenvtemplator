package main

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	sprig "github.com/Masterminds/sprig/v3"
	"io"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"text/template"
	"unicode"
)

func Require(arg string) (string, error) {
	_, _ = fmt.Fprintf(os.Stderr, "WARNING: require built-in function is deprecated. Use required instead.\n")
	if len(arg) == 0 {
		return "", errors.New("Required argument is missing or empty!")
	}
	return arg, nil
}

// copied from Helm source:
// https://github.com/kubernetes/helm/blob/78d6b930bd325ed87b297c57b02fc7c9c7dfcfac/pkg/engine/engine.go#L156-L165
func Required(warn string, val interface{}) (interface{}, error) {
	if val == nil {
		return val, fmt.Errorf(warn)
	} else if _, ok := val.(string); ok {
		if val == "" {
			return val, fmt.Errorf(warn)
		}
	}
	return val, nil
}

func EnvAll() (map[string]string, error) {
	res := make(map[string]string)

	for _, item := range os.Environ() {
		split := strings.Split(item, "=")
		res[split[0]] = strings.Join(split[1:], "=")
	}

	return res, nil
}

func getArrayValues(funcName string, entries interface{}) (*reflect.Value, error) {
	entriesVal := reflect.ValueOf(entries)

	kind := entriesVal.Kind()

	if kind == reflect.Ptr {
		entriesVal = entriesVal.Elem()
		kind = entriesVal.Kind()
	}

	switch kind {
	case reflect.Array, reflect.Slice:
		break
	default:
		return nil, fmt.Errorf("must pass an array or slice to '%v'; received %v; kind %v", funcName, entries, kind)
	}
	return &entriesVal, nil
}

// PathExists returns whether the given file or directory exists or not
func pathExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

func isBlank(str string) bool {
	for _, r := range str {
		if !unicode.IsSpace(r) {
			return false
		}
	}
	return true
}

func removeBlankLines(reader io.Reader, writer io.Writer) {
	breader := bufio.NewReader(reader)
	bwriter := bufio.NewWriter(writer)

	prevLineBlank := true

	for {
		line, err := breader.ReadString('\n')

		if err != nil {
			break
		}

		if isBlank(line) {
			if prevLineBlank {
				continue
			}
			prevLineBlank = true
		} else {
			prevLineBlank = false
		}

		_, err = bwriter.WriteString(line)
		if err != nil {
			break
		}
	}

	err := bwriter.Flush()
	if err != nil {
		return
	}
}

func generateTemplate(source, name string, delimLeft string, delimRight string) ([]byte, error) {
	sprigFuncMap := sprig.TxtFuncMap()

	tmpl := template.New(name)
	eval := func(name string, args ...any) (string, error) {
		buf := bytes.NewBuffer(nil)
		data := any(nil)
		if len(args) == 1 {
			data = args[0]
		} else if len(args) > 1 {
			return "", errors.New("too many arguments")
		}
		if err := tmpl.ExecuteTemplate(buf, name, data); err != nil {
			return "", err
		}
		return buf.String(), nil
	}

	funcMap := template.FuncMap{
		"require":  Require,
		"envall":   EnvAll,
		"required": Required,

		"closest":               arrayClosest,
		"coalesce":              coalesce,
		"comment":               comment,
		"contains":              contains,
		"dir":                   dirList,
		"eval":                  eval,
		"exists":                pathExists,
		"groupBy":               groupBy,
		"groupByWithDefault":    groupByWithDefault,
		"groupByKeys":           groupByKeys,
		"groupByMulti":          groupByMulti,
		"include":               include,
		"intersect":             intersect,
		"keys":                  keys,
		"replace":               strings.Replace,
		"parseBool":             strconv.ParseBool,
		"fromYaml":              fromYaml,
		"toYaml":                toYaml,
		"mustFromYaml":          mustFromYaml,
		"mustToYaml":            mustToYaml,
		"queryEscape":           url.QueryEscape,
		"split":                 strings.Split,
		"splitN":                strings.SplitN,
		"sortStringsAsc":        sortStringsAsc,
		"sortStringsDesc":       sortStringsDesc,
		"sortObjectsByKeysAsc":  sortObjectsByKeysAsc,
		"sortObjectsByKeysDesc": sortObjectsByKeysDesc,
		"toLower":               strings.ToLower,
		"toUpper":               strings.ToUpper,
		"when":                  when,
		"where":                 where,
		"whereNot":              whereNot,
		"whereExist":            whereExist,
		"whereNotExist":         whereNotExist,
		"whereAny":              whereAny,
		"whereAll":              whereAll,

		// legacy docker-gen template function aliased to their Sprig clone
		"json":      sprigFuncMap["mustToJson"],
		"parseJson": sprigFuncMap["mustFromJson"],
		"sha1":      sprigFuncMap["sha1sum"],

		// aliases to sprig template functions masked by docker-gen functions with the same name
		"sprigCoalesce": sprigFuncMap["coalesce"],
		"sprigContains": sprigFuncMap["contains"],
		"sprigDir":      sprigFuncMap["dir"],
		"sprigReplace":  sprigFuncMap["replace"],
		"sprigSplit":    sprigFuncMap["split"],
		"sprigSplitn":   sprigFuncMap["splitn"],
	}

	var ctx struct {
		Env map[string]string
	}

	ctx.Env = make(map[string]string)
	for _, item := range os.Environ() {
		split := strings.Split(item, "=")
		ctx.Env[split[0]] = strings.Join(split[1:], "=")
	}

	var t *template.Template
	var err error
	t, err = template.New(name).Delims(delimLeft, delimRight).Funcs(funcMap).Funcs(sprig.TxtFuncMap()).Parse(source)
	if err != nil {
		return nil, err
	}
	var buffer bytes.Buffer
	if err = t.Execute(&buffer, &ctx); err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}

func generateFile(templatePath, destinationPath string, debugTemplates bool, delimLeft string, delimRight string, keepBlankLines bool) error {
	var slice []byte
	var err error
	if slice, err = os.ReadFile(templatePath); err != nil {
		return err
	}
	s := string(slice)
	result, err := generateTemplate(s, filepath.Base(templatePath), delimLeft, delimRight)
	if err != nil {
		return err
	}

	if debugTemplates {
		log.Printf("Printing parsed template to stdout. (It's delimited by 2 character sequence of '\\x00\\n'.)\n%s\x00\n", result)
	}

	if !keepBlankLines {
		buf := new(bytes.Buffer)
		removeBlankLines(bytes.NewReader(result), buf)
		result = buf.Bytes()
	}

	if err = os.WriteFile(destinationPath, result, 0664); err != nil {
		return err
	}

	return nil
}
