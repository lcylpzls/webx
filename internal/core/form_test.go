package core

import (
	"bytes"
	"errors"
	testx "github.com/lcylpzls/testx"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type formModel struct {
	Name   string   `form:"name"`
	Age    int      `form:"age"`
	Active bool     `form:"active"`
	Tags   []string `form:"tags"`
	Skip   string   `form:"-"`
}

func TestBindFormURLEncoded(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/",
		strings.NewReader("name=webx&age=3&active=true&tags=a&tags=b&skip=1"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	c := NewContext(httptest.NewRecorder(), req)
	var m formModel
	if err := c.BindForm(&m); err != nil {
		t.Fatalf("BindForm 失败：%v", err)
	}
	if m.Name != "webx" || m.Age != 3 || !m.Active || len(m.Tags) != 2 || m.Tags[1] != "b" {
		t.Errorf("表单绑定不符：%+v", m)
	}
}

type formModelMore struct {
	Count uint    `form:"count"`
	Ratio float64 `form:"ratio"`
	On    bool    `form:"on"`
}

type formModelBad struct {
	IDs []int `form:"ids"`
}

type formModelUnsupported struct {
	T map[string]string `form:"t"`
}

type formModelHidden struct {
	hidden string `form:"hidden"`
}

func TestBindFormMoreTypes(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/",
		strings.NewReader("count=5&ratio=1.5&on=true"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	c := NewContext(httptest.NewRecorder(), req)
	var m formModelMore
	if err := c.BindForm(&m); err != nil {
		t.Fatalf("BindForm 失败：%v", err)
	}
	if m.Count != 5 || m.Ratio != 1.5 || !m.On {
		t.Errorf("更多类型绑定不符：%+v", m)
	}
}

func TestBindFormTypeErrors(t *testing.T) {
	cases := []struct {
		name string
		body string
		out  any
	}{
		{"uint 非法", "count=x", &formModelMore{}},
		{"float 非法", "ratio=x", &formModelMore{}},
		{"bool 非法", "on=x", &formModelMore{}},
		{"切片非字符串", "ids=1", &formModelBad{}},
		{"类型不支持", "t=x", &formModelUnsupported{}},
	}
	for _, tc := range cases {
		req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(tc.body))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		c := NewContext(httptest.NewRecorder(), req)
		if err := c.BindForm(tc.out); err == nil {
			t.Errorf("%s：应返回错误", tc.name)
		}
	}
}

func TestBindFormUnexportedField(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("hidden=x"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	c := NewContext(httptest.NewRecorder(), req)
	var m formModelHidden
	if err := c.BindForm(&m); err != nil {
		t.Fatalf("BindForm 失败：%v", err)
	}
	testx.Equal(t, m.hidden, "")

}

func TestBindFormMultipart(t *testing.T) {
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	_ = mw.WriteField("name", "多部分")
	_ = mw.WriteField("age", "7")
	_ = mw.Close()
	req := httptest.NewRequest(http.MethodPost, "/", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	c := NewContext(httptest.NewRecorder(), req)
	var m formModel
	if err := c.BindForm(&m); err != nil {
		t.Fatalf("BindForm 失败：%v", err)
	}
	if m.Name != "多部分" || m.Age != 7 {
		t.Errorf("multipart 绑定不符：%+v", m)
	}
}

func TestBindFormErrors(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("age=abc"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	c := NewContext(httptest.NewRecorder(), req)
	var m formModel
	if err := c.BindForm(&m); err == nil {
		t.Error("非法数值应报错")
	}
	if err := bindValues(nil, url.Values{}, "form"); err == nil {
		t.Error("nil 目标应报错")
	}
	if err := bindValues(m, url.Values{}, "form"); err == nil {
		t.Error("非指针目标应报错")
	}
}

func TestBindFormMaxBytes(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/",
		strings.NewReader("name="+strings.Repeat("x", 100)))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	c := NewContext(rec, req)
	c.SetMaxBodyBytes(10)
	var m formModel
	if err := c.BindForm(&m); err == nil {
		t.Error("超过最大请求体应报错")
	}
}

func TestBindFormMalformedMultipart(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("这不是 multipart 内容"))
	req.Header.Set("Content-Type", "multipart/form-data; boundary=x")
	c := NewContext(httptest.NewRecorder(), req)
	var m formModel
	if err := c.BindForm(&m); err == nil {
		t.Error("损坏的 multipart 应报错")
	}
}

type queryModel struct {
	Keyword string   `query:"keyword"`
	Page    int      `query:"page"`
	Limit   uint     `query:"limit"`
	Ratio   float64  `query:"ratio"`
	Active  bool     `query:"active"`
	Tags    []string `query:"tags"`
	Skip    string   `query:"-"`
}

func TestBindQuery(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/?keyword=webx&page=3&limit=10&ratio=1.5&active=true&tags=a&tags=b&skip=1", nil)
	c := NewContext(httptest.NewRecorder(), req)
	var m queryModel
	if err := c.BindQuery(&m); err != nil {
		t.Fatalf("BindQuery 失败：%v", err)
	}
	if m.Keyword != "webx" || m.Page != 3 || m.Limit != 10 || m.Ratio != 1.5 || !m.Active {
		t.Errorf("查询绑定不符：%+v", m)
	}
	if len(m.Tags) != 2 || m.Tags[1] != "b" || m.Skip != "" {
		t.Errorf("切片/忽略字段不符：%+v", m)
	}
}

type queryModelBad struct {
	IDs []int `query:"ids"`
}

type queryModelUnsupported struct {
	T map[string]string `query:"t"`
}

type queryModelHidden struct {
	hidden string `query:"hidden"`
}

func TestBindQueryErrors(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		out  any
	}{
		{"int 非法", "page=x", &queryModel{}},
		{"uint 非法", "limit=x", &queryModel{}},
		{"float 非法", "ratio=x", &queryModel{}},
		{"bool 非法", "active=x", &queryModel{}},
		{"切片非字符串", "ids=1", &queryModelBad{}},
		{"类型不支持", "t=x", &queryModelUnsupported{}},
	}
	for _, tc := range cases {
		req := httptest.NewRequest(http.MethodGet, "/?"+tc.raw, nil)
		c := NewContext(httptest.NewRecorder(), req)
		if err := c.BindQuery(tc.out); err == nil {
			t.Errorf("%s：应返回错误", tc.name)
		}
	}
	if err := bindValues(nil, url.Values{}, "query"); err == nil {
		t.Error("nil 目标应报错")
	}
}

func TestBindQueryUnexportedField(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/?hidden=x", nil)
	c := NewContext(httptest.NewRecorder(), req)
	var m queryModelHidden
	if err := c.BindQuery(&m); err != nil {
		t.Fatalf("BindQuery 失败：%v", err)
	}
	testx.Equal(t, m.hidden, "")

}

type defaultQueryModel struct {
	Page  int      `query:"page,default=10"`
	Limit uint     `query:"limit,default=20"`
	Name  string   `query:"name,default=匿名"`
	On    bool     `query:"on,default=true"`
	Tags  []string `query:"tags,default=a"`
	Weird string   `query:",default=x"`
	Dash  string   `query:"-,default=x"`
}

func TestBindQueryDefaults(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/?page=3&name=webx", nil)
	c := NewContext(httptest.NewRecorder(), req)
	var m defaultQueryModel
	if err := c.BindQuery(&m); err != nil {
		t.Fatalf("BindQuery 失败：%v", err)
	}
	if m.Page != 3 || m.Limit != 20 || m.Name != "webx" || !m.On {
		t.Errorf("默认值填充不符：%+v", m)
	}
	if len(m.Tags) != 1 || m.Tags[0] != "a" || m.Weird != "" || m.Dash != "" {
		t.Errorf("默认切片/忽略字段不符：%+v", m)
	}
}

type defaultFormModel struct {
	Age int `form:"age,default=18"`
}

func TestBindFormDefaults(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(""))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	c := NewContext(httptest.NewRecorder(), req)
	var m defaultFormModel
	if err := c.BindForm(&m); err != nil {
		t.Fatalf("BindForm 失败：%v", err)
	}
	if m.Age != 18 {
		t.Errorf("表单默认值未填充：%d", m.Age)
	}

	req = httptest.NewRequest(http.MethodPost, "/", strings.NewReader("age=30"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	c = NewContext(httptest.NewRecorder(), req)
	m = defaultFormModel{}
	if err := c.BindForm(&m); err != nil {
		t.Fatalf("BindForm 失败：%v", err)
	}
	if m.Age != 30 {
		t.Errorf("显式值应覆盖默认值：%d", m.Age)
	}
}

type defaultBadModel struct {
	X int `query:"x,default=abc"`
}

func TestBindQueryBadDefault(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	c := NewContext(httptest.NewRecorder(), req)
	var m defaultBadModel
	if err := c.BindQuery(&m); err == nil {
		t.Error("非法默认值应报错")
	}
}

func TestContextBindDispatch(t *testing.T) {
	// JSON
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"name":"webx"}`))
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	c := NewContext(httptest.NewRecorder(), req)
	var jm struct {
		Name string `json:"name"`
	}
	if err := c.Bind(&jm); err != nil || jm.Name != "webx" {
		t.Errorf("Bind JSON 不符：%v %+v", err, jm)
	}

	// 表单
	req = httptest.NewRequest(http.MethodPost, "/", strings.NewReader("name=form"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	c = NewContext(httptest.NewRecorder(), req)
	var fm struct {
		Name string `form:"name"`
	}
	if err := c.Bind(&fm); err != nil || fm.Name != "form" {
		t.Errorf("Bind 表单不符：%v %+v", err, fm)
	}

	// multipart
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	_ = mw.WriteField("name", "multipart")
	_ = mw.Close()
	req = httptest.NewRequest(http.MethodPost, "/", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	c = NewContext(httptest.NewRecorder(), req)
	fm = struct {
		Name string `form:"name"`
	}{}
	if err := c.Bind(&fm); err != nil || fm.Name != "multipart" {
		t.Errorf("Bind multipart 不符：%v %+v", err, fm)
	}

	// 无 Content-Type → 查询参数
	req = httptest.NewRequest(http.MethodGet, "/?name=query", nil)
	c = NewContext(httptest.NewRecorder(), req)
	var qm struct {
		Name string `query:"name"`
	}
	if err := c.Bind(&qm); err != nil || qm.Name != "query" {
		t.Errorf("Bind 查询不符：%v %+v", err, qm)
	}
}

type nestedAddr struct {
	City string `query:"pcity" form:"pcity"`
	Zip  int    `query:"pzip" form:"pzip"`
}

type nestedQueryModel struct {
	Name string `query:"name"`
	Addr struct {
		City string `query:"city"`
		Zip  int    `query:"zip"`
	} `query:"addr"`
	Home struct {
		City string `query:"hcity"`
	} `query:"home"`
	Skip struct {
		X string `query:"x"`
	} `query:"-"`
	Ptr   *nestedAddr `query:"ptr"`
	P     *int        `query:"p"`
	NoTag string
}

func TestBindQueryNested(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/?name=webx&city=北京&zip=100000&hcity=上海&pcity=深圳&x=no", nil)
	c := NewContext(httptest.NewRecorder(), req)
	var m nestedQueryModel
	m.Ptr = &nestedAddr{}
	if err := c.BindQuery(&m); err != nil {
		t.Fatalf("BindQuery 失败：%v", err)
	}
	if m.Name != "webx" || m.Addr.City != "北京" || m.Addr.Zip != 100000 ||
		m.Home.City != "上海" || m.Skip.X != "" || m.NoTag != "" {
		t.Errorf("嵌套绑定不符：%+v", m)
	}
	if m.Ptr == nil || m.Ptr.City != "深圳" {
		t.Errorf("嵌套指针绑定不符：%+v", m.Ptr)
	}
}

func TestBindQueryNestedError(t *testing.T) {
	m := nestedQueryModel{}
	m.Ptr = &nestedAddr{}
	req := httptest.NewRequest(http.MethodGet, "/?zip=abc", nil)
	c := NewContext(httptest.NewRecorder(), req)
	if err := c.BindQuery(&m); err == nil {
		t.Error("嵌套结构体解析失败应报错")
	}

	m2 := nestedQueryModel{}
	m2.Ptr = &nestedAddr{}
	req = httptest.NewRequest(http.MethodGet, "/?pzip=abc", nil)
	c = NewContext(httptest.NewRecorder(), req)
	if err := c.BindQuery(&m2); err == nil {
		t.Error("嵌套指针解析失败应报错")
	}
}

func TestBindQueryNestedNilPtr(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/?name=x", nil)
	c := NewContext(httptest.NewRecorder(), req)
	var m nestedQueryModel
	if err := c.BindQuery(&m); err != nil {
		t.Fatalf("BindQuery 失败：%v", err)
	}
	if m.Ptr != nil {
		t.Errorf("nil 指针应跳过：%+v", m.Ptr)
	}
}

func TestBindQueryNestedCycle(t *testing.T) {
	type cycleNode struct {
		Name string     `query:"name"`
		Next *cycleNode `query:"next"`
	}
	var n cycleNode
	n.Next = &n
	req := httptest.NewRequest(http.MethodGet, "/?name=x", nil)
	c := NewContext(httptest.NewRecorder(), req)
	if err := c.BindQuery(&n); err != nil {
		t.Fatalf("循环引用应安全跳过：%v", err)
	}
	testx.Equal(t, n.Name, "x")

}

func TestBindQueryPtrScalarError(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/?p=1", nil)
	c := NewContext(httptest.NewRecorder(), req)
	var m nestedQueryModel
	if err := c.BindQuery(&m); err == nil {
		t.Error("指针标量字段应报类型不支持")
	}
}

type nestedFormModel struct {
	Name string `form:"name"`
	Addr struct {
		City string `form:"city"`
	} `form:"addr"`
}

func TestBindFormNested(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("name=webx&city=北京"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	c := NewContext(httptest.NewRecorder(), req)
	var m nestedFormModel
	if err := c.BindForm(&m); err != nil {
		t.Fatalf("BindForm 失败：%v", err)
	}
	if m.Name != "webx" || m.Addr.City != "北京" {
		t.Errorf("表单嵌套绑定不符：%+v", m)
	}
}

func TestFormFileAndSave(t *testing.T) {
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	part, err := mw.CreateFormFile("file", "a.txt")
	testx.RequireNoError(t, err)

	if _, err := part.Write([]byte("文件内容")); err != nil {
		t.Fatal(err)
	}
	_ = mw.Close()
	req := httptest.NewRequest(http.MethodPost, "/", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	c := NewContext(httptest.NewRecorder(), req)
	fh, err := c.FormFile("file")
	testx.RequireNoError(t, err)

	testx.Equal(t, fh.Filename, "a.txt")

	dest := filepath.Join(t.TempDir(), "out.txt")
	if err := c.SaveUploadedFile(fh, dest); err != nil {
		t.Fatalf("SaveUploadedFile 失败：%v", err)
	}
	got, err := os.ReadFile(dest)
	testx.RequireNoError(t, err)

	if string(got) != "文件内容" {
		t.Errorf("落盘内容不符：%s", got)
	}
	if _, err := c.FormFile("missing"); err == nil {
		t.Error("缺失文件应报错")
	}
}

func TestFormFileMaxBytes(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(strings.Repeat("x", 100)))
	req.Header.Set("Content-Type", "multipart/form-data; boundary=x")
	c := NewContext(httptest.NewRecorder(), req)
	c.SetMaxBodyBytes(10)
	if _, err := c.FormFile("file"); err == nil {
		t.Error("超大请求体应报错")
	}
	fh := &multipart.FileHeader{Size: 100}
	c2 := NewContext(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/", nil))
	c2.SetMaxBodyBytes(10)
	if err := c2.SaveUploadedFile(fh, filepath.Join(t.TempDir(), "x")); err == nil {
		t.Error("文件大小超限应报错")
	}
}

func TestFormFileMalformedMultipart(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("这不是 multipart 内容"))
	req.Header.Set("Content-Type", "multipart/form-data; boundary=x")
	c := NewContext(httptest.NewRecorder(), req)
	if _, err := c.FormFile("file"); err == nil {
		t.Error("损坏的 multipart 应报错")
	}
}

func TestSaveUploadedFileErrors(t *testing.T) {
	c := NewContext(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/", nil))
	if err := c.SaveUploadedFile(nil, "x"); err == nil {
		t.Error("nil 文件应报错")
	}

	origOpen := openMultipartFile
	openMultipartFile = func(*multipart.FileHeader) (multipart.File, error) {
		return nil, errors.New("打开失败")
	}
	err := c.SaveUploadedFile(&multipart.FileHeader{}, filepath.Join(t.TempDir(), "x"))
	openMultipartFile = origOpen
	testx.Error(t, err)

	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	part, _ := mw.CreateFormFile("file", "a.txt")
	_, _ = part.Write([]byte("内容"))
	_ = mw.Close()
	req := httptest.NewRequest(http.MethodPost, "/", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	c2 := NewContext(httptest.NewRecorder(), req)
	fh, err := c2.FormFile("file")
	testx.RequireNoError(t, err)

	dir := t.TempDir()
	if err := c2.SaveUploadedFile(fh, dir); err == nil {
		t.Error("目标为目录应报错")
	}

	origCopy := copyFile
	copyFile = func(io.Writer, io.Reader) (int64, error) { return 0, errors.New("复制失败") }
	err = c2.SaveUploadedFile(fh, filepath.Join(dir, "out.txt"))
	copyFile = origCopy
	testx.Error(t, err)

}
