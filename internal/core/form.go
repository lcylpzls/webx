package core

import (
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"reflect"
	"strconv"
	"strings"

	"github.com/lcylpzls/errx"
)

// CodeFormInvalid 表单绑定/文件处理失败。
const CodeFormInvalid errx.Code = "WEBX_FORM_INVALID"

func init() {
	errx.RegisterCode(CodeFormInvalid, "webx 表单绑定/文件处理失败")
	errx.RegisterCodeKind(CodeFormInvalid, errx.KindInvalid)
}

// openMultipartFile 打开上传文件（测试可替换以覆盖异常分支）。
var openMultipartFile = func(fh *multipart.FileHeader) (multipart.File, error) {
	return fh.Open()
}

// createFile 创建落盘文件（测试可替换以覆盖异常分支）。
var createFile = os.Create

// copyFile 复制文件内容（测试可替换以覆盖异常分支）。
var copyFile = io.Copy

// BindForm 解析表单（multipart/form-data 或 application/x-www-form-urlencoded）到 out。
// 字段通过 form tag 映射，支持 string/int/uint/float/bool/[]string。
func (c *Context) BindForm(out any) error {
	body := c.request.Body
	if c.maxBody > 0 {
		body = http.MaxBytesReader(c.writer, body, c.maxBody)
	}
	c.request.Body = body
	if c.maxBody > 0 && c.request.ContentLength > c.maxBody {
		return &http.MaxBytesError{Limit: c.maxBody}
	}
	if err := c.request.ParseMultipartForm(32 << 20); err != nil && !errors.Is(err, http.ErrNotMultipart) {
		return err
	}
	return bindValues(out, c.request.Form, "form")
}

// FormFile 返回表单中指定名称的第一个上传文件。
// 请求体大小受 SetMaxBodyBytes 限制，超限返回 MaxBytesError。
func (c *Context) FormFile(name string) (*multipart.FileHeader, error) {
	body := c.request.Body
	if c.maxBody > 0 {
		body = http.MaxBytesReader(c.writer, body, c.maxBody)
	}
	c.request.Body = body
	if c.maxBody > 0 && c.request.ContentLength > c.maxBody {
		return nil, &http.MaxBytesError{Limit: c.maxBody}
	}
	if err := c.request.ParseMultipartForm(32 << 20); err != nil {
		return nil, err
	}
	files := c.request.MultipartForm.File[name]
	if len(files) == 0 {
		return nil, errx.NewCode(CodeFormInvalid, "webx：上传文件不存在："+name)
	}
	return files[0], nil
}

// SaveUploadedFile 将上传文件保存到 dest（沿用 SetMaxBodyBytes 限制）。
func (c *Context) SaveUploadedFile(fh *multipart.FileHeader, dest string) error {
	if fh == nil {
		return errx.NewCode(CodeFormInvalid, "webx：上传文件不能为空")
	}
	if c.maxBody > 0 && fh.Size > c.maxBody {
		return &http.MaxBytesError{Limit: c.maxBody}
	}
	src, err := openMultipartFile(fh)
	if err != nil {
		return err
	}
	defer src.Close()
	out, err := createFile(dest)
	if err != nil {
		return err
	}
	if _, err := copyFile(out, src); err != nil {
		_ = out.Close()
		return err
	}
	return out.Close()
}

// BindQuery 解析查询参数到 out。
// 字段通过 query tag 映射，支持 string/int/uint/float/bool/[]string。
func (c *Context) BindQuery(out any) error {
	return bindValues(out, c.request.URL.Query(), "query")
}

// bindValues 将 url.Values 按 tagName 绑定到结构体（支持嵌套结构体）。
func bindValues(out any, values url.Values, tagName string) error {
	return bindValuesRec(out, values, tagName, make(map[reflect.Type]bool))
}

// bindValuesRec 递归绑定，visited 用于防护循环引用。
func bindValuesRec(out any, values url.Values, tagName string, visited map[reflect.Type]bool) error {
	rv := reflect.ValueOf(out)
	if rv.Kind() != reflect.Ptr || rv.IsNil() {
		return errx.NewCode(CodeFormInvalid, "webx：绑定目标必须为非空指针")
	}
	rv = rv.Elem()
	rt := rv.Type()
	if visited[rt] {
		return nil
	}
	visited[rt] = true
	defer delete(visited, rt)
	for i := 0; i < rt.NumField(); i++ {
		sf := rt.Field(i)
		if !sf.IsExported() {
			continue
		}
		tag := sf.Tag.Get(tagName)
		if tag == "-" {
			continue
		}
		field := rv.Field(i)
		switch field.Kind() {
		case reflect.Struct:
			if err := bindValuesRec(field.Addr().Interface(), values, tagName, visited); err != nil {
				return err
			}
			continue
		case reflect.Ptr:
			if field.Type().Elem().Kind() == reflect.Struct {
				if field.IsNil() {
					continue
				}
				if err := bindValuesRec(field.Interface(), values, tagName, visited); err != nil {
					return err
				}
				continue
			}
		}
		if tag == "" {
			continue
		}
		defaultValue := ""
		if idx := strings.IndexByte(tag, ','); idx >= 0 {
			rest := tag[idx+1:]
			tag = tag[:idx]
			if strings.HasPrefix(rest, "default=") {
				defaultValue = rest[len("default="):]
			}
			if tag == "" || tag == "-" {
				continue
			}
		}
		vals, ok := values[tag]
		if !ok || len(vals) == 0 {
			if defaultValue != "" {
				vals = []string{defaultValue}
			} else {
				continue
			}
		}
		switch field.Kind() {
		case reflect.String:
			field.SetString(vals[0])
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			n, err := strconv.ParseInt(vals[0], 10, 64)
			if err != nil {
				return err
			}
			field.SetInt(n)
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			n, err := strconv.ParseUint(vals[0], 10, 64)
			if err != nil {
				return err
			}
			field.SetUint(n)
		case reflect.Float32, reflect.Float64:
			f, err := strconv.ParseFloat(vals[0], 64)
			if err != nil {
				return err
			}
			field.SetFloat(f)
		case reflect.Bool:
			b, err := strconv.ParseBool(vals[0])
			if err != nil {
				return err
			}
			field.SetBool(b)
		case reflect.Slice:
			if field.Type().Elem().Kind() != reflect.String {
				return errx.NewCode(CodeFormInvalid, "webx：表单切片仅支持 []string")
			}
			field.Set(reflect.ValueOf(append([]string(nil), vals...)))
		default:
			return errx.NewCode(CodeFormInvalid, "webx：表单字段类型不支持")
		}
	}
	return nil
}
