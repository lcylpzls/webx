package core

import (
	"errors"
	"net/http"
	"net/url"
	"reflect"
	"strconv"
)

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

// BindQuery 解析查询参数到 out。
// 字段通过 query tag 映射，支持 string/int/uint/float/bool/[]string。
func (c *Context) BindQuery(out any) error {
	return bindValues(out, c.request.URL.Query(), "query")
}

// bindValues 将 url.Values 按 tagName 绑定到结构体。
func bindValues(out any, values url.Values, tagName string) error {
	rv := reflect.ValueOf(out)
	if rv.Kind() != reflect.Ptr || rv.IsNil() {
		return errors.New("webx：绑定目标必须为非空指针")
	}
	rv = rv.Elem()
	rt := rv.Type()
	for i := 0; i < rt.NumField(); i++ {
		sf := rt.Field(i)
		if !sf.IsExported() {
			continue
		}
		tag := sf.Tag.Get(tagName)
		if tag == "" || tag == "-" {
			continue
		}
		vals, ok := values[tag]
		if !ok || len(vals) == 0 {
			continue
		}
		field := rv.Field(i)
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
				return errors.New("webx：表单切片仅支持 []string")
			}
			field.Set(reflect.ValueOf(append([]string(nil), vals...)))
		default:
			return errors.New("webx：表单字段类型不支持")
		}
	}
	return nil
}
